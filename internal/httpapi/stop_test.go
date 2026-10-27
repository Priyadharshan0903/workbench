package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"

	"workbench/internal/inventory"
	"workbench/internal/testutil"
)

type externalStopFixture struct {
	app           *App
	pid           int
	name          string
	cwd           string
	endpoints     string
	uid           string
	birth         string
	listening     bool
	inspectionErr error
	ownerErr      error
	birthErr      error
	signalErr     error
	signalCount   int
}

func newExternalStopFixture(t *testing.T) *externalStopFixture {
	t.Helper()
	fixture := &externalStopFixture{
		app:       fixtureApp(t),
		pid:       43210,
		name:      "node",
		cwd:       t.TempDir(),
		endpoints: "n127.0.0.1:3000\nn127.0.0.1:3001\n",
		uid:       strconv.Itoa(os.Geteuid()),
		birth:     "Tue Oct 27 09:00:00 2026",
		listening: true,
	}
	fixture.app.platform = "darwin"
	fixture.app.Supervisor.Inspector = inventory.Inspector{
		PID: 99999,
		Run: func(arguments ...string) (string, error) {
			if filepath.Base(arguments[0]) == "ps" {
				if slices.Contains(arguments, "uid=") {
					return fixture.uid, fixture.ownerErr
				}
				return fixture.birth, fixture.birthErr
			}
			if fixture.inspectionErr != nil {
				return "", fixture.inspectionErr
			}
			if !fixture.listening {
				return "", nil
			}
			if slices.Contains(arguments, "cwd") {
				return fmt.Sprintf("p%d\nn%s\n", fixture.pid, fixture.cwd), nil
			}
			return fmt.Sprintf("p%d\nc%s\n%s", fixture.pid, fixture.name, fixture.endpoints), nil
		},
	}
	fixture.app.Signal = func(pid int, signal syscall.Signal) error {
		if pid != fixture.pid || signal != syscall.SIGTERM {
			t.Fatalf("unexpected signal: PID %d, signal %v", pid, signal)
		}
		fixture.signalCount++
		return fixture.signalErr
	}

	return fixture
}

func TestExternalListenerStopRequiresReviewAndDoesNotRegisterProject(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unregistered root processes are intentionally protected")
	}
	fixture := newExternalStopFixture(t)
	before, err := fixture.app.Supervisor.Store.Read()
	if err != nil {
		t.Fatal(err)
	}

	state, err := fixture.app.State()
	if err != nil {
		t.Fatal(err)
	}
	service := state.Services[0]
	if service.ProjectID != nil || !service.CanStop || service.StopToken == "" || fixture.signalCount != 0 {
		t.Fatalf("listing must offer review without registering or signalling: %+v", service)
	}

	body, err := json.Marshal(struct {
		PID       int    `json:"pid"`
		StopToken string `json:"stopToken"`
	}{fixture.pid, service.StopToken})
	if err != nil {
		t.Fatal(err)
	}
	if response := requestApp(fixture.app, "POST", "/api/stop", string(body), map[string]string{"Content-Type": "application/json"}); response.Code != 403 || fixture.signalCount != 0 {
		t.Fatal("stop without the dashboard token was accepted")
	}
	if _, err := fixture.app.Stop(fixture.pid, ""); err == nil || fixture.signalCount != 0 {
		t.Fatal("stop without a reviewed process token was accepted")
	}
	if _, err := fixture.app.Stop(fixture.pid, "forged-token"); err == nil || fixture.signalCount != 0 {
		t.Fatal("stop with a forged process token was accepted")
	}

	headers := map[string]string{"Content-Type": "application/json", "X-Workbench-Token": fixture.app.CSRF}
	response := requestApp(fixture.app, "POST", "/api/stop", string(body), headers)
	if response.Code != 200 || fixture.signalCount != 1 {
		t.Fatalf("reviewed stop failed: %d %s", response.Code, response.Body.String())
	}

	after, err := fixture.app.Supervisor.Store.Read()
	if err != nil || after.Revision != before.Revision || len(after.Projects) != len(before.Projects) {
		t.Fatal("stopping an external listener changed the catalog", err)
	}
}

func TestExternalListenerStopEligibility(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*externalStopFixture)
	}{
		{"other platform", func(fixture *externalStopFixture) { fixture.app.platform = "linux" }},
		{"current engine", func(fixture *externalStopFixture) { fixture.app.Supervisor.Inspector.PID = fixture.pid }},
		{"init process", func(fixture *externalStopFixture) { fixture.pid = 1 }},
		{"root owner", func(fixture *externalStopFixture) { fixture.uid = "0" }},
		{"other owner", func(fixture *externalStopFixture) { fixture.uid = strconv.Itoa(os.Geteuid() + 1) }},
		{"missing owner", func(fixture *externalStopFixture) { fixture.uid = "" }},
		{"malformed owner", func(fixture *externalStopFixture) { fixture.uid = "501\n502" }},
		{"owner check failed", func(fixture *externalStopFixture) { fixture.ownerErr = errors.New("permission denied") }},
		{"missing folder", func(fixture *externalStopFixture) { fixture.cwd = "" }},
		{"root folder", func(fixture *externalStopFixture) { fixture.cwd = "/" }},
		{"relative folder", func(fixture *externalStopFixture) { fixture.cwd = "project" }},
		{"birth check failed", func(fixture *externalStopFixture) { fixture.birthErr = errors.New("identity unavailable") }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newExternalStopFixture(t)
			testCase.configure(fixture)
			state, err := fixture.app.State()
			if err != nil {
				t.Fatal(err)
			}
			if state.Services[0].CanStop || state.Services[0].StopToken != "" {
				t.Fatalf("protected listener offered Stop: %+v", state.Services[0])
			}
			if _, err := fixture.app.Stop(fixture.pid, "forged-token"); err == nil || fixture.signalCount != 0 {
				t.Fatal("protected listener was signalled")
			}
		})
	}
}

func TestExternalListenerStopRechecksIdentity(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unregistered root processes are intentionally protected")
	}
	cases := []struct {
		name   string
		change func(*externalStopFixture)
	}{
		{"PID reused", func(fixture *externalStopFixture) { fixture.birth = "Tue Oct 27 10:00:00 2026" }},
		{"name changed", func(fixture *externalStopFixture) { fixture.name = "other-process" }},
		{"folder changed", func(fixture *externalStopFixture) { fixture.cwd = filepath.Dir(fixture.cwd) }},
		{"ports changed", func(fixture *externalStopFixture) { fixture.endpoints = "n127.0.0.1:4000\n" }},
		{"owner changed", func(fixture *externalStopFixture) { fixture.uid = strconv.Itoa(os.Geteuid() + 1) }},
		{"listener exited", func(fixture *externalStopFixture) { fixture.listening = false }},
		{"inspection failed", func(fixture *externalStopFixture) { fixture.inspectionErr = errors.New("inspection blocked") }},
		{"identity unavailable", func(fixture *externalStopFixture) { fixture.birthErr = errors.New("identity blocked") }},
		{"became current engine", func(fixture *externalStopFixture) { fixture.app.Supervisor.Inspector.PID = fixture.pid }},
		{"project association changed", func(fixture *externalStopFixture) {
			project := testutil.FixtureProject(fixture.cwd)
			project.ID = "new-project"
			if _, err := fixture.app.Supervisor.Store.Save(project, 0); err != nil {
				t.Fatal(err)
			}
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newExternalStopFixture(t)
			state, err := fixture.app.State()
			if err != nil || state.Services[0].StopToken == "" {
				t.Fatal("could not prepare reviewed listener", err)
			}
			token := state.Services[0].StopToken
			testCase.change(fixture)
			if _, err := fixture.app.Stop(fixture.pid, token); err == nil || fixture.signalCount != 0 {
				t.Fatal("changed listener was signalled")
			}
		})
	}
}

func TestExternalListenerStopReportsSignalFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unregistered root processes are intentionally protected")
	}
	fixture := newExternalStopFixture(t)
	state, err := fixture.app.State()
	if err != nil {
		t.Fatal(err)
	}
	fixture.signalErr = syscall.EPERM
	if _, err := fixture.app.Stop(fixture.pid, state.Services[0].StopToken); !errors.Is(err, syscall.EPERM) || fixture.signalCount != 1 {
		t.Fatal("failed signal was reported as a successful stop", err)
	}
}
