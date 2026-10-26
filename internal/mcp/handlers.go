package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"workbench/internal/catalog"
	"workbench/internal/discovery"
	"workbench/internal/inventory"
)

const maxRegistrationBytes = 32 << 10

func decodeArguments(rawArguments json.RawMessage, target any) error {
	if len(rawArguments) == 0 {
		rawArguments = json.RawMessage("{}")
	}

	if bytes.Equal(bytes.TrimSpace(rawArguments), []byte("null")) {
		return errors.New("Tool arguments must be an object.")
	}

	decoder := json.NewDecoder(bytes.NewReader(rawArguments))
	decoder.DisallowUnknownFields()

	return decoder.Decode(target)
}

func (server *Server) call(name string, rawArguments json.RawMessage) (any, error) {
	switch name {
	case "list_projects":
		if err := decodeArguments(rawArguments, &struct{}{}); err != nil {
			return nil, err
		}
		return server.Store.Read()

	case "project_status":
		if err := decodeArguments(rawArguments, &struct{}{}); err != nil {
			return nil, err
		}
		return server.projectStatus()

	case "get_project", "launch_logs":
		var arguments struct {
			ProjectID string `json:"id"`
		}
		if err := decodeArguments(rawArguments, &arguments); err != nil {
			return nil, err
		}

		if !catalog.ValidID(arguments.ProjectID) {
			return nil, errors.New("A valid project ID is required.")
		}

		if name == "launch_logs" {
			return server.Supervisor.Logs(arguments.ProjectID)
		}

		registry, err := server.Store.Read()
		if err != nil {
			return nil, err
		}

		return catalog.Find(registry, arguments.ProjectID)

	case "check_port":
		return server.checkPort(rawArguments)

	case "discover_skills":
		return server.discoverSkills(rawArguments)

	case "register_project":
		return server.registerProject(rawArguments)

	default:
		return nil, fmt.Errorf("Unknown tool: %s", name)
	}
}

func (server *Server) projectStatus() (*inventory.Snapshot, error) {
	snapshot, err := inventory.Capture(server.Store, server.Supervisor.Inspector)
	if err != nil {
		return nil, err
	}

	server.Supervisor.Enrich(snapshot)
	for _, service := range snapshot.Services {
		service.CanStop = false
		service.StopToken = ""
	}

	return snapshot, nil
}

func (server *Server) checkPort(rawArguments json.RawMessage) (any, error) {
	var arguments struct {
		Port int `json:"port"`
	}
	if err := decodeArguments(rawArguments, &arguments); err != nil {
		return nil, err
	}

	if arguments.Port < 1 || arguments.Port > 65535 {
		return nil, errors.New("Port must be between 1 and 65535.")
	}

	snapshot, err := inventory.Capture(server.Store, server.Supervisor.Inspector)
	if err != nil {
		return nil, err
	}

	reservedBy := []string{}
	listeners := []*inventory.Service{}

	for _, project := range snapshot.Projects {
		if slices.Contains(project.Ports, arguments.Port) {
			reservedBy = append(reservedBy, project.Name)
		}
	}

	for _, service := range snapshot.Services {
		if slices.Contains(service.Ports, arguments.Port) {
			service.CanStop = false
			service.StopToken = ""
			listeners = append(listeners, service)
		}
	}

	return map[string]any{
		"port":            arguments.Port,
		"reservedBy":      reservedBy,
		"listeners":       listeners,
		"inspectionError": snapshot.InspectionError,
		"available":       len(reservedBy) == 0 && len(listeners) == 0 && snapshot.InspectionError == nil,
	}, nil
}

func (server *Server) discoverSkills(rawArguments json.RawMessage) (any, error) {
	var arguments struct {
		Provider string `json:"provider"`
	}
	if err := decodeArguments(rawArguments, &arguments); err != nil {
		return nil, err
	}

	if arguments.Provider != "" && arguments.Provider != "codex" && arguments.Provider != "claude" {
		return nil, errors.New("Provider must be codex or claude.")
	}

	registry, err := server.Store.Read()
	if err != nil {
		return nil, err
	}

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	discovered := discovery.DiscoverSkills(registry.Projects, homeDirectory)
	if arguments.Provider != "" {
		filteredSkills := []*discovery.Skill{}
		for _, skill := range discovered.Skills {
			if slices.Contains(skill.Providers, arguments.Provider) {
				filteredSkills = append(filteredSkills, skill)
			}
		}
		discovered.Skills = filteredSkills
	}

	return discovered, nil
}

func (server *Server) registerProject(rawArguments json.RawMessage) (any, error) {
	if !server.AllowWrites {
		return nil, errors.New("Catalog writes are disabled. Start MCP with --allow-writes to enable registration.")
	}

	if len(rawArguments) > maxRegistrationBytes {
		return nil, errors.New("Project registration is limited to 32 KB.")
	}

	var arguments struct {
		Project  catalog.Project `json:"project"`
		Revision int             `json:"revision"`
	}
	if err := decodeArguments(rawArguments, &arguments); err != nil {
		return nil, err
	}

	if arguments.Revision < 1 {
		return nil, errors.New("Read list_projects and supply its current revision before saving.")
	}

	registry, err := server.Store.Save(arguments.Project, arguments.Revision)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":       true,
		"id":       arguments.Project.ID,
		"revision": registry.Revision,
	}, nil
}
