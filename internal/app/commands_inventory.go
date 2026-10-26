package app

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"workbench/internal/inventory"
)

func (options *commandOptions) newStatusCommand() *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "status",
		Short: "Inspect project ownership and actual TCP listeners",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			snapshot, err := inventory.Capture(store, inventory.NewInspector())
			if err != nil {
				return err
			}

			output := command.OutOrStdout()
			if jsonOutput {
				return printJSON(output, snapshot)
			}

			for _, project := range snapshot.Projects {
				if _, err := fmt.Fprintf(output, "%-24s %-10s %s\n", project.ID, project.Status, project.Name); err != nil {
					return err
				}
			}

			if snapshot.InspectionError != nil {
				return errors.New(*snapshot.InspectionError)
			}

			return nil
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")

	return command
}

func (options *commandOptions) newCheckPortCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check-port PORT",
		Short: "Check catalog reservations and live listeners for a port",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			port, err := strconv.Atoi(arguments[0])
			if err != nil || port < 1 || port > 65535 {
				return errors.New("Port must be between 1 and 65535.")
			}

			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			snapshot, err := inventory.Capture(store, inventory.NewInspector())
			if err != nil {
				return err
			}

			reservedBy := []string{}
			listeners := []*inventory.Service{}

			for _, project := range snapshot.Projects {
				if slices.Contains(project.Ports, port) {
					reservedBy = append(reservedBy, project.Name)
				}
			}

			for _, service := range snapshot.Services {
				if slices.Contains(service.Ports, port) {
					listeners = append(listeners, service)
				}
			}

			if err := printJSON(command.OutOrStdout(), map[string]any{
				"port":            port,
				"reservedBy":      reservedBy,
				"listeners":       listeners,
				"inspectionError": snapshot.InspectionError,
			}); err != nil {
				return err
			}

			if len(reservedBy) > 0 || len(listeners) > 0 || snapshot.InspectionError != nil {
				options.exitCode = exitPortUnavailable
			}

			return nil
		},
	}
}
