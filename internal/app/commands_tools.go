package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"workbench/internal/discovery"
	"workbench/internal/distribution"
	"workbench/internal/mcp"
)

func (options *commandOptions) newSkillsCommand() *cobra.Command {
	var provider string
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "skills",
		Short: "Discover installed Codex and Claude skill metadata",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if provider != "" && provider != "codex" && provider != "claude" {
				return errors.New("Provider must be codex or claude.")
			}

			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			registry, err := store.Read()
			if err != nil {
				return err
			}

			homeDirectory, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			discovered := discovery.DiscoverSkills(registry.Projects, homeDirectory)
			if provider != "" {
				filteredSkills := []*discovery.Skill{}
				for _, skill := range discovered.Skills {
					if slices.Contains(skill.Providers, provider) {
						filteredSkills = append(filteredSkills, skill)
					}
				}
				discovered.Skills = filteredSkills
			}

			output := command.OutOrStdout()
			if jsonOutput {
				return printJSON(output, discovered)
			}

			if _, err := fmt.Fprintln(output, discovered.Note); err != nil {
				return err
			}

			for _, skill := range discovered.Skills {
				if _, err := fmt.Fprintf(output, "%s [%s]\n  %s\n", skill.Name, strings.Join(skill.Providers, ", "), skill.Path); err != nil {
					return err
				}
			}

			return nil
		},
	}

	command.Flags().StringVar(&provider, "provider", "", "Filter to codex or claude")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")

	return command
}

func (options *commandOptions) newPackageCommand() *cobra.Command {
	var runtimeOnly bool
	var outputFile string

	command := &cobra.Command{
		Use:   "package",
		Short: "Export a clean source or executable ZIP",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			if outputFile == "" {
				archiveName := "Workbench-codebase.zip"
				if runtimeOnly {
					archiveName = "Workbench.zip"
				}
				outputFile = filepath.Join(store.Root, "dist", archiveName)
			}

			if err := distribution.Create(store.Root, outputFile, runtimeOnly); err != nil {
				return err
			}

			_, err = fmt.Fprintln(command.OutOrStdout(), outputFile)
			return err
		},
	}

	command.Flags().BoolVar(&runtimeOnly, "runtime-only", false, "Package the current platform executable")
	command.Flags().StringVar(&outputFile, "output", "", "Destination ZIP")

	return command
}

func (options *commandOptions) newMCPCommand() *cobra.Command {
	var allowWrites bool

	command := &cobra.Command{
		Use:   "mcp",
		Short: "Serve local MCP tools over stdio (read-only by default)",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			store, err := options.catalogStore()
			if err != nil {
				return err
			}

			if _, err := store.Read(); err != nil {
				return err
			}

			server := mcp.New(store, allowWrites)
			return server.Serve(command.InOrStdin(), command.OutOrStdout())
		},
	}

	command.Flags().BoolVar(&allowWrites, "allow-writes", false, "Expose revision-checked project registration (no process execution)")

	return command
}
