package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"workbench/internal/testutil"
)

func TestMCPCLIHasProtocolOnlyStdout(t *testing.T) {
	store, _ := testutil.FixtureStore(t)
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	var out, stderr bytes.Buffer
	code, err := Run([]string{"--root", store.Root, "mcp"}, strings.NewReader(input), &out, &stderr)
	if code != 0 || err != nil || stderr.Len() != 0 {
		t.Fatal(code, err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatal(out.String())
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatal("non-JSON stdout", line)
		}
	}
}

func TestCLIRegisterRevisionRejectsStaleWrites(t *testing.T) {
	store, p := testutil.FixtureStore(t)
	p.Name = "Updated"
	b, _ := json.Marshal(p)
	var out, stderr bytes.Buffer
	args := []string{"--root", store.Root, "register", "--file", "-", "--revision", "1"}
	if code, err := Run(args, bytes.NewReader(b), &out, &stderr); code != 0 || err != nil {
		t.Fatal(code, err)
	}
	p.Name = "Stale"
	b, _ = json.Marshal(p)
	if _, err := Run(args, bytes.NewReader(b), &out, &stderr); err == nil {
		t.Fatal("stale CLI write accepted")
	}
	r, _ := store.Read()
	if r.Revision != 2 || r.Projects[0].Name != "Updated" {
		t.Fatal("stale CLI write changed catalog")
	}
}
