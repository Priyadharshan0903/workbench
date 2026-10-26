package app

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"workbench/internal/catalog"
	"workbench/internal/testutil"
)

func TestInitializeAndCLIUseSameRegistry(t *testing.T) {
	root := t.TempDir()
	s := catalog.Store{Root: root}
	if e := s.Initialize(8742); e != nil {
		t.Fatal(e)
	}
	if e := s.Initialize(8742); e != nil {
		t.Fatal(e)
	}
	r, _ := s.Read()
	if r.Revision != 1 || r.Projects[0].Ports[0] != 8742 {
		t.Fatal("initialization is not idempotent")
	}
	var out, stderr bytes.Buffer
	p := testutil.FixtureProject(filepath.Join(root, "site"))
	b, _ := json.Marshal(p)
	code, e := Run([]string{"--root", root, "register", "--file", "-"}, bytes.NewReader(b), &out, &stderr)
	if e != nil || code != 0 {
		t.Fatalf("register: %d %v", code, e)
	}
	out.Reset()
	code, e = Run([]string{"--root", root, "lifecycle", "sample", "paused"}, nil, &out, &stderr)
	if e != nil || code != 0 {
		t.Fatal(e)
	}
	out.Reset()
	code, e = Run([]string{"--root", root, "list", "--json"}, nil, &out, &stderr)
	if e != nil || code != 0 || !strings.Contains(out.String(), `"paused"`) {
		t.Fatalf("CLI: %s %v", out.String(), e)
	}
}
