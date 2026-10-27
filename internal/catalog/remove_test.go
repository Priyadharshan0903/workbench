package catalog_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"workbench/internal/launch"
	"workbench/internal/testutil"
)

func TestRemovePreservesFilesLogsAndPreviousRegistry(t *testing.T) {
	store, project := testutil.FixtureStore(t)
	projectFile := filepath.Join(project.Path, "keep.txt")
	logFile := filepath.Join(store.Root, ".runtime", "logs", "keep.log")
	testutil.Write(t, projectFile, "project files stay")
	testutil.Write(t, logFile, "launch output stays")
	previous, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}

	removed, err := store.Remove(project.ID, 1)
	if err != nil || removed.Revision != 2 || removed.Projects == nil || len(removed.Projects) != 0 {
		t.Fatalf("remove: %+v, %v", removed, err)
	}
	backup, err := os.ReadFile(filepath.Join(store.Root, "registry.previous.json"))
	if err != nil || !bytes.Equal(backup, previous) {
		t.Fatal("removal did not preserve the previous catalog", err)
	}
	for filename, expected := range map[string]string{projectFile: "project files stay", logFile: "launch output stays"} {
		contents, err := os.ReadFile(filename)
		if err != nil || string(contents) != expected {
			t.Fatal("removal changed project data", filename, err)
		}
	}
	if _, err := store.Read(); err != nil {
		t.Fatal("empty catalog cannot be read", err)
	}
}

func TestRemoveRejectsStaleMissingInvalidAndProtectedProjects(t *testing.T) {
	store, project := testutil.FixtureStore(t)
	self := testutil.FixtureProject(store.Root)
	self.ID = "workbench"
	if _, err := store.Save(self, 1); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.Path())
	for _, request := range []struct {
		id       string
		revision int
	}{
		{project.ID, 1}, {project.ID, 0}, {project.ID, -1},
		{"missing", 2}, {"../sample", 2}, {"workbench", 2},
	} {
		if _, err := store.Remove(request.id, request.revision); err == nil {
			t.Fatalf("unreviewed removal accepted: %+v", request)
		}
		after, _ := os.ReadFile(store.Path())
		if !bytes.Equal(before, after) {
			t.Fatal("rejected removal changed the catalog")
		}
	}
}

func TestRemovingProjectInvalidatesReviewedLaunch(t *testing.T) {
	store, project := testutil.FixtureStore(t)
	supervisor := launch.New(store)
	supervisor.Inspector = testutil.FakeInspector("", "")
	plan, err := supervisor.LaunchPlan(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Remove(project.ID, plan.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := supervisor.Start(project.ID, plan.Fingerprint, plan.Revision); err == nil {
		t.Fatal("a removed project started from an old launch review")
	}
	if _, err := os.Stat(filepath.Join(store.Root, ".runtime", "runs")); !os.IsNotExist(err) {
		t.Fatal("a removed project created a runner")
	}
}
