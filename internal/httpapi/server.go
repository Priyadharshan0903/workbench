package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"workbench/internal/catalog"
	"workbench/internal/discovery"
	"workbench/internal/inventory"
	"workbench/internal/launch"
	"workbench/internal/skills"
	"workbench/web"
)

type App struct {
	Supervisor  *launch.Supervisor
	Port        int
	CSRF        string
	SigningKey  string
	Stamp       string
	Assets      fs.FS
	Reload      func()
	Signal      func(int, syscall.Signal) error
	SkillFiles  *skills.Library
	skillsMu    sync.Mutex
	skillsAt    time.Time
	skillsCache discovery.Skills
}

func New(supervisor *launch.Supervisor, port int) *App {
	home, _ := os.UserHomeDir()

	return &App{
		Supervisor: supervisor,
		Port:       port,
		CSRF:       randomToken(),
		SigningKey: randomToken(),
		Stamp:      BinaryStamp(supervisor.Executable),
		Assets:     web.Static,
		Signal:     syscall.Kill,
		SkillFiles: &skills.Library{Store: supervisor.Store, Home: home},
	}
}

func BinaryStamp(path string) string {
	f, e := os.Open(path)
	if e != nil {
		return "unknown"
	}
	defer f.Close()
	hash := sha256.New()
	if _, e = io.Copy(hash, f); e != nil {
		return "unknown"
	}
	return hex.EncodeToString(hash.Sum(nil))[:20]
}
func (a *App) allowed(r *http.Request) bool {
	hosts := []string{fmt.Sprintf("127.0.0.1:%d", a.Port), fmt.Sprintf("localhost:%d", a.Port)}
	if !slices.Contains(hosts, r.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if values, ok := r.Header["Origin"]; ok {
		if len(values) != 1 || !slices.Contains([]string{"http://" + hosts[0], "http://" + hosts[1]}, values[0]) {
			return false
		}
	}
	return true
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, e error) {
	respond(w, status, map[string]string{"error": e.Error()})
}
func (a *App) stopToken(s *inventory.Service) (string, error) {
	birth, e := a.Supervisor.Inspector.Birth(s.PID)
	if e != nil {
		return "", e
	}
	endpoints := append([]string{}, s.Endpoints...)
	sort.Strings(endpoints)
	b, e := json.Marshal(processIdentity{
		PID:       s.PID,
		Birth:     birth,
		CWD:       s.CWD,
		Endpoints: endpoints,
		ProjectID: s.ProjectID,
	})
	if e != nil {
		return "", e
	}
	mac := hmac.New(sha256.New, []byte(a.SigningKey))
	mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil)), nil
}
func (a *App) State() (*inventory.Snapshot, error) {
	state, e := inventory.Capture(a.Supervisor.Store, a.Supervisor.Inspector)
	if e != nil {
		return nil, e
	}
	a.Supervisor.Enrich(state)
	for _, s := range state.Services {
		if s.CanStop {
			token, e := a.stopToken(s)
			if e != nil {
				s.CanStop = false
				s.StopUnavailable = "Cannot verify process identity: " + e.Error()
				warning := "Stop controls are unavailable because process identity cannot be verified. Retry from a normal desktop session."
				if !slices.Contains(state.ControlWarnings, warning) {
					state.ControlWarnings = append(state.ControlWarnings, warning)
				}
			} else {
				s.StopToken = token
			}
		}
	}
	state.Features = map[string]bool{"launch": true, "discovery": true, "logs": true, "readiness": true, "htmlLogs": true}
	state.Features["skillFiles"] = true
	state.CSRFToken = a.CSRF
	return state, nil
}
func (a *App) Stop(pid int, token string) (ActionResult, error) {
	if pid <= 1 || token == "" {
		return ActionResult{}, errors.New("Choose a currently listed service.")
	}
	state, e := inventory.Capture(a.Supervisor.Store, a.Supervisor.Inspector)
	if e != nil {
		return ActionResult{}, e
	}
	if state.InspectionError != nil {
		return ActionResult{}, errors.New("Process inspection is unavailable; no stop was attempted.")
	}
	for _, s := range state.Services {
		if s.PID == pid && s.CanStop {
			expected, e := a.stopToken(s)
			if e != nil || !hmac.Equal([]byte(token), []byte(expected)) {
				return ActionResult{}, errors.New("The process changed since it was listed. Refresh and inspect it again.")
			}
			if e = a.Signal(pid, syscall.SIGTERM); e != nil {
				return ActionResult{}, e
			}
			return ActionResult{
				OK:      true,
				Message: fmt.Sprintf("Sent a graceful stop request to PID %d. Refresh to verify it exited.", pid),
			}, nil
		}
	}
	return ActionResult{}, errors.New("This process is no longer a stoppable registered web service. Refresh the list.")
}
func (a *App) Skills(force bool) (discovery.Skills, error) {
	a.skillsMu.Lock()
	defer a.skillsMu.Unlock()
	if !force && !a.skillsAt.IsZero() && time.Since(a.skillsAt) < 30*time.Second {
		return a.skillsCache, nil
	}
	discovered, err := a.SkillFiles.Discover()
	if err != nil {
		return discovery.Skills{}, err
	}
	a.skillsCache = discovered
	a.skillsAt = time.Now()
	return a.skillsCache, nil
}

var logsTemplate = template.Must(template.ParseFS(web.Files, "templates/logs.html"))

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if !a.allowed(r) {
		failure(w, 403, errors.New("Local same-origin access only."))
		return
	}
	if r.Method == http.MethodGet {
		a.get(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		failure(w, 405, errors.New("Method not allowed."))
		return
	}
	if !hmac.Equal([]byte(r.Header.Get("X-Workbench-Token")), []byte(a.CSRF)) {
		failure(w, 403, errors.New("Refresh this local dashboard before making changes."))
		return
	}
	mimeType, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || mimeType != "application/json" {
		failure(w, 400, errors.New("A JSON request up to 32 KB is required."))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/skill/") {
		a.writeSkill(w, r)
		return
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
	if e != nil {
		failure(w, 400, errors.New("A JSON request up to 32 KB is required."))
		return
	}
	var data map[string]json.RawMessage
	if e = json.Unmarshal(body, &data); e != nil || data == nil {
		failure(w, 400, errors.New("A JSON object is required."))
		return
	}
	// Decode scalar fields explicitly so null, booleans and fractions are rejected.
	read := func(key string, target any) error {
		value, ok := data[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("Review the current %s before continuing.", key)
		}
		return json.Unmarshal(value, target)
	}
	var result any
	switch r.URL.Path {
	case "/api/project":
		var revision int
		var project catalog.Project
		if e = read("revision", &revision); e == nil && revision > 0 {
			if e = read("project", &project); e == nil {
				_, e = a.Supervisor.Store.Save(project, revision)
				result = map[string]bool{"ok": true}
			}
		} else if e == nil {
			e = errors.New("Refresh to get the current registry revision.")
		}
	case "/api/start":
		var id, fp string
		var revision int
		if e = read("id", &id); e == nil {
			e = read("fingerprint", &fp)
		}
		if e == nil {
			e = read("revision", &revision)
		}
		if e == nil {
			result, e = a.Supervisor.Start(id, fp, revision)
		}
	case "/api/stop":
		var pid int
		var token string
		if e = read("pid", &pid); e == nil {
			e = read("stopToken", &token)
		}
		if e == nil {
			result, e = a.Stop(pid, token)
		}
	case "/api/discover":
		var path string
		if e = read("path", &path); e == nil {
			if len(path) > 2000 {
				e = errors.New("Enter a project or workspace folder.")
			} else {
				var registry catalog.Registry
				registry, e = a.Supervisor.Store.Read()
				if e == nil {
					result, e = discovery.Discover(path, registry.Projects, 2)
				}
			}
		}
	case "/api/choose-folder":
		if runtime.GOOS != "darwin" {
			e = errors.New("Paste an absolute folder path to discover projects on this platform.")
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
			defer cancel()
			var b []byte
			b, e = exec.CommandContext(ctx, "osascript", "-e", `POSIX path of (choose folder with prompt "Choose a project or workspace folder for Workbench")`).Output()
			if e != nil {
				e = errors.New("Folder selection was cancelled or unavailable. You can paste a folder path instead.")
			} else {
				result = map[string]string{"path": strings.TrimSpace(string(b))}
			}
		}
	case "/api/reload":
		if a.Reload == nil {
			e = errors.New("Engine reload is unavailable in this session.")
		} else {
			result = ActionResult{OK: true, Message: "Reloading this Workbench engine. Project runners stay active."}
			defer func() { go func() { time.Sleep(300 * time.Millisecond); a.Reload() }() }()
		}
	default:
		failure(w, 404, errors.New("Not found."))
		return
	}
	if e != nil {
		failure(w, 400, e)
		return
	}
	respond(w, 200, result)
}
func (a *App) get(w http.ResponseWriter, r *http.Request) {
	var result any
	var e error
	switch r.URL.Path {
	case "/api/health":
		result = engineHealth{
			OK:          true,
			App:         "workbench",
			Version:     3,
			Runtime:     "go",
			SourceStamp: a.Stamp,
			SourceRoot:  a.Supervisor.Store.Root,
		}
	case "/api/state":
		result, e = a.State()
	case "/api/registry":
		result, e = a.Supervisor.Store.Read()
	case "/api/skills":
		result, e = a.Skills(r.URL.Query().Get("refresh") == "1")
	case "/api/skill":
		result, e = a.SkillFiles.View(r.URL.Query().Get("id"), r.URL.Query().Get("file"))
		if e != nil {
			skillFailure(w, e)
			return
		}
	case "/api/skill/export":
		a.exportSkill(w, r)
		return
	case "/api/launch-plan":
		result, e = a.Supervisor.LaunchPlan(r.URL.Query().Get("id"))
	case "/api/logs":
		result, e = a.Supervisor.Logs(r.URL.Query().Get("id"))
	case "/fragments/logs":
		var output launch.LogOutput
		output, e = a.Supervisor.Logs(r.URL.Query().Get("id"))
		if e == nil {
			var b bytes.Buffer
			e = logsTemplate.Execute(&b, output)
			if e == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(b.Bytes())
				return
			}
		}
	default:
		files := map[string]string{"/": "index.html", "/app.js": "app.js", "/skills.js": "skills.js", "/styles.css": "styles.css", "/board-grid.svg": "board-grid.svg", "/favicon.svg": "favicon.svg", "/vendor/htmx.min.js": "vendor/htmx.min.js", "/fonts/InterVariable.woff2": "fonts/InterVariable.woff2"}
		file, ok := files[r.URL.Path]
		if !ok {
			failure(w, 404, errors.New("Not found."))
			return
		}
		var b []byte
		b, e = fs.ReadFile(a.Assets, file)
		if e == nil {
			kind := mime.TypeByExtension(filepath.Ext(file))
			if kind == "" {
				kind = "application/octet-stream"
			}
			w.Header().Set("Content-Type", kind)
			w.Header().Set("Content-Length", strconv.Itoa(len(b)))
			w.Write(b)
			return
		}
	}
	if e != nil {
		failure(w, 500, e)
		return
	}
	respond(w, 200, result)
}

func randomToken() string {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}
