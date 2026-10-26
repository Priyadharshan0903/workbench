package discovery

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
)

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	Providers   []string `json:"providers"`
	Scopes      []string `json:"scopes"`
	Locations   []string `json:"locations"`
}
type Skills struct {
	Skills    []*Skill `json:"skills"`
	Warnings  []string `json:"warnings"`
	ScannedAt string   `json:"scannedAt"`
	Note      string   `json:"note"`
}
type skillRoot struct {
	path, provider, scope string
	depth                 int
}

func skillRoots(projects []catalog.Project, home string) []skillRoot {
	roots := []skillRoot{{filepath.Join(home, ".codex/skills"), "codex", "Personal", 4}, {filepath.Join(home, ".agents/skills"), "codex", "Shared", 3}, {filepath.Join(home, ".claude/skills"), "claude", "Personal", 5}, {filepath.Join(home, ".codex/plugins/cache"), "codex", "Plugin cache", 8}, {filepath.Join(home, ".claude/plugins/cache"), "claude", "Plugin cache", 8}, {filepath.Join(home, ".claude/plugins/synced"), "claude", "Synced plugin", 7}}
	add := func(path, scope string) {
		for _, v := range []struct{ folder, provider string }{{".codex/skills", "codex"}, {".agents/skills", "codex"}, {".claude/skills", "claude"}} {
			roots = append(roots, skillRoot{filepath.Join(path, v.folder), v.provider, scope, 4})
		}
	}
	for _, p := range projects {
		if p.Path != "" && fsutil.IsDir(p.Path) && !strings.HasSuffix(p.Path, ".app") {
			add(p.Path, p.Name)
			parent := filepath.Dir(p.Path)
			if fsutil.IsDir(filepath.Join(parent, ".git")) {
				add(parent, filepath.Base(parent))
			}
		}
	}
	return roots
}

var metadataKey = regexp.MustCompile(`^(name|description):\s*(.*)$`)

func skillMetadata(text, fallback string) (string, string) {
	fields := map[string]string{}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.HasPrefix(text, "---\n") {
		parts := strings.SplitN(text, "---", 3)
		if len(parts) == 3 {
			key := ""
			for _, line := range strings.Split(parts[1], "\n") {
				if m := metadataKey.FindStringSubmatch(line); len(m) > 0 {
					key = m[1]
					value := strings.TrimSpace(m[2])
					if slices.Contains([]string{">", "|", ">-", "|-"}, value) {
						value = ""
					}
					fields[key] = strings.Trim(value, "\"'")
				} else if key != "" && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && strings.TrimSpace(line) != "" {
					fields[key] += " " + strings.TrimSpace(line)
				} else {
					key = ""
				}
			}
		}
	}
	name, desc := strings.TrimSpace(fields["name"]), strings.TrimSpace(fields["description"])
	if name == "" {
		name = fallback
	}
	if desc == "" {
		desc = "No description recorded in SKILL.md."
	}
	return name, desc
}
func DiscoverSkills(projects []catalog.Project, home string) Skills {
	result := Skills{Skills: []*Skill{}, Warnings: []string{}, ScannedAt: now(), Note: "Discovery shows files on disk, not proof a skill is enabled in the current agent session. Plugin caches may contain older versions."}
	byPath := map[string]*Skill{}
	for _, root := range skillRoots(projects, home) {
		seen := map[string]bool{}
		type entry struct {
			path  string
			level int
		}
		stack := []entry{{root.path, 0}}
		count := 0
		for len(stack) > 0 && count < 12000 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			real, e := filepath.EvalSymlinks(n.path)
			if e != nil || seen[real] || fsutil.SecretPath(real) {
				continue
			}
			seen[real] = true
			count++
			file := filepath.Join(n.path, "SKILL.md")
			if fsutil.Exists(file) {
				actual, e := filepath.EvalSymlinks(file)
				if e != nil || fsutil.SecretPath(actual) {
					continue
				}
				item := byPath[actual]
				if item == nil {
					b, e := fsutil.ReadBounded(actual, 512000)
					if e != nil {
						result.Warnings = append(result.Warnings, "Cannot read "+file+": "+e.Error())
						continue
					}
					name, desc := skillMetadata(string(b), filepath.Base(n.path))
					item = &Skill{ID: digest(actual)[:24], Name: name, Description: desc, Path: actual, Providers: []string{}, Scopes: []string{}, Locations: []string{}}
					byPath[actual] = item
					result.Skills = append(result.Skills, item)
				}
				if !slices.Contains(item.Providers, root.provider) {
					item.Providers = append(item.Providers, root.provider)
				}
				if !slices.Contains(item.Scopes, root.scope) {
					item.Scopes = append(item.Scopes, root.scope)
				}
				if !slices.Contains(item.Locations, file) {
					item.Locations = append(item.Locations, file)
				}
				continue
			}
			if n.level >= root.depth {
				continue
			}
			children, e := os.ReadDir(n.path)
			if e != nil {
				continue
			}
			for _, c := range children {
				if slices.Contains([]string{".git", "node_modules", "vendor", ".env"}, c.Name()) || strings.HasPrefix(c.Name(), ".env.") {
					continue
				}
				child := filepath.Join(n.path, c.Name())
				if fsutil.IsDir(child) {
					stack = append(stack, entry{child, n.level + 1})
				}
			}
		}
		if len(stack) > 0 {
			result.Warnings = append(result.Warnings, "Skill traversal limit reached: "+root.path)
		}
	}
	sort.Slice(result.Skills, func(i, j int) bool {
		a, b := result.Skills[i], result.Skills[j]
		if strings.ToLower(a.Name) == strings.ToLower(b.Name) {
			return a.Path < b.Path
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return result
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
