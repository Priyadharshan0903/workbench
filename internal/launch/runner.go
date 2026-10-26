package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"workbench/internal/fsutil"
)

type RunnerPayload struct {
	Commands  [][]string `json:"commands"`
	CWD       string     `json:"cwd"`
	Env       []string   `json:"env"`
	StatePath string     `json:"statePath"`
	LogPath   string     `json:"logPath"`
	Record    RunRecord  `json:"record"`
}
type rotatingLog struct {
	path string
	mu   sync.Mutex
}

func (l *rotatingLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if info, e := os.Stat(l.path); e == nil && info.Size() > 1<<20 {
		if e = os.Rename(l.path, strings.TrimSuffix(l.path, ".log")+".log.previous"); e != nil {
			return 0, e
		}
	}
	f, e := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return 0, e
	}
	n, e := f.Write(b)
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	return n, e
}
func RunDetached(input io.Reader) error {
	var payload RunnerPayload
	decoder := json.NewDecoder(io.LimitReader(input, 2<<20))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&payload); e != nil {
		return e
	}
	if len(payload.Commands) == 0 || payload.StatePath == "" || payload.LogPath == "" || !fsutil.IsDir(payload.CWD) {
		return errors.New("Invalid runner payload.")
	}
	record := payload.Record
	record.PID = os.Getpid()
	record.Phase = "starting"
	save := func() error { record.UpdatedAt = now(); return fsutil.WriteJSON(payload.StatePath, record) }
	if e := save(); e != nil {
		return e
	}
	log := &rotatingLog{path: payload.LogPath}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	done := make(chan struct{})
	defer close(done)
	var mu sync.Mutex
	var child *os.Process
	var stopping atomic.Bool
	go func() {
		for {
			select {
			case <-signals:
				stopping.Store(true)
				mu.Lock()
				if child != nil {
					_ = child.Signal(syscall.SIGTERM)
				}
				mu.Unlock()
			case <-done:
				return
			}
		}
	}()
	finish := func(code int, failed bool) error {
		record.ExitCode = &code
		record.FinishedAt = now()
		record.Phase = "completed"
		if stopping.Load() {
			record.Phase = "stopped"
		} else if failed {
			record.Phase = "failed"
		}
		return save()
	}
	for _, argv := range payload.Commands {
		if len(argv) == 0 {
			return errors.New("Empty launch step.")
		}
		mu.Lock()
		if stopping.Load() {
			mu.Unlock()
			break
		}
		_, _ = fmt.Fprintln(log, "\n$ "+displayCommand([][]string{argv}))
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = payload.CWD
		cmd.Env = payload.Env
		cmd.Stdout = log
		cmd.Stderr = log
		e := cmd.Start()
		if e != nil {
			mu.Unlock()
			_, _ = fmt.Fprintln(log, "\nWorkbench: "+e.Error())
			return finish(1, true)
		}
		child = cmd.Process
		record.ChildPID = child.Pid
		mu.Unlock()
		if e = save(); e != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
			return e
		}
		e = cmd.Wait()
		mu.Lock()
		child = nil
		mu.Unlock()
		if e != nil {
			code := 1
			var exit *exec.ExitError
			if errors.As(e, &exit) {
				code = exit.ExitCode()
			}
			_, _ = fmt.Fprintln(log, "\nWorkbench: "+e.Error())
			return finish(code, true)
		}
	}
	return finish(0, false)
}
