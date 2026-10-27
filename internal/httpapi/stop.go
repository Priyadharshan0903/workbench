package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"syscall"

	"workbench/internal/inventory"
)

// stopOwner preserves registered-project controls and permits reviewed stops of
// unregistered macOS listeners only when their owner and working folder are known.
func (app *App) stopOwner(service *inventory.Service) (*int, error) {
	if service.PID <= 1 || service.Self || service.PID == app.Supervisor.Inspector.PID {
		return nil, errors.New("This process is protected from Stop.")
	}

	if service.ProjectID != nil {
		if !service.CanStop {
			return nil, errors.New("Only registered web services can be stopped.")
		}
		return nil, nil
	}

	if app.platform != "darwin" {
		return nil, errors.New("Stopping unregistered listeners is supported on macOS only.")
	}
	if !filepath.IsAbs(service.CWD) || filepath.Clean(service.CWD) == "/" {
		return nil, errors.New("Cannot review Stop without a specific working folder.")
	}

	uid, err := app.Supervisor.Inspector.OwnerUID(service.PID)
	if err != nil {
		return nil, err
	}
	if uid == 0 || uid != os.Geteuid() {
		return nil, errors.New("Only processes owned by your macOS user can be stopped here.")
	}

	return &uid, nil
}

func (app *App) stopToken(service *inventory.Service) (string, error) {
	uid, err := app.stopOwner(service)
	if err != nil {
		return "", err
	}

	birth, err := app.Supervisor.Inspector.Birth(service.PID)
	if err != nil {
		return "", err
	}

	endpoints := append([]string{}, service.Endpoints...)
	sort.Strings(endpoints)
	identity, err := json.Marshal(processIdentity{
		PID:       service.PID,
		Name:      service.Name,
		Birth:     birth,
		CWD:       service.CWD,
		Endpoints: endpoints,
		ProjectID: service.ProjectID,
		UID:       uid,
	})
	if err != nil {
		return "", err
	}

	signature := hmac.New(sha256.New, []byte(app.SigningKey))
	signature.Write(identity)
	return hex.EncodeToString(signature.Sum(nil)), nil
}

func (app *App) prepareStopControls(state *inventory.Snapshot) {
	for _, service := range state.Services {
		registeredStop := service.CanStop
		externalStop := app.platform == "darwin" && service.ProjectID == nil && !service.Self
		if !registeredStop && !externalStop {
			continue
		}

		token, err := app.stopToken(service)
		if err == nil {
			service.CanStop = true
			service.StopToken = token
			continue
		}

		service.CanStop = false
		service.StopUnavailable = "Cannot verify process identity: " + err.Error()
		if registeredStop {
			warning := "Stop controls are unavailable because process identity cannot be verified. Retry from a normal desktop session."
			if !slices.Contains(state.ControlWarnings, warning) {
				state.ControlWarnings = append(state.ControlWarnings, warning)
			}
		}
	}
}

func (app *App) Stop(pid int, token string) (ActionResult, error) {
	if pid <= 1 || token == "" {
		return ActionResult{}, errors.New("Choose a currently listed service and review Stop first.")
	}

	state, err := inventory.Capture(app.Supervisor.Store, app.Supervisor.Inspector)
	if err != nil {
		return ActionResult{}, err
	}
	if state.InspectionError != nil {
		return ActionResult{}, errors.New("Process inspection is unavailable; no stop was attempted.")
	}

	for _, service := range state.Services {
		if service.PID != pid {
			continue
		}

		expected, err := app.stopToken(service)
		if err != nil || !hmac.Equal([]byte(token), []byte(expected)) {
			return ActionResult{}, errors.New("The process changed or is no longer eligible for Stop. Refresh and inspect it again.")
		}
		if err := app.Signal(pid, syscall.SIGTERM); err != nil {
			return ActionResult{}, err
		}

		return ActionResult{
			OK:      true,
			Message: fmt.Sprintf("Sent a graceful stop request to PID %d. Refresh to verify it exited.", pid),
		}, nil
	}

	return ActionResult{}, errors.New("This process is no longer listening. Refresh the list.")
}
