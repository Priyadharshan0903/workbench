package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
	"workbench/internal/inventory"
)

func FixtureProject(path string) catalog.Project {
	return catalog.Project{ID: "sample", Name: "Sample", Description: "Fixture", Kind: "web", Path: path, URL: "http://127.0.0.1:3000/", Ports: []int{3000}, Lifecycle: "active", Owner: "Tester", Launch: "echo ready", Docs: "README.md"}
}
func FixtureStore(t *testing.T) (catalog.Store, catalog.Project) {
	t.Helper()
	root := t.TempDir()
	site := filepath.Join(root, "site")
	if e := os.Mkdir(site, 0700); e != nil {
		t.Fatal(e)
	}
	p := FixtureProject(site)
	s := catalog.Store{Root: root}
	if e := fsutil.WriteJSON(s.Path(), catalog.Registry{Version: 1, Revision: 1, Projects: []catalog.Project{p}}); e != nil {
		t.Fatal(e)
	}
	return s, p
}
func Write(t *testing.T, path, text string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(text), 0600); e != nil {
		t.Fatal(e)
	}
}

func FakeInspector(listeners, cwds string) inventory.Inspector {
	return inventory.Inspector{PID: 99999, Run: func(args ...string) (string, error) {
		if filepath.Base(args[0]) == "ps" {
			return "Mon Oct 26 09:00:00 2026\n", nil
		}
		if slices.Contains(args, "cwd") {
			return cwds, nil
		}
		return listeners, nil
	}}
}

func ID(n int) string {
	value := make([]byte, n)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}
