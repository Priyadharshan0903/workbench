package distribution

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"workbench/internal/testutil"
)

func TestSourceArchiveExcludesPersonalDataAndContainsBuildInputs(t *testing.T) {
	root := t.TempDir()
	testutil.Write(t, filepath.Join(root, "web/static/skills.js"), "fixture")
	for _, name := range []string{"web/static/js/shared/dom.js", "web/static/css/tokens.css", "scripts/check-frontend.cjs", "tests/ui/helpers/dashboard.cjs"} {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	testutil.Write(t, filepath.Join(root, "web/static/js/shared/private.js"), "personal-sentinel")
	for _, name := range append([]string{"go.sum"}, dependencyLicenses...) {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	for _, name := range []string{"go.mod", ".gitignore", "cmd/workbench/main.go", "internal/app/cli_test.go", "Open Workbench.command", "README.md", "docs/setup.md", "docs/migration.md", "AGENTS.md", "Makefile", "web/static/index.html", "web/static/app.js", "web/static/vendor/htmx.min.js", "web/static/vendor/HTMX-LICENSE", "web/static/fonts/InterVariable.woff2", "web/static/fonts/INTER-LICENSE.txt", "tests/ui/interface.cjs", "tests/ui/htmx.cjs", "web/embed.go", "web/templates/logs.html", "scripts/test-ui.sh", "docs/architecture.md"} {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	for _, name := range []string{"registry.json", "registry.previous.json", ".runtime/local.json", ".runtime/logs/private.log", ".env", ".env.private", "start.py", "web/static/.env", "web/static/.env.private.js", "web/static/private-browser-preferences.js", "tests/__pycache__/private.pyc", "legacy/python/server.py", "scripts/compat/start.py"} {
		testutil.Write(t, filepath.Join(root, name), "personal-sentinel")
	}
	archivePath := filepath.Join(t.TempDir(), "source.zip")
	if e := Create(root, archivePath, false); e != nil {
		t.Fatal(e)
	}
	archive, e := zip.OpenReader(archivePath)
	if e != nil {
		t.Fatal(e)
	}
	defer archive.Close()
	names := []string{}
	for _, f := range archive.File {
		names = append(names, f.Name)
		if strings.Contains(f.Name, "registry") || strings.Contains(f.Name, ".runtime") || strings.Contains(f.Name, ".env") || strings.Contains(f.Name, "private-browser") || strings.HasSuffix(f.Name, ".py") {
			t.Fatal("private or legacy file archived", f.Name)
		}
	}
	for _, name := range []string{"web/static/skills.js", "web/static/js/shared/dom.js", "web/static/css/tokens.css", "scripts/check-frontend.cjs", "tests/ui/helpers/dashboard.cjs", "third_party/licenses/goldmark.txt"} {
		if !slices.Contains(names, "Workbench/"+name) {
			t.Fatal("missing skill workspace build input", name)
		}
	}
	if slices.Contains(names, "Workbench/web/static/js/shared/private.js") {
		t.Fatal("unlisted browser module entered the source archive")
	}
	for _, name := range []string{"go.mod", "go.sum", "third_party/licenses/cobra.txt", "third_party/licenses/pflag.txt", "third_party/licenses/mousetrap.txt", "cmd/workbench/main.go", "internal/app/cli_test.go", "docs/migration.md", "web/static/vendor/htmx.min.js", "web/static/vendor/HTMX-LICENSE", "web/static/fonts/InterVariable.woff2", "web/static/fonts/INTER-LICENSE.txt", "tests/ui/htmx.cjs", "web/embed.go", "web/templates/logs.html", "scripts/test-ui.sh", "docs/architecture.md"} {
		if !slices.Contains(names, "Workbench/"+name) {
			t.Fatal("missing build input", name)
		}
	}
}
func TestRuntimeArchivePreservesExecutableMode(t *testing.T) {
	root := t.TempDir()
	for _, name := range dependencyLicenses {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	for _, name := range []string{"workbench", "Open Workbench.command", "README.md", "docs/setup.md", "docs/migration.md", "docs/agents.md", "web/static/vendor/HTMX-LICENSE", "web/static/fonts/INTER-LICENSE.txt"} {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	os.Chmod(filepath.Join(root, "workbench"), 0755)
	archivePath := filepath.Join(t.TempDir(), "runtime.zip")
	if e := Create(root, archivePath, true); e != nil {
		t.Fatal(e)
	}
	archive, e := zip.OpenReader(archivePath)
	if e != nil {
		t.Fatal(e)
	}
	defer archive.Close()
	if len(archive.File) != 8+len(dependencyLicenses) {
		t.Fatal("unexpected runtime files")
	}
	for _, f := range archive.File {
		if f.Name == "Workbench/workbench" && f.Mode().Perm()&0111 == 0 {
			t.Fatal("executable mode lost")
		}
	}
}
func TestSourceArchiveRefusesSymlinkAssets(t *testing.T) {
	root := t.TempDir()
	for _, name := range append([]string{"go.sum"}, dependencyLicenses...) {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	for _, name := range []string{"go.mod", ".gitignore", "Open Workbench.command", "README.md", "docs/setup.md", "docs/migration.md", "AGENTS.md", "Makefile"} {
		testutil.Write(t, filepath.Join(root, name), "fixture")
	}
	for _, dir := range []string{"cmd", "internal", "web/static", "docs", "tests/ui", "scripts"} {
		os.MkdirAll(filepath.Join(root, dir), 0700)
	}
	os.MkdirAll(filepath.Join(root, "tests"), 0700)
	os.Symlink(filepath.Join(t.TempDir(), "private.js"), filepath.Join(root, "web/static/linked.js"))
	if e := Create(root, filepath.Join(t.TempDir(), "source.zip"), false); e == nil {
		t.Fatal("symlink archived")
	}
}
