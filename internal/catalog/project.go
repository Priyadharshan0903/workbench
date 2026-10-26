package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"workbench/internal/fsutil"
)

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	URL         string `json:"url"`
	Ports       []int  `json:"ports"`
	Lifecycle   string `json:"lifecycle"`
	Owner       string `json:"owner"`
	Launch      string `json:"launch"`
	Maintenance string `json:"maintenance"`
	Docs        string `json:"docs"`
	UpdatedAt   string `json:"updatedAt"`
}

// Reject unknown fields and null values rather than silently dropping corrupt data.
func (p *Project) UnmarshalJSON(b []byte) error {
	type plain Project
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil || fields == nil {
		return errors.New("Project must be an object.")
	}
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("Invalid %s.", key)
		}
	}
	var v plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	if v.Ports == nil {
		v.Ports = []int{}
	}
	*p = Project(v)
	return p.Validate()
}

var projectID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func (p Project) Validate() error {
	if !projectID.MatchString(p.ID) {
		return errors.New("ID must contain lowercase letters, numbers, and hyphens (up to 64 characters).")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("A project name is required.")
	}
	if !slices.Contains([]string{"web", "desktop", "cli", "library"}, p.Kind) {
		return errors.New("Choose web, desktop, cli, or library.")
	}
	if !slices.Contains([]string{"active", "paused", "archived"}, p.Lifecycle) {
		return errors.New("Choose active, paused, or archived.")
	}
	for key, value := range map[string]string{"id": p.ID, "name": p.Name, "description": p.Description, "kind": p.Kind, "path": p.Path, "url": p.URL, "lifecycle": p.Lifecycle, "owner": p.Owner, "launch": p.Launch, "maintenance": p.Maintenance, "docs": p.Docs, "updatedAt": p.UpdatedAt} {
		limit := 2000
		if key == "description" || key == "maintenance" {
			limit = 4000
		}
		if len([]rune(value)) > limit {
			return fmt.Errorf("Invalid %s.", key)
		}
	}
	seen := map[int]bool{}
	if len(p.Ports) > 30 {
		return errors.New("Use at most 30 ports.")
	}
	for _, port := range p.Ports {
		if port < 1 || port > 65535 || seen[port] {
			return errors.New("Ports must be distinct integers from 1 to 65535.")
		}
		seen[port] = true
	}
	if p.URL != "" {
		u, e := url.Parse(p.URL)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return errors.New("URL must be an http(s) address without credentials.")
		}
	}
	if p.Path != "" && (!filepath.IsAbs(p.Path) || strings.ContainsRune(p.Path, 0) || fsutil.SecretPath(p.Path)) {
		return errors.New("Use an absolute project path outside secret files.")
	}
	return nil
}

func ValidID(value string) bool { return projectID.MatchString(value) }
