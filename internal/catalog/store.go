// Package catalog validates project records and updates the shared registry atomically.
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"workbench/internal/fsutil"
)

type Registry struct {
	Version  int       `json:"version"`
	Revision int       `json:"revision"`
	Projects []Project `json:"projects"`
}
type Store struct{ Root string }

func (s Store) Path() string { return filepath.Join(s.Root, "registry.json") }
func (s Store) Read() (Registry, error) {
	var r Registry
	b, e := fsutil.ReadBounded(s.Path(), 8<<20)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, fmt.Errorf("Invalid registry; restore a valid backup: %w", e)
	}
	if r.Version != 1 || r.Revision < 1 || r.Projects == nil {
		return r, errors.New("Unsupported or invalid registry. Restore a valid backup before editing.")
	}
	ids := map[string]bool{}
	for _, p := range r.Projects {
		if e = p.Validate(); e != nil {
			return r, e
		}
		if ids[p.ID] {
			return r, errors.New("Duplicate project IDs in registry.")
		}
		ids[p.ID] = true
	}
	return r, nil
}
func (s Store) WithLock(fn func() error) error {
	f, e := os.OpenFile(s.Path()+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}
func (s Store) Save(p Project, revision int) (Registry, error) {
	var result Registry
	if e := p.Validate(); e != nil {
		return result, e
	}
	e := s.WithLock(func() error {
		r, e := s.Read()
		if e != nil {
			return e
		}
		if revision != 0 && revision != r.Revision {
			return errors.New("The registry changed. Refresh before saving your edit.")
		}
		previous, e := os.ReadFile(s.Path())
		if e != nil {
			return e
		}
		p.UpdatedAt = now()
		if p.Ports == nil {
			p.Ports = []int{}
		}
		found := false
		for i, old := range r.Projects {
			if old.ID == p.ID {
				r.Projects[i] = p
				found = true
				break
			}
		}
		if !found {
			r.Projects = append(r.Projects, p)
		}
		if e = fsutil.AtomicWrite(filepath.Join(s.Root, "registry.previous.json"), previous); e != nil {
			return e
		}
		r.Revision++
		if e = fsutil.WriteJSON(s.Path(), r); e != nil {
			return e
		}
		result = r
		return nil
	})
	return result, e
}
func (s Store) Initialize(port int) error {
	return s.WithLock(func() error {
		if _, e := os.Lstat(s.Path()); e == nil {
			_, e = s.Read()
			return e
		} else if !os.IsNotExist(e) {
			return e
		}
		owner := os.Getenv("USER")
		p := Project{ID: "workbench", Name: "Workbench", Description: "Your local project workspace and launcher.", Kind: "web", Path: s.Root, URL: fmt.Sprintf("http://127.0.0.1:%d/", port), Ports: []int{port}, Lifecycle: "active", Owner: owner, Launch: "./workbench start", Maintenance: "Start reuses this installation. Launch logs are in .runtime/logs.", Docs: "README.md; docs/setup.md", UpdatedAt: now()}
		return fsutil.WriteJSON(s.Path(), Registry{Version: 1, Revision: 1, Projects: []Project{p}})
	})
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func Find(r Registry, id string) (Project, error) {
	for _, p := range r.Projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, errors.New("This project is no longer in the registry. Refresh the workspace.")
}
