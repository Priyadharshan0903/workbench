package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
	"workbench/internal/httpapi"
	"workbench/internal/launch"
)

type EngineHealth struct {
	OK      bool   `json:"ok"`
	App     string `json:"app"`
	Version int    `json:"version"`
	Runtime string `json:"runtime"`
	Root    string `json:"sourceRoot"`
	Stamp   string `json:"sourceStamp"`
}

func localClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func engineHealth(client *http.Client, address string) (EngineHealth, error) {
	var health EngineHealth
	response, e := client.Get(address + "/api/health")
	if e != nil {
		return health, e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return health, errors.New("Engine health check failed.")
	}
	e = json.NewDecoder(io.LimitReader(response.Body, 16384)).Decode(&health)
	if e == nil && (health.App != "workbench" || !health.OK) {
		e = errors.New("The listener is not a healthy Workbench instance.")
	}
	return health, e
}
func compatibleEngine(health EngineHealth, root string) error {
	if health.Runtime != "go" || health.Version != 3 {
		return errors.New("Another runtime or older Workbench is using this port. Reuse that workspace or choose a free port in .runtime/local.json. No process was stopped.")
	}
	if health.Root == "" || fsutil.Canonical(health.Root) != fsutil.Canonical(root) {
		return errors.New("Another Workbench installation is already using this port. Open that workspace or choose another port. No process was stopped.")
	}
	return nil
}
func startWorkbench(store catalog.Store, port int, noBrowser bool, out io.Writer) error {
	address := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := localClient(1500 * time.Millisecond)
	exe, _ := os.Executable()
	stamp := httpapi.BinaryStamp(exe)
	if health, e := engineHealth(client, address); e == nil {
		if e = compatibleEngine(health, store.Root); e != nil {
			return e
		}
		if health.Stamp != stamp {
			response, e := client.Get(address + "/api/state")
			if e != nil {
				return e
			}
			var state struct {
				Token string `json:"csrfToken"`
			}
			e = json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&state)
			response.Body.Close()
			if e != nil || state.Token == "" {
				return errors.New("Could not obtain the engine reload token.")
			}
			request, _ := http.NewRequest(http.MethodPost, address+"/api/reload", strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Workbench-Token", state.Token)
			response, e = client.Do(request)
			if e != nil {
				return e
			}
			response.Body.Close()
			if response.StatusCode != 200 {
				return errors.New("Engine reload was rejected.")
			}
			ready := false
			for n := 0; n < 60; n++ {
				time.Sleep(100 * time.Millisecond)
				h, e := engineHealth(client, address)
				if e == nil && h.Stamp == stamp && compatibleEngine(h, store.Root) == nil {
					ready = true
					break
				}
			}
			if !ready {
				return errors.New("The engine could not reload. See .runtime/server.log.")
			}
		}
	} else {
		conn, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if e == nil {
			conn.Close()
			return fmt.Errorf("Port %d belongs to another service. Inspect it before starting Workbench.", port)
		}
		if e = os.MkdirAll(filepath.Join(store.Root, ".runtime"), 0700); e != nil {
			return e
		}
		log, e := os.OpenFile(filepath.Join(store.Root, ".runtime/server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer log.Close()
		cmd := exec.Command(exe, "--root", store.Root, "serve", "--port", strconv.Itoa(port))
		cmd.Dir = store.Root
		cmd.Stdout = log
		cmd.Stderr = log
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if e = cmd.Start(); e != nil {
			return e
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		_ = fsutil.AtomicWrite(filepath.Join(store.Root, ".runtime/server.pid"), []byte(strconv.Itoa(cmd.Process.Pid)))
		ready := false
		for n := 0; n < 60; n++ {
			if health, e := engineHealth(client, address); e == nil {
				if e = compatibleEngine(health, store.Root); e != nil {
					return e
				}
				ready = true
				break
			}
			select {
			case <-done:
				return errors.New("Workbench could not start. See .runtime/server.log.")
			default:
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !ready {
			return errors.New("Workbench did not become ready. See .runtime/server.log.")
		}
	}
	// Synchronize only this installation's own URL after successful reuse/start.
	r, e := store.Read()
	if e != nil {
		return e
	}
	for _, p := range r.Projects {
		if p.ID == "workbench" && fsutil.Canonical(p.Path) == store.Root && (p.URL != address+"/" || len(p.Ports) != 1 || p.Ports[0] != port) {
			p.URL = address + "/"
			p.Ports = []int{port}
			if _, e = store.Save(p, r.Revision); e != nil {
				return e
			}
			break
		}
	}
	fmt.Fprintln(out, "Workbench is ready: "+address)
	if !noBrowser {
		program := "xdg-open"
		if runtime.GOOS == "darwin" {
			program = "open"
		}
		cmd := exec.Command(program, address)
		if e = cmd.Start(); e != nil {
			return fmt.Errorf("Workbench is ready, but the browser could not open: %w", e)
		}
		go cmd.Wait()
	}
	return nil
}
func serve(store catalog.Store, port int, out io.Writer) error {
	if _, e := store.Read(); e != nil {
		return e
	}
	listener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		return e
	}
	app := httpapi.New(launch.New(store), port)
	app.ReleaseVersion = Version
	server := &http.Server{Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32768}
	app.Reload = func() {
		exe, e := os.Executable()
		if e != nil {
			return
		}
		if e = syscall.Exec(exe, []string{exe, "--root", store.Root, "serve", "--port", strconv.Itoa(port)}, os.Environ()); e != nil {
			fmt.Fprintln(out, "Engine reload failed:", e)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(out, "Workbench (Go) is running at http://127.0.0.1:%d\n", port)
	e = server.Serve(listener)
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
