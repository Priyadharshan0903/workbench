package app

import "testing"

func TestHealthCompatibilityRejectsOtherRuntimeAndInstallation(t *testing.T) {
	root := t.TempDir()
	health := EngineHealth{OK: true, App: "workbench", Runtime: "go", Version: 3, Root: root}
	if err := compatibleEngine(health, root); err != nil {
		t.Fatal(err)
	}
	health.Runtime = "python"
	if compatibleEngine(health, root) == nil {
		t.Fatal("Python engine reused")
	}
	health.Runtime = "go"
	health.Root = t.TempDir()
	if compatibleEngine(health, root) == nil {
		t.Fatal("other installation reused")
	}
}
