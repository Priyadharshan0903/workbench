package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"workbench/internal/fsutil"
	"workbench/internal/inventory"
	"workbench/internal/launch"
	"workbench/internal/testutil"
)

func rpc(id any, method string, params any) string {
	value := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		value["id"] = id
	}
	if params != nil {
		value["params"] = params
	}
	b, _ := json.Marshal(value)
	return string(b) + "\n"
}
func initialize(version string) string {
	return rpc(1, "initialize", map[string]any{"protocolVersion": version, "clientInfo": map[string]string{"name": "fixture", "version": "1"}, "capabilities": map[string]any{}}) + rpc(nil, "notifications/initialized", nil)
}
func messages(t *testing.T, s *Server, input string) []map[string]any {
	t.Helper()
	var output bytes.Buffer
	if err := s.Serve(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	result := []map[string]any{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		var message map[string]any
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatalf("non-protocol stdout: %q", line)
		}
		result = append(result, message)
	}
	return result
}
func result(message map[string]any) map[string]any { return message["result"].(map[string]any) }
func call(t *testing.T, s *Server, name string, args any) map[string]any {
	t.Helper()
	items := messages(t, s, rpc(2, "tools/call", map[string]any{"name": name, "arguments": args}))
	return result(items[0])
}
func fixture(t *testing.T, writes bool) *Server {
	t.Helper()
	store, _ := testutil.FixtureStore(t)
	s := New(store, writes)
	s.Supervisor.Inspector = testutil.FakeInspector("", "")
	messages(t, s, initialize("2025-11-25"))
	return s
}
func TestLifecycleProtocolNegotiationAndNoNotificationOutput(t *testing.T) {
	for _, version := range append(append([]string{}, supportedProtocols...), "2099-01-01") {
		store, _ := testutil.FixtureStore(t)
		s := New(store, false)
		items := messages(t, s, initialize(version)+rpc(2, "tools/list", map[string]any{})+rpc(3, "ping", nil))
		if len(items) != 3 {
			t.Fatal("notification produced a response", items)
		}
		want := version
		if !strings.HasPrefix(version, "202") {
			want = supportedProtocols[0]
		}
		if result(items[0])["protocolVersion"] != want {
			t.Fatal("wrong protocol negotiation", items[0])
		}
		tools := result(items[1])["tools"].([]any)
		if len(tools) != 6 {
			t.Fatal("unexpected default tools", tools)
		}
		for _, raw := range tools {
			tool := raw.(map[string]any)
			if tool["annotations"].(map[string]any)["readOnlyHint"] != true {
				t.Fatal("default tool writes", tool)
			}
		}
	}
}
func TestInvalidRequestsAndInitializationBoundaries(t *testing.T) {
	store, _ := testutil.FixtureStore(t)
	s := New(store, false)
	items := messages(t, s, "{\n"+`{"jsonrpc":"2.0","id":true,"method":"ping"}`+"\n"+rpc(2, "tools/list", nil)+rpc(3, "initialize", map[string]any{}))
	for i, want := range []float64{-32700, -32600, -32002, -32602} {
		if items[i]["error"].(map[string]any)["code"] != want {
			t.Fatal(items[i])
		}
	}
	if err := s.Serve(strings.NewReader(strings.Repeat("x", maxMessageBytes+1)), &bytes.Buffer{}); err == nil {
		t.Fatal("oversized frame accepted")
	}
}
func TestReadToolsHaveNoWriteOrProcessControl(t *testing.T) {
	s := fixture(t, false)
	if r := call(t, s, "list_projects", map[string]any{}); r["isError"] != false || r["structuredContent"].(map[string]any)["revision"] != float64(1) {
		t.Fatal(r)
	}
	if r := call(t, s, "get_project", map[string]any{"id": "../other"}); r["isError"] != true {
		t.Fatal("path traversal accepted")
	}
	if r := call(t, s, "get_project", map[string]any{"id": "sample", "root": "/other"}); r["isError"] != true {
		t.Fatal("tool changed catalog root")
	}
	for _, name := range []string{"start_project", "stop_service", "register_project"} {
		r := messages(t, s, rpc(2, "tools/call", map[string]any{"name": name, "arguments": map[string]any{}}))[0]
		if r["error"] == nil || r["error"].(map[string]any)["code"] != float64(-32602) {
			t.Fatal("control exposed", name)
		}
	}
	r, _ := s.Store.Read()
	if r.Revision != 1 {
		t.Fatal("read-only server changed catalog")
	}
}
func TestPortChecksPreserveReservationsAndInspectionFailures(t *testing.T) {
	s := fixture(t, false)
	r := call(t, s, "check_port", map[string]any{"port": 3000})["structuredContent"].(map[string]any)
	if r["available"] != false || len(r["reservedBy"].([]any)) != 1 {
		t.Fatal("reservation ignored", r)
	}
	r = call(t, s, "check_port", map[string]any{"port": 9999})["structuredContent"].(map[string]any)
	if r["available"] != true {
		t.Fatal(r)
	}
	s.Supervisor.Inspector = inventory.Inspector{Run: func(...string) (string, error) { return "", errors.New("inspection blocked") }}
	r = call(t, s, "check_port", map[string]any{"port": 9999})["structuredContent"].(map[string]any)
	if r["available"] != false || r["inspectionError"] == nil {
		t.Fatal("inspection failure treated as free", r)
	}
	if call(t, s, "check_port", map[string]any{"port": true})["isError"] != true {
		t.Fatal("boolean port accepted")
	}
}
func TestOptionalWritesRequireCurrentRevisionAndValidatedProject(t *testing.T) {
	s := fixture(t, true)
	p := testutil.FixtureProject(t.TempDir())
	p.Name = "Updated"
	listed := messages(t, s, rpc(8, "tools/list", map[string]any{}))
	if len(result(listed[0])["tools"].([]any)) != 7 {
		t.Fatal("write capability missing")
	}
	if call(t, s, "register_project", map[string]any{"project": p})["isError"] != true {
		t.Fatal("missing revision accepted")
	}
	if call(t, s, "register_project", map[string]any{"project": p, "revision": 1})["isError"] != false {
		t.Fatal("valid registration failed")
	}
	p.Name = "Stale"
	if call(t, s, "register_project", map[string]any{"project": p, "revision": 1})["isError"] != true {
		t.Fatal("stale registration accepted")
	}
	p.Path = "/tmp/.env"
	if call(t, s, "register_project", map[string]any{"project": p, "revision": 2})["isError"] != true {
		t.Fatal("invalid record accepted")
	}
	r, _ := s.Store.Read()
	if r.Revision != 2 || r.Projects[0].Name != "Updated" {
		t.Fatal("failed write changed records", r)
	}
}
func TestLogToolUsesBoundedRedactedOutput(t *testing.T) {
	s := fixture(t, false)
	run := launch.RunRecord{ProjectID: "sample", RunID: "0123456789abcdef", Phase: "failed", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), LogFile: "sample-0123456789abcdef.log"}
	dir := filepath.Join(s.Store.Root, ".runtime/runs")
	os.MkdirAll(dir, 0700)
	if err := fsutil.WriteJSON(filepath.Join(dir, "sample.json"), run); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(s.Store.Root, ".runtime/logs", run.LogFile), "token=private-fixture Bearer secret-fixture\ncommand output")
	r := call(t, s, "launch_logs", map[string]any{"id": "sample"})
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "private-fixture") || strings.Contains(string(b), "secret-fixture") || !strings.Contains(string(b), "command output") {
		t.Fatal("unsafe log result", string(b))
	}
}
