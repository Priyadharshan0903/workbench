// Package distribution exports allowlisted source and runtime archives without personal data.
package distribution

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"workbench/internal/fsutil"
	"workbench/web"
)

// Package from an explicit source allowlist. Never traverse catalog/runtime data.
func SourceFiles(root string) ([]string, error) {
	files := []string{"go.mod", "go.sum", ".gitignore", "README.md", "AGENTS.md", "Makefile"}
	files = append(files, dependencyLicenses...)
	assets := []string{
		"web/static/vendor/HTMX-LICENSE", "web/static/vendor/README.md",
		"web/static/fonts/INTER-LICENSE.txt", "web/static/fonts/README.md",
		"web/templates/logs.html",
	}
	for _, filename := range web.PublicAssets() {
		assets = append(assets, "web/static/"+filename)
	}
	docs := []string{"docs/setup.md", "docs/design.md", "docs/architecture.md", "docs/agents.md"}
	for _, folder := range []string{"cmd", "internal", "web", "docs", "scripts"} {
		err := filepath.WalkDir(filepath.Join(root, folder), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errors.New("Source archives do not follow symlinks.")
			}
			if strings.HasPrefix(entry.Name(), ".") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(relative)
			allowed := false
			switch folder {
			case "cmd", "internal":
				allowed = filepath.Ext(name) == ".go"
			case "web":
				allowed = filepath.Ext(name) == ".go" || slices.Contains(assets, name)
			case "docs":
				allowed = slices.Contains(docs, name)
			case "scripts":
				allowed = name == "scripts/check-frontend.cjs"
			}
			if allowed {
				files = append(files, relative)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

var dependencyLicenses = []string{
	"third_party/licenses/cobra.txt",
	"third_party/licenses/pflag.txt",
	"third_party/licenses/mousetrap.txt",
	"third_party/licenses/goldmark.txt",
}

func Create(root, output string, runtimeOnly bool) error {
	files := []string{"workbench", "README.md", "docs/setup.md", "docs/agents.md", "web/static/vendor/HTMX-LICENSE", "web/static/fonts/INTER-LICENSE.txt"}
	files = append(files, dependencyLicenses...)
	var e error
	if !runtimeOnly {
		files, e = SourceFiles(root)
		if e != nil {
			return e
		}
	}
	if e = os.MkdirAll(filepath.Dir(output), 0755); e != nil {
		return e
	}
	temporary, e := os.CreateTemp(filepath.Dir(output), ".workbench-package-*")
	if e != nil {
		return e
	}
	defer os.Remove(temporary.Name())
	archive := zip.NewWriter(temporary)
	for _, name := range files {
		path := filepath.Join(root, name)
		info, e := os.Lstat(path)
		if e != nil {
			archive.Close()
			temporary.Close()
			return e
		}
		if !info.Mode().IsRegular() {
			archive.Close()
			temporary.Close()
			return fmt.Errorf("Not a regular source file: %s", name)
		}
		b, e := fsutil.ReadBounded(path, 128<<20)
		if e != nil {
			archive.Close()
			temporary.Close()
			return e
		}
		header, e := zip.FileInfoHeader(info)
		if e != nil {
			archive.Close()
			temporary.Close()
			return e
		}
		header.Name = "Workbench/" + filepath.ToSlash(name)
		header.Method = zip.Deflate
		writer, e := archive.CreateHeader(header)
		if e != nil {
			archive.Close()
			temporary.Close()
			return e
		}
		if _, e = writer.Write(b); e != nil {
			archive.Close()
			temporary.Close()
			return e
		}
	}
	if e = archive.Close(); e != nil {
		temporary.Close()
		return e
	}
	if e = temporary.Sync(); e != nil {
		temporary.Close()
		return e
	}
	if e = temporary.Close(); e != nil {
		return e
	}
	return os.Rename(temporary.Name(), output)
}
