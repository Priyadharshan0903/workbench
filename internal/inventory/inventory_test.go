package inventory_test

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"workbench/internal/httpapi"
	"workbench/internal/inventory"
	"workbench/internal/launch"
	"workbench/internal/testutil"
)

func TestListenerParsingAndOwnership(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	nested := testutil.FixtureProject(filepath.Join(p.Path, "nested"))
	nested.ID = "nested"
	nested.Ports = []int{4000}
	shared := testutil.FixtureProject(p.Path)
	shared.ID = "shared"
	shared.Ports = []int{3002}
	_, e := s.Save(nested, 0)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Save(shared, 0)
	if e != nil {
		t.Fatal(e)
	}
	r, _ := s.Read()
	i := testutil.FakeInspector("p123\ncNode\nn127.0.0.1:3000\nn[::1]:3000\nn[::1]:3000\np124\ncNode\nn*:3001\np125\ncNode\nn*:3002\np126\ncUnknown\nn*:4000\n", "p123\nn"+p.Path+"\np124\nn"+nested.Path+"\np125\nn"+p.Path+"\np126\nn/other\n")
	services, e := i.Services(r.Projects)
	if e != nil {
		t.Fatal(e)
	}
	if *services[0].ProjectID != "sample" || len(services[0].Endpoints) != 2 || *services[1].ProjectID != "nested" || services[1].UnexpectedPorts[0] != 3001 || *services[2].ProjectID != "shared" || services[3].ProjectID != nil || services[3].CanStop {
		t.Fatalf("wrong ownership: %+v", services)
	}
}
func TestInspectionFailureUnknownAndUnverifiedReadiness(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	i := inventory.Inspector{Run: func(...string) (string, error) { return "", errors.New("Permission denied") }}
	state, e := inventory.Capture(s, i)
	if e != nil || state.Projects[0].Status != "unknown" || state.InspectionError == nil {
		t.Fatal("inspection failure treated as stopped")
	}
	supervisor := launch.New(s)
	supervisor.Enrich(state)
	if state.Projects[0].Status != "unknown" {
		t.Fatal("unknown became ready")
	}
	i = testutil.FakeInspector("p123\ncOther\nn127.0.0.1:3000\n", "p123\nn/other\n")
	state, _ = inventory.Capture(s, i)
	supervisor.Enrich(state)
	if state.Projects[0].Status != "unverified" {
		t.Fatal("unmatched listener ownership guessed")
	}
	if p.Path == "" {
		t.Fatal("invalid fixture")
	}
}
func TestStopFreshIdentityAndProtection(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	i := testutil.FakeInspector("p123\ncOwned\nn127.0.0.1:3000\n", "p123\nn"+p.Path+"\n")
	sup := launch.New(s)
	sup.Inspector = i
	app := httpapi.New(sup, 8730)
	signals := 0
	app.Signal = func(pid int, sig syscall.Signal) error {
		if pid != 123 || sig != syscall.SIGTERM {
			t.Fatal("wrong signal")
		}
		signals++
		return nil
	}
	state, e := app.State()
	if e != nil {
		t.Fatal(e)
	}
	token := state.Services[0].StopToken
	if token == "" || !state.Services[0].CanStop {
		t.Fatal("owned process cannot be reviewed")
	}
	if _, e = app.Stop(123, "stale"); e == nil || signals != 0 {
		t.Fatal("stale stop signalled")
	}
	original := i.Run
	sup.Inspector.Run = func(args ...string) (string, error) {
		if filepath.Base(args[0]) == "ps" {
			return "Tue Oct 27 09:00:00 2026", nil
		}
		return original(args...)
	}
	if _, e = app.Stop(123, token); e == nil || signals != 0 {
		t.Fatal("reused PID signalled")
	}
	sup.Inspector = i
	if _, e = app.Stop(123, token); e != nil || signals != 1 {
		t.Fatalf("verified stop: %v %d", e, signals)
	}
	sup.Inspector.PID = 123
	if _, e = app.Stop(123, token); e == nil || signals != 1 {
		t.Fatal("current engine signalled")
	}
	sup.Inspector = testutil.FakeInspector("p123\ncOwned\nn127.0.0.1:3000\n", "p123\nn/other\n")
	if _, e = app.Stop(123, token); e == nil || signals != 1 {
		t.Fatal("unmatched process signalled")
	}
}
func TestStopUnavailableExplained(t *testing.T) {
	s, p := testutil.FixtureStore(t)
	i := testutil.FakeInspector("p123\ncOwned\nn127.0.0.1:3000\n", "p123\nn"+p.Path+"\n")
	run := i.Run
	i.Run = func(args ...string) (string, error) {
		if strings.HasSuffix(args[0], "ps") {
			return "", errors.New("Identity check blocked")
		}
		return run(args...)
	}
	sup := launch.New(s)
	sup.Inspector = i
	app := httpapi.New(sup, 8730)
	state, e := app.State()
	if e != nil || state.Services[0].CanStop || !strings.Contains(state.Services[0].StopUnavailable, "blocked") || len(state.ControlWarnings) == 0 {
		t.Fatal("disabled stop has no explanation")
	}
}
