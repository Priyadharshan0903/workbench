package app

import (
	"errors"

	"github.com/spf13/cobra"
)

func (options *commandOptions) newEngineCommand(name string) *cobra.Command {
	var port int
	var skipBrowser bool

	command := &cobra.Command{
		Use:   name,
		Short: "Start or reuse Workbench in the background",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			if port == 0 {
				port, err = configuredPort(store.Root)
				if err != nil {
					return err
				}
			}

			if port < 1024 || port > 65535 {
				return errors.New("Use a port from 1024 to 65535.")
			}

			if err := store.Initialize(port); err != nil {
				return err
			}

			if name == "serve" {
				return serve(store, port, command.OutOrStdout())
			}

			return startWorkbench(store, port, skipBrowser, command.OutOrStdout())
		},
	}

	if name == "serve" {
		command.Short = "Run Workbench in the foreground"
	}

	command.Flags().IntVar(&port, "port", 0, "Override configured local port")
	command.Flags().BoolVar(&skipBrowser, "no-browser", false, "Do not open a browser")

	return command
}
