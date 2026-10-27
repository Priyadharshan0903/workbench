package app

import "testing"

func TestHealthCompatibilityRejectsOtherRuntimeAndInstallation(t *testing.T) {
	root := t.TempDir()
	health := EngineHealth{OK: true, App: "workbench", Runtime: "go", Version: 3, Root: root}
	if err := compatibleEngine(health, root); err != nil {
		t.Fatal(err)
	}
	health.Runtime = "unsupported"
	if compatibleEngine(health, root) == nil {
		t.Fatal("unsupported engine runtime reused")
	}
	health.Runtime = "go"
	health.Root = t.TempDir()
	if compatibleEngine(health, root) == nil {
		t.Fatal("other installation reused")
	}
}
