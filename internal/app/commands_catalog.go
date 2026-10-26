package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
)

const maxProjectRecordBytes = 32 << 10

func (options *commandOptions) newListCommand() *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List durable project records, including stopped projects",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			registry, err := store.Read()
			if err != nil {
				return err
			}

			output := command.OutOrStdout()
			if jsonOutput {
				return printJSON(output, registry)
			}

			for _, project := range registry.Projects {
				if _, err := fmt.Fprintf(output, "%-24s %-10s %s\n", project.ID, project.Lifecycle, project.Name); err != nil {
					return err
				}
			}

			return nil
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")

	return command
}

func (options *commandOptions) newRegisterCommand() *cobra.Command {
	var projectFile string
	var expectedRevision int

	command := &cobra.Command{
		Use:   "register --file FILE",
		Short: "Add or replace a project from a JSON record",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if projectFile == "" {
				return errors.New("Use register --file FILE (or - for stdin).")
			}

			if expectedRevision < 0 {
				return errors.New("Registry revision must not be negative.")
			}

			project, err := readProjectRecord(projectFile, command.InOrStdin())
			if err != nil {
				return err
			}

			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			registry, err := store.Save(project, expectedRevision)
			if err != nil {
				return err
			}

			return printJSON(command.OutOrStdout(), catalog.RegistrationResult{
				OK:       true,
				ID:       project.ID,
				Revision: registry.Revision,
			})
		},
	}

	command.Flags().StringVar(&projectFile, "file", "", "Project JSON file, or - for stdin")
	command.Flags().IntVar(&expectedRevision, "revision", 0, "Require this registry revision before saving")

	return command
}

func readProjectRecord(filename string, input io.Reader) (catalog.Project, error) {
	var contents []byte
	var err error

	if filename == "-" {
		contents, err = io.ReadAll(io.LimitReader(input, maxProjectRecordBytes+1))
	} else {
		if fsutil.SecretPath(filename) {
			return catalog.Project{}, errors.New("Secret files cannot be read as project records.")
		}

		contents, err = fsutil.ReadBounded(filename, maxProjectRecordBytes)
	}

	if err != nil {
		return catalog.Project{}, err
	}

	if len(contents) > maxProjectRecordBytes {
		return catalog.Project{}, errors.New("Project records are limited to 32 KB.")
	}

	var project catalog.Project
	if err := json.Unmarshal(contents, &project); err != nil {
		return catalog.Project{}, err
	}

	return project, nil
}

func (options *commandOptions) newLifecycleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lifecycle PROJECT_ID active|paused|archived",
		Short: "Change a project's catalog lifecycle without stopping services",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, arguments []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			registry, err := store.Read()
			if err != nil {
				return err
			}

			project, err := catalog.Find(registry, arguments[0])
			if err != nil {
				return err
			}

			project.Lifecycle = arguments[1]
			if _, err := store.Save(project, registry.Revision); err != nil {
				return err
			}

			_, err = fmt.Fprintf(command.OutOrStdout(), "%s: %s. This changes the catalog only; it does not stop services.\n", project.ID, project.Lifecycle)
			return err
		},
	}
}

func (options *commandOptions) newContextCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "context PROJECT_ID",
		Short: "Print a project's path, command and maintenance handoff",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			registry, err := store.Read()
			if err != nil {
				return err
			}

			project, err := catalog.Find(registry, arguments[0])
			if err != nil {
				return err
			}

			const contextFormat = `# %s (%s)
%s

path: %s
url: %s
ports: %v
lifecycle: %s
owner: %s
launch: %s
maintenance: %s
docs: %s

Read the project AGENTS.md / CLAUDE.md before changes. Check live ports before starting a service.
`
			_, err = fmt.Fprintf(command.OutOrStdout(), contextFormat,
				project.Name, project.ID, project.Description, project.Path, project.URL,
				project.Ports, project.Lifecycle, project.Owner, project.Launch,
				project.Maintenance, project.Docs,
			)
			return err
		},
	}
}
