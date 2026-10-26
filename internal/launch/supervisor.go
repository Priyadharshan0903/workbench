package launch

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
	"workbench/internal/inventory"
)

type Plan struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	CWD         string     `json:"cwd"`
	Commands    [][]string `json:"commands"`
	Command     string     `json:"command"`
	URL         string     `json:"url"`
	Ports       []int      `json:"ports"`
	Revision    int        `json:"revision"`
	Fingerprint string     `json:"fingerprint,omitempty"`
}
type RunRecord struct {
	RunID      string `json:"runId"`
	ProjectID  string `json:"projectId"`
	StartedAt  string `json:"startedAt"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Phase      string `json:"phase"`
	CWD        string `json:"cwd"`
	Command    string `json:"command"`
	LogFile    string `json:"logFile"`
	PID        int    `json:"pid"`
	Identity   string `json:"identity"`
	ChildPID   int    `json:"childPid,omitempty"`
	ExitCode   *int   `json:"exitCode,omitempty"`
}
type cachedHealth struct {
	at    time.Time
	value inventory.Health
}
type Supervisor struct {
	Store      catalog.Store
	Inspector  inventory.Inspector
	Executable string
	startMu    sync.Mutex
	healthMu   sync.Mutex
	cache      map[string]cachedHealth
}

func New(store catalog.Store) *Supervisor {
	exe, _ := os.Executable()
	return &Supervisor{Store: store, Inspector: inventory.NewInspector(), Executable: exe, cache: map[string]cachedHealth{}}
}
func localURL(value string) bool {
	u, e := url.Parse(value)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && slices.Contains([]string{"127.0.0.1", "localhost", "::1"}, u.Hostname()) && u.User == nil
}
func validateWebsite(project catalog.Project) error {
	if len(project.Ports) == 0 || !localURL(project.URL) {
		return errors.New("Add a local website URL and its expected port in project settings before starting.")
	}
	u, _ := url.Parse(project.URL)
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	if !slices.Contains(project.Ports, port) {
		return errors.New("The website URL must use one of the project's expected ports.")
	}
	return nil
}

func (s *Supervisor) LaunchPlan(id string) (Plan, error) {
	p := Plan{}
	r, e := s.Store.Read()
	if e != nil {
		return p, e
	}
	project, e := catalog.Find(r, id)
	if e != nil {
		return p, e
	}
	if project.ID == "workbench" || (project.Path != "" && fsutil.Canonical(project.Path) == fsutil.Canonical(s.Store.Root)) {
		return p, errors.New("Workbench is already open. Use its launcher to reopen it after quitting.")
	}
	if project.Kind != "web" {
		return p, errors.New("This item uses its own launcher. Open its details for the recorded command.")
	}
	if project.Lifecycle != "active" {
		return p, errors.New("Set this project to Active in project settings before starting it.")
	}
	if !filepath.IsAbs(project.Path) || !fsutil.IsDir(project.Path) {
		return p, errors.New("The project folder has moved or is missing. Update its path in project settings.")
	}
	if err := validateWebsite(project); err != nil {
		return p, err
	}
	reserved := []string{}
	for _, other := range r.Projects {
		if other.ID != id && inventory.Overlap(project.Ports, other.Ports) {
			reserved = append(reserved, other.Name)
		}
	}
	if len(reserved) > 0 {
		return p, errors.New("An expected port is also reserved by " + strings.Join(reserved, ", ") + ". Choose distinct ports first.")
	}
	commands, e := parseCommands(project.Launch, project.Path, nil)
	if e != nil {
		return p, e
	}
	p = Plan{ID: id, Name: project.Name, CWD: fsutil.Canonical(project.Path), Commands: commands, Command: displayCommand(commands), URL: project.URL, Ports: project.Ports, Revision: r.Revision}
	b, _ := json.Marshal(p)
	p.Fingerprint = digest(string(b))
	return p, nil
}
func (s *Supervisor) runPath(id string) (string, error) {
	if !catalog.ValidID(id) {
		return "", errors.New("Invalid project ID.")
	}
	return filepath.Join(s.Store.Root, ".runtime/runs", id+".json"), nil
}
func (s *Supervisor) ReadRun(id string) *RunRecord {
	path, e := s.runPath(id)
	if e != nil {
		return nil
	}
	b, e := fsutil.ReadBounded(path, 32768)
	if e != nil {
		return nil
	}
	var r RunRecord
	if json.Unmarshal(b, &r) != nil || r.ProjectID != id {
		return nil
	}
	return &r
}
func runAge(r *RunRecord) time.Duration {
	t, e := time.Parse(time.RFC3339Nano, r.StartedAt)
	if e != nil {
		return time.Hour
	}
	return time.Since(t)
}
func (s *Supervisor) ActiveRun(r *RunRecord) bool {
	if r == nil || r.Phase != "starting" || r.Identity == "" {
		return false
	}
	birth, e := s.Inspector.Birth(r.PID)
	return e == nil && birth == r.Identity
}
func randomID(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (s *Supervisor) Start(id, fingerprint string, revision int) (map[string]any, error) {
	if revision < 1 || fingerprint == "" {
		return nil, errors.New("Review the current launch plan before starting.")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	// A file lock protects duplicate starts across CLI/server processes as well.
	var result map[string]any
	e := s.Store.WithLock(func() error {
		plan, e := s.LaunchPlan(id)
		if e != nil {
			return e
		}
		if plan.Revision != revision || plan.Fingerprint != fingerprint {
			return errors.New("The launch details changed. Review the current command before starting.")
		}
		current, e := inventory.Capture(s.Store, s.Inspector)
		if e != nil {
			return e
		}
		if current.InspectionError != nil {
			return errors.New("Cannot inspect local ports. No duplicate service was started. Retry in a normal desktop session.")
		}
		for _, p := range current.Projects {
			if p.ID == id {
				if len(p.Services) > 0 {
					result = map[string]any{"ok": true, "alreadyRunning": true, "message": "This project already has a running service."}
					return nil
				}
				if len(p.Conflicts) > 0 {
					return errors.New("An expected port is occupied. Inspect the listener in Diagnostics before starting.")
				}
			}
		}
		if s.ActiveRun(s.ReadRun(id)) {
			result = map[string]any{"ok": true, "alreadyRunning": true, "message": "This project is already starting."}
			return nil
		}
		runtime := filepath.Join(s.Store.Root, ".runtime")
		for _, name := range []string{"runs", "logs"} {
			if e = os.MkdirAll(filepath.Join(runtime, name), 0700); e != nil {
				return e
			}
		}
		runID := randomID(8)
		logPath := filepath.Join(runtime, "logs", id+"-"+runID+".log")
		statePath, _ := s.runPath(id)
		record := RunRecord{RunID: runID, ProjectID: id, StartedAt: now(), Phase: "starting", CWD: plan.CWD, Command: plan.Command, LogFile: filepath.Base(logPath)}
		log, e := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer log.Close()
		cmd := exec.Command(s.Executable, "runner")
		cmd.Dir = s.Store.Root
		cmd.Stdout = log
		cmd.Stderr = log
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		stdin, e := cmd.StdinPipe()
		if e != nil {
			return e
		}
		if e = cmd.Start(); e != nil {
			stdin.Close()
			return e
		}
		go cmd.Wait()
		record.PID = cmd.Process.Pid
		record.Identity, e = s.Inspector.Birth(record.PID)
		if e != nil {
			stdin.Close()
			return fmt.Errorf("Could not verify the new runner identity. No project command was handed to it: %w", e)
		}
		if e = fsutil.WriteJSON(statePath, record); e != nil {
			stdin.Close()
			return e
		}
		payload := RunnerPayload{Commands: plan.Commands, CWD: plan.CWD, Env: runtimeEnvironment(), StatePath: statePath, LogPath: logPath, Record: record}
		e = json.NewEncoder(stdin).Encode(payload)
		stdin.Close()
		if e != nil {
			return errors.New("Could not hand the launch to the runner. Check the project logs.")
		}
		result = map[string]any{"ok": true, "runId": runID, "message": "Starting " + plan.Name + "…"}
		return nil
	})
	return result, e
}
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
