package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
	"workbench/internal/testutil"
)

func TestDiscoveryBoundedReadOnlyAndGoSupport(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "web/package.json"), `{"name":"my-site","scripts":{"dev":"vite --port 8833"}}`)
	testutil.Write(t, filepath.Join(root, "python/start.py"), `URL="http://127.0.0.1:8834" # --no-browser`)
	testutil.Write(t, filepath.Join(root, "golang/go.mod"), "module fixture\n")
	testutil.Write(t, filepath.Join(root, "golang/main.go"), "package main\n")
	for _, folder := range []string{"node_modules/hidden", ".ssh/hidden", ".env.private/hidden"} {
		testutil.Write(t, filepath.Join(root, folder, "package.json"), `{"scripts":{"dev":"DO NOT EXECUTE"}}`)
	}
	if e := os.Symlink(filepath.Join(root, "web"), filepath.Join(root, "linked")); e != nil {
		t.Fatal(e)
	}
	data, e := Discover(root, nil, 2)
	if e != nil || len(data.Candidates) != 3 {
		t.Fatalf("discovery: %+v %v", data, e)
	}
	found := map[string]catalog.Project{}
	for _, c := range data.Candidates {
		found[c.Project.ID] = c.Project
	}
	if found["web"].Ports[0] != 8833 || found["python"].Launch != "python3 start.py --no-browser" || found["golang"].Launch != "go run ." {
		t.Fatal(found)
	}
	again, e := Discover(root, []catalog.Project{found["web"]}, 2)
	if e != nil || len(again.Candidates) != 2 {
		t.Fatal("existing project rediscovered")
	}
}
func TestDiscoverySkipsSymlinkedManifestsAndInvalidPackages(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "package.json")
	testutil.Write(t, target, `{"scripts":{"dev":"echo bad"}}`)
	os.Symlink(target, filepath.Join(root, "package.json"))
	if c := inspectProject(fsutil.Canonical(root), nil); c != nil {
		t.Fatal("linked manifest read")
	}
	testutil.Write(t, filepath.Join(root, "bad/package.json"), `{"scripts":[]}`)
	data, e := Discover(root, nil, 2)
	if e != nil || len(data.Candidates) != 0 {
		t.Fatalf("invalid manifest accepted: %+v %v", data, e)
	}
}
func TestDiscoveryRejectsSystemSecretAndSymlinkRoots(t *testing.T) {
	home, _ := os.UserHomeDir()
	for _, path := range []string{"/", home} {
		if _, e := Discover(path, nil, 2); e == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	root := t.TempDir()
	for _, name := range []string{".env", ".ssh"} {
		path := filepath.Join(root, name)
		os.Mkdir(path, 0700)
		if _, e := Discover(path, nil, 2); e == nil {
			t.Fatal("private root accepted")
		}
	}
	link := filepath.Join(root, "link")
	os.Symlink(t.TempDir(), link)
	if _, e := Discover(link, nil, 2); e == nil {
		t.Fatal("linked root accepted")
	}
}
func TestDiscoveryDepthAndStaticReservations(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "index.html"), "<title>Static fixture</title>")
	p := testutil.FixtureProject(root + "-other")
	p.Ports = []int{8800, 8801}
	data, e := Discover(root, []catalog.Project{p}, 0)
	if e != nil || len(data.Candidates) != 1 || data.Candidates[0].Project.Ports[0] != 8802 {
		t.Fatal(data, e)
	}
	testutil.Write(t, filepath.Join(root, "deep/child/index.html"), "<title>Hidden by depth</title>")
	data, _ = Discover(root, nil, 0)
	if len(data.Candidates) != 1 || data.Scanned != 1 {
		t.Fatal("depth ignored")
	}
}
func TestSkillLinksMergeProvidersAndCyclesTerminate(t *testing.T) {
	home := t.TempDir()
	shared := filepath.Join(home, ".agents/skills/design")
	testutil.Write(t, filepath.Join(shared, "SKILL.md"), "---\nname: design\ndescription: >\n  A useful skill\n  for design.\n---\n")
	claude := filepath.Join(home, ".claude/skills")
	os.MkdirAll(claude, 0700)
	os.Symlink(shared, filepath.Join(claude, "design"))
	os.Symlink(claude, filepath.Join(shared, "cycle"))
	testutil.Write(t, filepath.Join(claude, "claude-only/SKILL.md"), "No metadata")
	testutil.Write(t, filepath.Join(home, ".claude/plugins/synced/plugin/skills/special/SKILL.md"), "No metadata")
	data := DiscoverSkills(nil, home)
	if len(data.Skills) != 3 {
		t.Fatalf("%+v", data)
	}
	for _, s := range data.Skills {
		if s.Name == "design" {
			if len(s.Providers) != 2 || len(s.Locations) != 2 || s.Description != "A useful skill for design." {
				t.Fatalf("linked metadata: %+v", s)
			}
		}
	}
}
