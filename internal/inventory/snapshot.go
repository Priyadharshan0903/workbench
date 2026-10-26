package inventory

import (
	"time"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
)

type Health struct {
	State     string `json:"state"`
	Message   string `json:"message"`
	Code      int    `json:"code,omitempty"`
	LatencyMS int64  `json:"latencyMs"`
}

type ProjectView struct {
	catalog.Project
	Services            []*Service     `json:"services"`
	Conflicts           []*Service     `json:"conflicts"`
	PathExists          bool           `json:"pathExists"`
	ExtraServices       bool           `json:"extraServices"`
	Status              string         `json:"status"`
	StatusMessage       string         `json:"statusMessage"`
	CanLaunch           bool           `json:"canLaunch"`
	LaunchSetupRequired bool           `json:"launchSetupRequired"`
	Run                 map[string]any `json:"run"`
	Health              *Health        `json:"health"`
}
type Snapshot struct {
	Revision        int             `json:"revision"`
	Projects        []*ProjectView  `json:"projects"`
	Services        []*Service      `json:"services"`
	InspectionError *string         `json:"inspectionError"`
	ScannedAt       string          `json:"scannedAt"`
	RegistryPath    string          `json:"registryPath"`
	Features        map[string]bool `json:"features,omitempty"`
	DiscoveryRoots  []string        `json:"discoveryRoots"`
	ControlWarnings []string        `json:"controlWarnings"`
	CSRFToken       string          `json:"csrfToken,omitempty"`
}

func Capture(s catalog.Store, i Inspector) (*Snapshot, error) {
	r, e := s.Read()
	if e != nil {
		return nil, e
	}
	services, inspectErr := i.Services(r.Projects)
	var msg *string
	if inspectErr != nil {
		v := inspectErr.Error()
		msg = &v
		services = []*Service{}
	}
	result := &Snapshot{Revision: r.Revision, Projects: []*ProjectView{}, Services: services, InspectionError: msg, ScannedAt: now(), RegistryPath: s.Path(), DiscoveryRoots: []string{}, ControlWarnings: []string{}}
	for _, p := range r.Projects {
		v := &ProjectView{Project: p, Services: []*Service{}, Conflicts: []*Service{}, PathExists: p.Path != "" && fsutil.Exists(p.Path)}
		for _, service := range services {
			if service.ProjectID != nil && *service.ProjectID == p.ID {
				v.Services = append(v.Services, service)
				v.ExtraServices = v.ExtraServices || len(service.UnexpectedPorts) > 0
			} else if Overlap(service.Ports, p.Ports) {
				v.Conflicts = append(v.Conflicts, service)
			}
		}
		v.ExtraServices = v.ExtraServices || len(v.Services) > 1
		switch {
		case inspectErr != nil:
			v.Status = "unknown"
		case len(v.Conflicts) > 0:
			v.Status = "conflict"
		case len(v.Services) > 0:
			v.Status = "running"
		case p.Path != "" && !v.PathExists:
			v.Status = "unlocated"
		case p.Kind == "web" && len(p.Ports) > 0:
			v.Status = "stopped"
		case v.PathExists:
			if p.Kind == "desktop" {
				v.Status = "installed"
			} else {
				v.Status = "available"
			}
		default:
			v.Status = "unlocated"
		}
		result.Projects = append(result.Projects, v)
	}
	return result, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
