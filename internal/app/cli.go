// Package app connects CLI commands to catalog, HTTP and engine lifecycle operations.
package app

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"workbench/internal/catalog"
	"workbench/internal/fsutil"
	"workbench/internal/launch"
)

const (
	exitSuccess         = 0
	exitFailure         = 1
	exitPortUnavailable = 2
)

// commandOptions belongs to one invocation, so commands never share flag state.
type commandOptions struct {
	rootDirectory string
	exitCode      int
}

// Run executes the CLI without exiting the process. Errors are printed by main;
// keeping Cobra silent prevents duplicate diagnostics and protects MCP stdout.
func Run(arguments []string, input io.Reader, output, stderr io.Writer) (int, error) {
	options := &commandOptions{rootDirectory: defaultRoot(), exitCode: exitSuccess}
	rootCommand := options.newRootCommand()

	if input == nil {
		input = strings.NewReader("")
	}

	rootCommand.SetArgs(arguments)
	rootCommand.SetIn(input)
	rootCommand.SetOut(output)
	rootCommand.SetErr(stderr)

	if err := rootCommand.Execute(); err != nil {
		return exitFailure, err
	}

	return options.exitCode, nil
}

func (options *commandOptions) newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "workbench",
		Short:         "Manage your local project workspace",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	command.PersistentFlags().StringVar(
		&options.rootDirectory, "root", options.rootDirectory, "Installation/catalog folder",
	)

	command.AddCommand(
		options.newEngineCommand("start"),
		options.newEngineCommand("serve"),
		options.newListCommand(),
		options.newStatusCommand(),
		options.newRegisterCommand(),
		options.newLifecycleCommand(),
		options.newContextCommand(),
		options.newSkillsCommand(),
		options.newCheckPortCommand(),
		options.newPackageCommand(),
		options.newMCPCommand(),
		newRunnerCommand(),
	)

	return command
}

func (options *commandOptions) catalogStore() (catalog.Store, error) {
	absoluteRoot, err := filepath.Abs(options.rootDirectory)
	if err != nil {
		return catalog.Store{}, err
	}

	return catalog.Store{Root: fsutil.Canonical(absoluteRoot)}, nil
}

func newRunnerCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "runner",
		Short:  "Run a supervised project from its internal stdin payload",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return launch.RunDetached(command.InOrStdin())
		},
	}
}

func printJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	return encoder.Encode(value)
}
