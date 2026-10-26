package launch

import (
	"testing"

	"workbench/internal/inventory"
	"workbench/internal/testutil"
)

func TestMissingWebsiteConfigurationIsSetupNotPermissionToRun(t *testing.T) {
	store, project := testutil.FixtureStore(t)
	project.URL, project.Ports = "", []int{}
	if _, err := store.Save(project, 1); err != nil {
		t.Fatal(err)
	}
	supervisor := New(store)
	supervisor.Inspector = testutil.FakeInspector("", "")
	state, err := inventory.Capture(store, supervisor.Inspector)
	if err != nil {
		t.Fatal(err)
	}
	supervisor.Enrich(state)
	view := state.Projects[0]
	if !view.CanLaunch || !view.LaunchSetupRequired || view.Status != "available" {
		t.Fatalf("missing setup hint: %+v", view)
	}
	if _, err := supervisor.LaunchPlan(project.ID); err == nil {
		t.Fatal("missing URL/ports produced a launch plan")
	}
	if _, err := supervisor.Start(project.ID, "unreviewed", state.Revision); err == nil {
		t.Fatal("missing setup launched a command")
	}
	project.URL, project.Ports = "http://127.0.0.1:5173/", []int{5173}
	if _, err := store.Save(project, state.Revision); err != nil {
		t.Fatal(err)
	}
	state, err = inventory.Capture(store, supervisor.Inspector)
	if err != nil {
		t.Fatal(err)
	}
	supervisor.Enrich(state)
	if state.Projects[0].LaunchSetupRequired || state.Projects[0].Status != "stopped" {
		t.Fatal("configured project still needs setup")
	}
	if _, err := supervisor.LaunchPlan(project.ID); err != nil {
		t.Fatal(err)
	}
}
