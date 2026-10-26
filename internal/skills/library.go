// Package skills reads discovered skill bundles and manages reviewed local copies
// and instruction edits. It never runs a skill or configures an agent client.
package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"workbench/internal/catalog"
	"workbench/internal/discovery"
	"workbench/internal/fsutil"
)

const MaxContentBytes = 512000

var (
	ErrNotFound = errors.New("This skill or file is no longer available. Refresh the skills list.")
	ErrConflict = errors.New("The skill changed on disk. Reload it before saving or copying; your draft is still available.")
	ErrReadOnly = errors.New("This skill is managed or outside a writable skill folder. Make an editable copy instead.")
	ErrInvalid  = errors.New("Invalid skill request.")
	validID     = regexp.MustCompile(`^[a-f0-9]{24}$`)
	validName   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

type Library struct {
	Store catalog.Store
	Home  string
}

type File struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Readable bool   `json:"readable"`
}

type Destination struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type Document struct {
	Skill          *discovery.Skill `json:"skill"`
	File           string           `json:"file"`
	Content        string           `json:"content"`
	PreviewHTML    string           `json:"previewHtml"`
	Revision       string           `json:"revision"`
	BundleRevision string           `json:"bundleRevision"`
	Editable       bool             `json:"editable"`
	ReadOnlyReason string           `json:"readOnlyReason"`
	Files          []File           `json:"files"`
	Destinations   []Destination    `json:"destinations"`
	Warnings       []string         `json:"warnings"`
	BundleComplete bool             `json:"bundleComplete"`
}

type Preview struct {
	HTML string `json:"html"`
}

type SaveRequest struct {
	ID       string  `json:"id"`
	Revision string  `json:"revision"`
	Content  *string `json:"content"`
}

type SaveResult struct {
	OK         bool   `json:"ok"`
	Revision   string `json:"revision"`
	BackupPath string `json:"backupPath"`
}

type CopyRequest struct {
	ID          string `json:"id"`
	Revision    string `json:"revision"`
	Destination string `json:"destination"`
	Name        string `json:"name"`
}

type CopyResult struct {
	OK   bool   `json:"ok"`
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (library *Library) Discover() (discovery.Skills, error) {
	registry, err := library.Store.Read()
	if err != nil {
		return discovery.Skills{}, err
	}

	return discovery.DiscoverSkills(registry.Projects, library.Home), nil
}

func (library *Library) resolve(id string) (*discovery.Skill, error) {
	if !validID.MatchString(id) {
		return nil, ErrNotFound
	}

	discovered, err := library.Discover()
	if err != nil {
		return nil, err
	}

	for _, skill := range discovered.Skills {
		if skill.ID == id && !fsutil.SecretPath(skill.Path) {
			if filepath.Base(skill.Path) != "SKILL.md" {
				return nil, fmt.Errorf("%w The linked entry must resolve to a SKILL.md file to use this workspace.", ErrInvalid)
			}
			return skill, nil
		}
	}

	return nil, ErrNotFound
}

func (library *Library) View(id, filename string) (Document, error) {
	skill, err := library.resolve(id)
	if err != nil {
		return Document{}, err
	}

	bundle, err := readBundle(filepath.Dir(skill.Path))
	if err != nil {
		return Document{}, err
	}
	if filename == "" {
		filename = "SKILL.md"
	}

	contents, found := bundle.contents[filename]
	if !found {
		return Document{}, ErrNotFound
	}

	destinations, err := library.destinations()
	if err != nil {
		return Document{}, err
	}

	document := Document{
		Skill:          skill,
		File:           filename,
		Revision:       digest(contents),
		BundleRevision: bundle.revision,
		Files:          bundle.files,
		Warnings:       bundle.warnings,
		Destinations:   destinations,
		BundleComplete: bundle.complete,
	}

	if readable(contents) {
		document.Content = string(contents)
		if strings.EqualFold(filepath.Ext(filename), ".md") {
			preview, err := Render(document.Content)
			if err != nil {
				return Document{}, err
			}
			document.PreviewHTML = preview.HTML
		}
	}

	document.Editable = filename == "SKILL.md" && !managedSkill(skill) && library.editable(skill.Path, destinations)
	if !document.Editable {
		document.ReadOnlyReason = "Supporting files are available for viewing and copying. Edit SKILL.md to change instructions."
		if filename == "SKILL.md" {
			document.ReadOnlyReason = "Managed and external skills are read-only. Make a personal or project copy to edit."
		}
	}

	return document, nil
}

func Render(content string) (Preview, error) {
	if len(content) > MaxContentBytes || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return Preview{}, fmt.Errorf("%w Markdown must be UTF-8 text up to 500 KB.", ErrInvalid)
	}

	// Frontmatter remains in raw/copy output, while the preview shows instructions.
	markdownSource := strings.ReplaceAll(strings.TrimPrefix(content, "\ufeff"), "\r\n", "\n")
	if strings.HasPrefix(markdownSource, "---\n") {
		if end := strings.Index(markdownSource[4:], "\n---\n"); end >= 0 {
			markdownSource = markdownSource[4+end+5:]
		}
	}

	var output bytes.Buffer
	// Goldmark's default renderer omits raw HTML and unsafe URLs. Never enable WithUnsafe.
	markdown := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	if err := markdown.Convert([]byte(markdownSource), &output); err != nil {
		return Preview{}, err
	}

	return Preview{HTML: output.String()}, nil
}

func (library *Library) destinations() ([]Destination, error) {
	registry, err := library.Store.Read()
	if err != nil {
		return nil, err
	}

	destinations := []Destination{}
	add := func(root, label string) {
		root = filepath.Clean(root)
		canonicalRoot := fsutil.Canonical(root)
		if fsutil.SecretPath(root) || fsutil.SecretPath(canonicalRoot) || managedPath(canonicalRoot) {
			return
		}
		for _, existing := range destinations {
			if fsutil.Canonical(existing.Path) == fsutil.Canonical(root) {
				return
			}
		}
		destinations = append(destinations, Destination{
			ID:    digest([]byte(root + "\x00" + canonicalRoot))[:24],
			Label: label,
			Path:  canonicalRoot,
		})
	}

	add(filepath.Join(library.Home, ".codex/skills"), "Personal · Codex")
	add(filepath.Join(library.Home, ".claude/skills"), "Personal · Claude")
	add(filepath.Join(library.Home, ".agents/skills"), "Personal · Shared")
	for _, project := range registry.Projects {
		if !fsutil.IsDir(project.Path) || strings.HasSuffix(project.Path, ".app") {
			continue
		}
		add(filepath.Join(project.Path, ".codex/skills"), project.Name+" · Codex")
		add(filepath.Join(project.Path, ".claude/skills"), project.Name+" · Claude")
		add(filepath.Join(project.Path, ".agents/skills"), project.Name+" · Shared")
	}

	return destinations, nil
}

func (library *Library) editable(path string, destinations []Destination) bool {
	if managedPath(path) || fsutil.SecretPath(path) {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0200 == 0 {
		return false
	}
	for _, destination := range destinations {
		if fsutil.Within(path, destination.Path) {
			return true
		}
	}

	return false
}

func managedPath(path string) bool {
	path = "/" + filepath.ToSlash(filepath.Clean(path)) + "/"
	return strings.Contains(path, "/.codex/plugins/") || strings.Contains(path, "/.claude/plugins/") ||
		strings.Contains(path, "/.system/")
}

func managedSkill(skill *discovery.Skill) bool {
	for _, scope := range skill.Scopes {
		if scope == "Plugin cache" || scope == "Synced plugin" {
			return true
		}
	}
	return managedPath(skill.Path)
}

func digest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func readable(contents []byte) bool {
	return len(contents) <= MaxContentBytes && utf8.Valid(contents) && !bytes.ContainsRune(contents, 0)
}

func readRegular(root *os.Root, name string, limit int64) ([]byte, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, nil, fmt.Errorf("%w File is unavailable, linked or too large: %s", ErrInvalid, name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, nil, ErrConflict
	}
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(contents)) > limit {
		return nil, nil, fmt.Errorf("%w Could not read bounded file: %s", ErrInvalid, name)
	}

	return contents, openedInfo, nil
}
