// Package inventory inspects local TCP ownership and derives workspace snapshots.
package inventory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
)

type Service struct {
	PID             int      `json:"pid"`
	Name            string   `json:"name"`
	Endpoints       []string `json:"endpoints"`
	Ports           []int    `json:"ports"`
	CWD             string   `json:"cwd"`
	ProjectID       *string  `json:"projectId"`
	ProjectName     *string  `json:"projectName"`
	UnexpectedPorts []int    `json:"unexpectedPorts"`
	Self            bool     `json:"self"`
	CanStop         bool     `json:"canStop"`
	StopToken       string   `json:"stopToken,omitempty"`
	StopUnavailable string   `json:"stopUnavailable,omitempty"`
}
type Inspector struct {
	Run func(...string) (string, error)
	PID int
}

func NewInspector() Inspector { return Inspector{Run: RunCommand, PID: os.Getpid()} }
func RunCommand(argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	e := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if e != nil {
		var exit *exec.ExitError
		if errors.As(e, &exit) && exit.ExitCode() == 1 && out.Len() == 0 && stderr.Len() == 0 {
			return "", nil
		}
		msg := stderr.String()
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", fmt.Errorf("Local process inspection failed (%s): %s (%w)", argv[0], strings.TrimSpace(msg), e)
	}
	return out.String(), nil
}
func utility(name, fallback string) string {
	p, e := exec.LookPath(name)
	if e == nil {
		return p
	}
	return fallback
}
func parseListeners(output string) ([]*Service, error) {
	result := []*Service{}
	byPID := map[int]*Service{}
	var current *Service
	for _, line := range strings.Split(output, "\n") {
		if len(line) < 2 {
			continue
		}
		value := line[1:]
		switch line[0] {
		case 'p':
			pid, e := strconv.Atoi(value)
			if e != nil || pid <= 0 {
				return nil, errors.New("Invalid PID in listener inspection.")
			}
			current = byPID[pid]
			if current == nil {
				current = &Service{PID: pid, Endpoints: []string{}, Ports: []int{}, UnexpectedPorts: []int{}}
				byPID[pid] = current
				result = append(result, current)
			}
		case 'c':
			if current != nil {
				current.Name = value
			}
		case 'n':
			if current != nil {
				colon := strings.LastIndex(value, ":")
				if colon >= 0 {
					port, e := strconv.Atoi(value[colon+1:])
					if e == nil && port > 0 && port <= 65535 && !slices.Contains(current.Endpoints, value) {
						current.Endpoints = append(current.Endpoints, value)
						if !slices.Contains(current.Ports, port) {
							current.Ports = append(current.Ports, port)
						}
					}
				}
			}
		}
	}
	for _, s := range result {
		sort.Ints(s.Ports)
	}
	return result, nil
}
func Overlap(a, b []int) bool {
	for _, p := range a {
		if slices.Contains(b, p) {
			return true
		}
	}
	return false
}
func (i Inspector) Services(projects []catalog.Project) ([]*Service, error) {
	output, e := i.Run(utility("lsof", "/usr/sbin/lsof"), "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpcn")
	if e != nil {
		return nil, e
	}
	services, e := parseListeners(output)
	if e != nil {
		return nil, e
	}
	if len(services) > 0 {
		pids := []string{}
		for _, s := range services {
			pids = append(pids, strconv.Itoa(s.PID))
		}
		out, e := i.Run(utility("lsof", "/usr/sbin/lsof"), "-a", "-p", strings.Join(pids, ","), "-d", "cwd", "-Fn")
		if e != nil {
			return nil, e
		}
		cwds := map[int]string{}
		pid := 0
		for _, line := range strings.Split(out, "\n") {
			if len(line) < 2 {
				continue
			}
			if line[0] == 'p' {
				pid, _ = strconv.Atoi(line[1:])
			} else if line[0] == 'n' && pid > 0 {
				cwds[pid] = line[1:]
			}
		}
		for _, s := range services {
			s.CWD = cwds[s.PID]
		}
	}
	for _, s := range services {
		var best *catalog.Project
		for idx := range projects {
			p := &projects[idx]
			if p.Path != "" && fsutil.Within(s.CWD, p.Path) {
				if best == nil || len(p.Path) > len(best.Path) || (len(p.Path) == len(best.Path) && Overlap(s.Ports, p.Ports) && !Overlap(s.Ports, best.Ports)) {
					best = p
				}
			}
		}
		if best != nil {
			id, name := best.ID, best.Name
			s.ProjectID = &id
			s.ProjectName = &name
			for _, port := range s.Ports {
				if !slices.Contains(best.Ports, port) {
					s.UnexpectedPorts = append(s.UnexpectedPorts, port)
				}
			}
		}
		s.Self = s.PID == i.PID
		s.CanStop = best != nil && best.Kind == "web" && !s.Self
	}
	return services, nil
}
func (i Inspector) Birth(pid int) (string, error) {
	if pid <= 1 {
		return "", errors.New("Invalid process identity.")
	}
	out, e := i.Run(utility("ps", "/bin/ps"), "-p", strconv.Itoa(pid), "-o", "lstart=")
	if e != nil {
		return "", e
	}
	if strings.TrimSpace(out) == "" {
		return "", errors.New("Process is no longer running.")
	}
	return strings.TrimSpace(out), nil
}
