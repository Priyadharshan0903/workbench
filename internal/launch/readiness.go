package launch

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"workbench/internal/fsutil"
	"workbench/internal/inventory"
)

func (s *Supervisor) CheckHTTP(value string) inventory.Health {
	if !localURL(value) {
		return inventory.Health{State: "unchecked", Message: "Website readiness is checked only for local URLs."}
	}
	s.healthMu.Lock()
	cached, ok := s.cache[value]
	s.healthMu.Unlock()
	if ok && time.Since(cached.at) < 4*time.Second {
		return cached.value
	}
	started := time.Now()
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 1200 * time.Millisecond, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	result := inventory.Health{State: "unreachable", Message: "The local website is not responding yet. Check the URL and project logs."}
	response, e := client.Get(value)
	if e == nil {
		response.Body.Close()
		result.Code = response.StatusCode
		result.State = "error"
		if response.StatusCode >= 200 && response.StatusCode < 400 {
			result.State = "ready"
		}
		result.Message = fmt.Sprintf("The local URL returned HTTP %d.", response.StatusCode)
	}
	result.LatencyMS = time.Since(started).Milliseconds()
	s.healthMu.Lock()
	if len(s.cache) > 512 {
		s.cache = map[string]cachedHealth{}
	}
	s.cache[value] = cachedHealth{time.Now(), result}
	s.healthMu.Unlock()
	return result
}
func publicRun(r *RunRecord) map[string]any {
	if r == nil {
		return nil
	}
	v := map[string]any{"runId": r.RunID, "startedAt": r.StartedAt, "phase": r.Phase}
	if r.UpdatedAt != "" {
		v["updatedAt"] = r.UpdatedAt
	}
	if r.FinishedAt != "" {
		v["finishedAt"] = r.FinishedAt
	}
	if r.ExitCode != nil {
		v["exitCode"] = *r.ExitCode
	}
	return v
}
func (s *Supervisor) Enrich(state *inventory.Snapshot) {
	var wg sync.WaitGroup
	limit := make(chan struct{}, 8)
	for _, p := range state.Projects {
		if p.Kind == "web" && (len(p.Services) > 0 || len(p.Conflicts) > 0) && p.URL != "" {
			wg.Add(1)
			go func(p *inventory.ProjectView) {
				defer wg.Done()
				limit <- struct{}{}
				defer func() { <-limit }()
				v := s.CheckHTTP(p.URL)
				p.Health = &v
			}(p)
		}
	}
	wg.Wait()
	for _, p := range state.Projects {
		r := s.ReadRun(p.ID)
		p.Run = publicRun(r)
		p.CanLaunch = p.Kind == "web" && p.ID != "workbench" && p.Lifecycle == "active"
		p.LaunchSetupRequired = p.CanLaunch && validateWebsite(p.Project) != nil
		message := ""
		switch p.Status {
		case "unknown":
			message = "Process inspection is unavailable. Ownership and readiness cannot be confirmed."
		case "conflict":
			unverified := true
			for _, v := range p.Conflicts {
				unverified = unverified && v.ProjectID == nil
			}
			if unverified {
				p.Status = "unverified"
				message = "A listener uses this port, but its project ownership is unverified."
			} else {
				message = "An expected port is used by another registered project."
			}
		case "running":
			if p.Kind == "web" && p.Health != nil {
				p.Status = "unhealthy"
				if p.Health.State == "ready" {
					p.Status = "ready"
				} else if r != nil && runAge(r) < 45*time.Second {
					p.Status = "starting"
				}
				message = p.Health.Message
			}
		case "stopped":
			if r != nil {
				switch {
				case r.Phase == "failed":
					p.Status = "failed"
					logs, _ := s.Logs(p.ID)
					code := "unknown"
					if r.ExitCode != nil {
						code = strconv.Itoa(*r.ExitCode)
					}
					message = diagnose(logs.Text, code)
				case s.ActiveRun(r):
					p.Status = "unhealthy"
					message = "The command is still running, but no website appeared. Check its URL, port, and logs."
					if runAge(r) < 45*time.Second {
						p.Status = "starting"
						message = "Waiting for the local website."
					}
				case r.Phase == "completed" && runAge(r) < 45*time.Second:
					p.Status = "starting"
					message = "The launcher finished. Waiting for the website to become available."
				case r.Phase == "completed":
					message = "The last launch has finished. Start again to reopen, or inspect its logs and settings."
				}
			}
		}
		if message == "" && p.LaunchSetupRequired && strings.TrimSpace(p.Launch) != "" {
			message = "Add a local website URL and expected port, then review the launch command to start this project."
		}
		if p.ExtraServices {
			message = strings.TrimSpace(message + " Multiple processes are associated with this project. Inspect Diagnostics before stopping one.")
		}
		p.StatusMessage = message
	}
	roots := []string{}
	for _, p := range state.Projects {
		if p.Path != "" && fsutil.IsDir(p.Path) && !strings.HasSuffix(p.Path, ".app") {
			root := filepath.Dir(p.Path)
			if !slices.Contains(roots, root) {
				roots = append(roots, root)
			}
		}
	}
	sort.Strings(roots)
	if len(roots) > 8 {
		roots = roots[:8]
	}
	state.DiscoveryRoots = roots
}
