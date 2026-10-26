package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
)

var skipFolders = []string{".git", "node_modules", "vendor", ".venv", "venv", "dist", "build", "__pycache__", ".ssh", ".aws", ".gnupg", ".azure", ".config", ".codex", ".claude", ".cache", ".Trash", ".env"}

type Candidate struct {
	Project   catalog.Project `json:"project"`
	Reasons   []string        `json:"reasons"`
	NeedsPort bool            `json:"needsPort"`
}
type Discovery struct {
	Candidates []Candidate `json:"candidates"`
	Warnings   []string    `json:"warnings"`
	Scanned    int         `json:"scanned"`
	Limited    bool        `json:"limited"`
	Root       string      `json:"root"`
}

var scriptPort = regexp.MustCompile(`(?:--port[=\s]+|(?:^|\s)-p\s+|PORT=)(\d{2,5})`)
var pythonPort = regexp.MustCompile(`https?://(?:127\.0\.0\.1|localhost):(\d{2,5})`)
var pythonDefaultPort = regexp.MustCompile(`['"]--port['"].{0,80}?default\s*=\s*(\d{2,5})`)
var titleTag = regexp.MustCompile(`(?i)<title[^>]*>([^<]{1,160})</title>`)
var slugChars = regexp.MustCompile(`[^a-z0-9]+`)

func digest(text string) string { v := sha256.Sum256([]byte(text)); return hex.EncodeToString(v[:]) }
func matchedPort(rx *regexp.Regexp, source string) int {
	m := rx.FindStringSubmatch(source)
	if len(m) < 2 {
		return 0
	}
	v, _ := strconv.Atoi(m[1])
	return v
}
func inspectProject(root string, existing []catalog.Project) *Candidate {
	for _, p := range existing {
		if p.Path != "" && fsutil.Canonical(p.Path) == root {
			return nil
		}
	}
	name, description, command, manifest := filepath.Base(root), "", "", ""
	port := 0
	reasons := []string{}
	if source := fsutil.ReadText(filepath.Join(root, "package.json"), 65536); source != "" {
		var data struct {
			Name            string                     `json:"name"`
			Description     string                     `json:"description"`
			Scripts         map[string]json.RawMessage `json:"scripts"`
			Dependencies    map[string]any             `json:"dependencies"`
			DevDependencies map[string]any             `json:"devDependencies"`
		}
		if json.Unmarshal([]byte(source), &data) != nil {
			return nil
		}
		script, body := "", ""
		for _, s := range []string{"dev", "start", "serve", "preview"} {
			if b, ok := data.Scripts[s]; ok && json.Unmarshal(b, &body) == nil {
				script = s
				break
			}
		}
		if script == "" {
			return nil
		}
		manager := "npm"
		switch {
		case fsutil.Exists(filepath.Join(root, "pnpm-lock.yaml")):
			manager = "pnpm"
		case fsutil.Exists(filepath.Join(root, "yarn.lock")):
			manager = "yarn"
		case fsutil.Exists(filepath.Join(root, "bun.lock")) || fsutil.Exists(filepath.Join(root, "bun.lockb")):
			manager = "bun"
		}
		command = manager + " run " + script
		manifest = "package.json"
		if data.Name != "" {
			name = strings.ReplaceAll(filepath.Base(data.Name), "-", " ")
		}
		description = data.Description
		reasons = append(reasons, script+" script found in package.json")
		port = matchedPort(scriptPort, body)
		if port > 0 {
			reasons = append(reasons, "Port found in the package script")
		} else {
			for _, d := range []struct {
				name string
				port int
			}{{"vite", 5173}, {"next", 3000}, {"astro", 4321}} {
				if _, ok := data.Dependencies[d.name]; ok {
					port = d.port
				}
				if _, ok := data.DevDependencies[d.name]; ok {
					port = d.port
				}
				if port > 0 {
					reasons = append(reasons, "Framework default port; review before adding")
					break
				}
			}
		}
	} else if source := fsutil.ReadText(filepath.Join(root, "go.mod"), 65536); source != "" {
		// A module alone is not necessarily an executable web application.
		entry := ""
		if b := fsutil.ReadText(filepath.Join(root, "main.go"), 65536); strings.Contains(b, "package main") {
			entry = "."
		} else if b := fsutil.ReadText(filepath.Join(root, "cmd/server/main.go"), 65536); strings.Contains(b, "package main") {
			entry = "./cmd/server"
		}
		if entry == "" {
			return nil
		}
		command = "go run " + entry
		manifest = "go.mod"
		reasons = append(reasons, "Go executable found; review command and website port")
	} else if fsutil.ReadText(filepath.Join(root, "start.py"), 65536) != "" || fsutil.ReadText(filepath.Join(root, "server.py"), 65536) != "" {
		manifest = "start.py"
		source := fsutil.ReadText(filepath.Join(root, manifest), 65536)
		if source == "" {
			manifest = "server.py"
			source = fsutil.ReadText(filepath.Join(root, manifest), 65536)
		}
		command = "python3 " + manifest
		if strings.Contains(source, "--no-browser") {
			command += " --no-browser"
		}
		port = matchedPort(pythonPort, source)
		if port == 0 {
			port = matchedPort(pythonDefaultPort, source)
		}
		reasons = append(reasons, manifest+" launcher found")
		if port > 0 {
			reasons = append(reasons, "Port found in the Python launcher")
		}
	} else if source := fsutil.ReadText(filepath.Join(root, "index.html"), 65536); source != "" {
		for n := 8800; n < 8900; n++ {
			reserved := false
			for _, p := range existing {
				reserved = reserved || slices.Contains(p.Ports, n)
			}
			if !reserved {
				port = n
				break
			}
		}
		if port == 0 {
			return nil
		}
		command = fmt.Sprintf("python3 -m http.server %d --bind 127.0.0.1", port)
		manifest = "index.html"
		if m := titleTag.FindStringSubmatch(source); len(m) > 1 {
			name = strings.TrimSpace(m[1])
		}
		reasons = []string{"Static website found", "Suggested local port; checked again before launch"}
	} else {
		return nil
	}
	if port < 1 || port > 65535 {
		port = 0
	}
	slug := strings.Trim(slugChars.ReplaceAllString(strings.ToLower(filepath.Base(root)), "-"), "-")
	if len(slug) > 48 {
		slug = slug[:48]
	}
	if slug == "" {
		slug = "project"
	}
	for _, p := range existing {
		if p.ID == slug {
			slug += "-" + digest(root)[:7]
			break
		}
	}
	docs := []string{}
	for _, n := range []string{"README.md", "AGENTS.md", "CLAUDE.md"} {
		if fsutil.Exists(filepath.Join(root, n)) {
			docs = append(docs, n)
		}
	}
	if description == "" {
		description = "Local project in " + filepath.Base(root) + "."
	}
	if len([]rune(name)) > 160 {
		name = string([]rune(name)[:160])
	}
	if len([]rune(description)) > 4000 {
		description = string([]rune(description)[:4000])
	}
	p := catalog.Project{ID: slug, Name: name, Description: description, Kind: "web", Path: root, Ports: []int{}, Lifecycle: "active", Owner: os.Getenv("USER"), Launch: command, Docs: strings.Join(docs, "; "), Maintenance: "Discovered from " + manifest + ". Review the launch command and URL before starting."}
	if port > 0 {
		p.Ports = []int{port}
		p.URL = fmt.Sprintf("http://127.0.0.1:%d/", port)
	}
	return &Candidate{Project: p, Reasons: reasons, NeedsPort: port == 0}
}
func Discover(path string, existing []catalog.Project, depth int) (Discovery, error) {
	result := Discovery{Candidates: []Candidate{}, Warnings: []string{}}
	if path == "~" || strings.HasPrefix(path, "~/") {
		h, _ := os.UserHomeDir()
		path = h + strings.TrimPrefix(path, "~")
	}
	info, e := os.Lstat(path)
	if !filepath.IsAbs(path) || e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return result, errors.New("Choose an existing absolute project folder.")
	}
	root := fsutil.Canonical(path)
	home, _ := os.UserHomeDir()
	if root == string(os.PathSeparator) || root == fsutil.Canonical(home) {
		return result, errors.New("Choose a project or workspace folder, rather than your home or a system folder.")
	}
	for _, p := range strings.Split(root, string(os.PathSeparator)) {
		if slices.Contains(skipFolders, p) || strings.HasPrefix(p, ".env") {
			return result, errors.New("Choose a project folder outside private or dependency folders.")
		}
	}
	result.Root = root
	if depth < 0 {
		depth = 0
	}
	if depth > 3 {
		depth = 3
	}
	type item struct {
		path  string
		level int
	}
	stack := []item{{root, 0}}
	for len(stack) > 0 && result.Scanned < 300 && len(result.Candidates) < 40 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result.Scanned++
		if candidate := inspectProject(n.path, existing); candidate != nil {
			result.Candidates = append(result.Candidates, *candidate)
		}
		if n.level >= depth {
			continue
		}
		children, e := os.ReadDir(n.path)
		if e != nil {
			result.Warnings = append(result.Warnings, "Could not inspect "+filepath.Base(n.path)+": "+e.Error())
			continue
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Name() > children[j].Name() })
		for _, c := range children {
			if c.IsDir() && c.Type()&os.ModeSymlink == 0 && !strings.HasPrefix(c.Name(), ".") && !slices.Contains(skipFolders, c.Name()) {
				stack = append(stack, item{filepath.Join(n.path, c.Name()), n.level + 1})
			}
		}
	}
	result.Limited = len(stack) > 0
	return result, nil
}
