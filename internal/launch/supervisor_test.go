package launch

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"workbench/internal/fsutil"
	"workbench/internal/inventory"
	"workbench/internal/testutil"
)

// The test executable supplies an isolated service and runs the real runner.
// Production has no fixture routes or test modes.

func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "runner":
			if e := RunDetached(os.Stdin); e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			os.Exit(0)
		case "fixture-prepare":
			fmt.Println("prepare complete")
			os.Exit(0)
		case "fixture-fail":
			fmt.Println("ModuleNotFoundError: fixture dependency token=private-value")
			os.Exit(7)
		case "fixture-service":
			port := os.Args[2]
			listener, e := net.Listen("tcp", "127.0.0.1:"+port)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			server := &http.Server{}
			server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "fixture ready")
				if r.URL.Path == "/__stop" {
					go func() { time.Sleep(30 * time.Millisecond); server.Close() }()
				}
			})
			fmt.Println("fixture started")
			_ = server.Serve(listener)
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}
func waitUntil(t *testing.T, description string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("Timed out: " + description)
}

// Some hosted macOS sessions prohibit ps even after execution approval. Keep
// the actual child, lsof and HTTP lifecycle checks, injecting just the blocked
// identity query for these test-owned fixtures. Stop-token identity changes are
// independently tested with explicit old/new birth values.
func fixtureIdentityInspector(t *testing.T, inspector inventory.Inspector) inventory.Inspector {
	t.Helper()
	if _, e := inventory.NewInspector().Birth(os.Getpid()); e != nil {
		t.Logf("OS process birth inspection unavailable; injecting identity for test-owned fixture: %v", e)
		run := inspector.Run
		inspector.Run = func(args ...string) (string, error) {
			if filepath.Base(args[0]) == "ps" {
				return "test-owned-fixture-birth", nil
			}
			return run(args...)
		}
	}
	return inspector
}
func TestCommandLexerLiteralArgumentsAndRejections(t *testing.T) {
	cwd := t.TempDir()
	commands, e := parseCommands(`echo 'print(1); print(2)' "" 'a && b' && echo "double quoted"`, cwd, nil)
	if e != nil || len(commands) != 2 || commands[0][1] != "print(1); print(2)" || commands[0][2] != "" || commands[0][3] != "a && b" {
		t.Fatal(commands, e)
	}
	for _, cmd := range []string{"echo x &", "echo x > out", "echo $(whoami)", "echo `whoami`", "echo x | cat", "echo x; echo y", "echo x || echo y", "echo x &&", "&& echo x", "echo 'unclosed", "PORT=3000 echo x", "echo x\ncat", "echo x >>> file"} {
		if _, e = parseCommands(cmd, cwd, nil); e == nil {
			t.Errorf("accepted shell syntax: %s", cmd)
		}
	}
	if _, e = parseCommands("workbench-nonexistent-runtime main.js", cwd, nil); e == nil || !strings.Contains(e.Error(), "not installed") {
		t.Fatal("missing runtime not explained", e)
	}
}
func TestLaunchPlanGuardsAndChangeFingerprint(t *testing.T) {
	store, p := testutil.FixtureStore(t)
	sup := New(store)
	sup.Inspector = testutil.FakeInspector("", "")
	plan, e := sup.LaunchPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sup.Start(p.ID, "stale", 1); e == nil {
		t.Fatal("stale plan started")
	}
	if fsutil.Exists(filepath.Join(store.Root, ".runtime/runs")) {
		t.Fatal("stale plan created a runner")
	}
	p.Lifecycle = "paused"
	store.Save(p, 0)
	if _, e = sup.LaunchPlan(p.ID); e == nil {
		t.Fatal("paused project planned")
	}
	p.Lifecycle = "active"
	p.URL = "https://example.com"
	store.Save(p, 0)
	if _, e = sup.LaunchPlan(p.ID); e == nil {
		t.Fatal("remote URL planned")
	}
	p.URL = "http://127.0.0.1:3001"
	store.Save(p, 0)
	if _, e = sup.LaunchPlan(p.ID); e == nil {
		t.Fatal("unreserved URL port planned")
	}
	p.URL = "http://127.0.0.1:3000"
	store.Save(p, 0)
	other := testutil.FixtureProject("/other")
	other.ID = "other"
	store.Save(other, 0)
	if _, e = sup.LaunchPlan(p.ID); e == nil || !strings.Contains(e.Error(), "reserved") {
		t.Fatal("overlapping reservation planned", e)
	}
	if plan.Fingerprint == "" {
		t.Fatal("missing fingerprint")
	}
}
func TestRunnerLifecycleDuplicateGuardReattachAndFailure(t *testing.T) {
	if _, e := exec.LookPath("lsof"); e != nil {
		t.Skip("lsof is required for process integration")
	}
	store, p := testutil.FixtureStore(t)
	probe, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()
	exe, _ := os.Executable()
	p.Ports = []int{port}
	p.URL = fmt.Sprintf("http://127.0.0.1:%d/", port)
	p.Launch = shellQuote(exe) + " fixture-prepare && " + shellQuote(exe) + " fixture-service " + strconv.Itoa(port)
	r, e := store.Save(p, 1)
	if e != nil {
		t.Fatal(e)
	}
	sup := New(store)
	sup.Inspector = fixtureIdentityInspector(t, sup.Inspector)
	plan, e := sup.LaunchPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(func() {
		response, e := client.Get(p.URL + "__stop")
		if e == nil {
			response.Body.Close()
		}
		record := sup.ReadRun(p.ID)
		if record != nil && sup.ActiveRun(record) {
			process, e := os.FindProcess(record.PID)
			if e == nil {
				_ = process.Signal(os.Interrupt)
			}
		}
	})
	first, e := sup.Start(p.ID, plan.Fingerprint, r.Revision)
	if e != nil {
		t.Fatal(e)
	}
	// A new supervisor has no in-memory child handles: identity on disk must suffice.
	reattached := New(store)
	reattached.Inspector = fixtureIdentityInspector(t, reattached.Inspector)
	waitUntil(t, "runner starts listening", func() bool {
		record := reattached.ReadRun(p.ID)
		return record != nil && record.ChildPID > 0 && reattached.ActiveRun(record)
	})
	second, e := reattached.Start(p.ID, plan.Fingerprint, r.Revision)
	if e != nil || !second.AlreadyRunning {
		t.Fatalf("duplicate runner: %v %v", second, e)
	}
	if reattached.ReadRun(p.ID).RunID != first.RunID {
		t.Fatal("duplicate start replaced run")
	}
	waitUntil(t, "website ready", func() bool {
		state, e := inventory.Capture(store, reattached.Inspector)
		if e != nil {
			return false
		}
		reattached.Enrich(state)
		return state.Projects[0].Status == "ready"
	})
	logs, e := sup.Logs(p.ID)
	if e != nil || !strings.Contains(logs.Text, "prepare complete") || !strings.Contains(logs.Text, "fixture started") {
		t.Fatal("sequential output missing", logs, e)
	}
	response, e := client.Get(p.URL + "__stop")
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	waitUntil(t, "runner completed", func() bool { return sup.ReadRun(p.ID).Phase == "completed" })
	p.Launch = shellQuote(exe) + " fixture-fail"
	r, e = store.Save(p, r.Revision)
	if e != nil {
		t.Fatal(e)
	}
	plan, e = sup.LaunchPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sup.Start(p.ID, plan.Fingerprint, r.Revision); e != nil {
		t.Fatal(e)
	}
	waitUntil(t, "failed command recorded", func() bool { return sup.ReadRun(p.ID).Phase == "failed" })
	record := sup.ReadRun(p.ID)
	if record.ExitCode == nil || *record.ExitCode != 7 {
		t.Fatal("exit code not preserved", record)
	}
	state, e := inventory.Capture(store, sup.Inspector)
	if e != nil {
		t.Fatal(e)
	}
	sup.Enrich(state)
	if state.Projects[0].Status != "failed" || !strings.Contains(state.Projects[0].StatusMessage, "dependency") {
		t.Fatal("failure diagnosis missing", state.Projects[0])
	}
	logs, _ = sup.Logs(p.ID)
	if strings.Contains(logs.Text, "private-value") {
		t.Fatal("credentials leaked")
	}
}
func TestConcurrentStartsAcrossSupervisors(t *testing.T) {
	store, p := testutil.FixtureStore(t)
	exe, _ := os.Executable()
	p.Launch = shellQuote(exe) + " fixture-service 0"
	r, e := store.Save(p, 1)
	if e != nil {
		t.Fatal(e)
	}
	sup := New(store)
	sup.Inspector = testutil.FakeInspector("", "")
	other := New(store)
	other.Inspector = sup.Inspector
	plan, e := sup.LaunchPlan(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	// Use actual birth checks; service inspection is isolated from the host catalog.
	for _, s := range []*Supervisor{sup, other} {
		fake := s.Inspector.Run
		s.Inspector.Run = func(args ...string) (string, error) {
			if filepath.Base(args[0]) == "ps" {
				return inventory.RunCommand(args...)
			}
			return fake(args...)
		}
		s.Inspector = fixtureIdentityInspector(t, s.Inspector)
	}
	var wg sync.WaitGroup
	results := make(chan StartResult, 2)
	for _, s := range []*Supervisor{sup, other} {
		wg.Add(1)
		go func(s *Supervisor) {
			defer wg.Done()
			result, e := s.Start(p.ID, plan.Fingerprint, r.Revision)
			if e != nil {
				t.Error(e)
			}
			results <- result
		}(s)
	}
	wg.Wait()
	close(results)
	started := 0
	for result := range results {
		if result.RunID != "" {
			started++
		}
	}
	record := sup.ReadRun(p.ID)
	if record != nil {
		waitUntil(t, "concurrent fixture child starts", func() bool { r := sup.ReadRun(p.ID); return r != nil && r.ChildPID > 0 })
		process, _ := os.FindProcess(record.PID)
		if process != nil {
			_ = process.Signal(os.Interrupt)
		}
		waitUntil(t, "concurrent fixture stopped", func() bool { r := sup.ReadRun(p.ID); return r != nil && r.Phase == "stopped" })
	}
	if started != 1 {
		t.Fatalf("created %d runners", started)
	}
}
func TestLogRotationBoundsAndRedaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.log")
	log := rotatingLog{path: path}
	for n := 0; n < 300; n++ {
		if _, e := log.Write([]byte(strings.Repeat("x", 8192))); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{path, strings.TrimSuffix(path, ".log") + ".log.previous"} {
		info, e := os.Stat(name)
		if e != nil || info.Size() > (1<<20)+8192 {
			t.Fatal("unbounded log", info, e)
		}
	}
	text := redact("\x1b[31mERROR\x1b[0m token=private-value Bearer abcd.efg ghp_abcdefghijklmnopq")
	if strings.Contains(text, "private-value") || strings.Contains(text, "abcd.efg") || strings.Contains(text, "ghp_") || !strings.Contains(text, "ERROR") {
		t.Fatal(text)
	}
}
func TestRunStateRejectsOversizeAndWrongProject(t *testing.T) {
	s, _ := testutil.FixtureStore(t)
	sup := New(s)
	path, _ := sup.runPath("sample")
	b, _ := json.Marshal(RunRecord{ProjectID: "other"})
	testutil.Write(t, path, string(b))
	if sup.ReadRun("sample") != nil {
		t.Fatal("cross-project run accepted")
	}
	testutil.Write(t, path, strings.Repeat("x", 32769))
	if sup.ReadRun("sample") != nil {
		t.Fatal("oversized run read")
	}
}
