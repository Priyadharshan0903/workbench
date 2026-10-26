package inventory

import "slices"

// PortCheck describes reservations and listener evidence from one snapshot.
type PortCheck struct {
	Port            int        `json:"port"`
	ReservedBy      []string   `json:"reservedBy"`
	Listeners       []*Service `json:"listeners"`
	InspectionError *string    `json:"inspectionError"`
}

func (check PortCheck) Available() bool {
	return len(check.ReservedBy) == 0 && len(check.Listeners) == 0 && check.InspectionError == nil
}

func (snapshot *Snapshot) CheckPort(port int) PortCheck {
	check := PortCheck{
		Port:            port,
		ReservedBy:      []string{},
		Listeners:       []*Service{},
		InspectionError: snapshot.InspectionError,
	}

	for _, project := range snapshot.Projects {
		if slices.Contains(project.Ports, port) {
			check.ReservedBy = append(check.ReservedBy, project.Name)
		}
	}

	for _, service := range snapshot.Services {
		if slices.Contains(service.Ports, port) {
			check.Listeners = append(check.Listeners, service)
		}
	}

	return check
}
