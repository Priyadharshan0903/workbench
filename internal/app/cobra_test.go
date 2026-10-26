package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"workbench/internal/catalog"
	"workbench/internal/testutil"
)

func TestCommandHelpDoesNotReadOrChangeCatalog(t *testing.T) {
	rootDirectory := t.TempDir()
	registryPath := filepath.Join(rootDirectory, "registry.json")
	const corruptCatalog = "this is deliberately not JSON"
	testutil.Write(t, registryPath, corruptCatalog)

	commands := []string{
		"start", "serve", "list", "status", "register", "lifecycle",
		"context", "skills", "check-port", "package", "mcp", "completion",
	}
	invocations := [][]string{{}, {"--help"}, {"help"}}
	for _, command := range commands {
		invocations = append(invocations, []string{command, "--help"})
	}

	for _, arguments := range invocations {
		t.Run(strings.Join(arguments, " "), func(t *testing.T) {
			var output, stderr bytes.Buffer
			arguments = append([]string{"--root", rootDirectory}, arguments...)

			exitCode, err := Run(arguments, nil, &output, &stderr)
			if err != nil || exitCode != exitSuccess {
				t.Fatalf("help: exit %d, error %v", exitCode, err)
			}
			if !strings.Contains(output.String(), "Usage:") || stderr.Len() != 0 {
				t.Fatalf("unexpected help streams: stdout=%q, stderr=%q", output.String(), stderr.String())
			}
		})
	}

	contents, err := os.ReadFile(registryPath)
	if err != nil || string(contents) != corruptCatalog {
		t.Fatalf("help changed the catalog: %q, %v", contents, err)
	}

	entries, err := os.ReadDir(rootDirectory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("help created runtime files: %v, %v", entries, err)
	}
}

func TestInvalidCLIArgumentsFailBeforeCatalogWork(t *testing.T) {
	testCases := []struct {
		name      string
		arguments []string
		message   string
	}{
		{"unknown command", []string{"missing-command"}, "unknown command"},
		{"unknown flag", []string{"list", "--invalid"}, "unknown flag"},
		{"extra list argument", []string{"list", "sample"}, "unknown command"},
		{"missing project", []string{"context"}, "accepts 1 arg"},
		{"missing lifecycle", []string{"lifecycle", "sample"}, "accepts 2 arg"},
		{"missing port", []string{"check-port"}, "accepts 1 arg"},
		{"invalid port", []string{"check-port", "not-a-port"}, "Port must be"},
		{"port out of range", []string{"check-port", "65536"}, "Port must be"},
		{"invalid engine port", []string{"start", "--port", "80"}, "Use a port"},
		{"extra serve argument", []string{"serve", "extra"}, "unknown command"},
		{"extra package argument", []string{"package", "extra"}, "unknown command"},
		{"extra MCP argument", []string{"mcp", "extra"}, "unknown command"},
		{"missing registration file", []string{"register"}, "Use register --file"},
		{"negative revision", []string{"register", "--file", "-", "--revision", "-1"}, "revision must not be negative"},
		{"invalid provider", []string{"skills", "--provider", "invalid"}, "Provider must be"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rootDirectory := t.TempDir()
			arguments := append([]string{"--root", rootDirectory}, testCase.arguments...)
			var output, stderr bytes.Buffer

			exitCode, err := Run(arguments, nil, &output, &stderr)
			if exitCode != exitFailure || err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("got exit %d and error %v; want %q", exitCode, err, testCase.message)
			}
			if output.Len() != 0 || stderr.Len() != 0 {
				t.Fatal("Run printed an error that main would print again")
			}

			entries, err := os.ReadDir(rootDirectory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid arguments created catalog/runtime data: %v, %v", entries, err)
			}
		})
	}
}

func TestPersistentRootFlagWorksBeforeAndAfterCommand(t *testing.T) {
	store, _ := testutil.FixtureStore(t)
	invocations := [][]string{
		{"--root", store.Root, "list", "--json"},
		{"list", "--root", store.Root, "--json"},
		{"list", "--json", "--root", store.Root},
	}

	for _, arguments := range invocations {
		var output, stderr bytes.Buffer
		exitCode, err := Run(arguments, nil, &output, &stderr)
		if exitCode != exitSuccess || err != nil {
			t.Fatal(exitCode, err)
		}

		var registry catalog.Registry
		if err := json.Unmarshal(output.Bytes(), &registry); err != nil {
			t.Fatal("list stdout contains non-JSON output", err)
		}
		if len(registry.Projects) != 1 || registry.Projects[0].ID != "sample" || registry.Revision != 1 {
			t.Fatal("wrong catalog selected", output.String())
		}
	}
}

func TestCLIFlagAndExitStateDoNotLeakAcrossInvocations(t *testing.T) {
	store, _ := testutil.FixtureStore(t)
	var output, stderr bytes.Buffer

	exitCode, err := Run([]string{"check-port", "3000", "--root", store.Root}, nil, &output, &stderr)
	if exitCode != exitPortUnavailable || err != nil {
		t.Fatal("reserved port did not retain exit 2", exitCode, err)
	}

	var portState struct {
		ReservedBy []string `json:"reservedBy"`
	}
	if err := json.Unmarshal(output.Bytes(), &portState); err != nil || len(portState.ReservedBy) != 1 {
		t.Fatal("reservation missing from JSON", output.String(), err)
	}

	for _, jsonOutput := range []bool{true, false} {
		output.Reset()
		arguments := []string{"--root", store.Root, "list"}
		if jsonOutput {
			arguments = append(arguments, "--json")
		}

		exitCode, err = Run(arguments, nil, &output, &stderr)
		if exitCode != exitSuccess || err != nil {
			t.Fatal("previous command exit state leaked", exitCode, err)
		}
		if json.Valid(output.Bytes()) != jsonOutput {
			t.Fatalf("JSON flag leaked: want JSON %v, got %q", jsonOutput, output.String())
		}
	}
}

func TestRunnerRemainsHiddenAndReadsOnlyItsPayload(t *testing.T) {
	options := &commandOptions{rootDirectory: t.TempDir()}
	rootCommand := options.newRootCommand()
	var visibleCommands []string
	for _, command := range rootCommand.Commands() {
		if !command.Hidden {
			visibleCommands = append(visibleCommands, command.Name())
		}
	}
	if slices.Contains(visibleCommands, "runner") {
		t.Fatal("internal runner exposed in command list")
	}

	var output, stderr bytes.Buffer
	exitCode, err := Run([]string{"--root", options.rootDirectory, "runner"}, strings.NewReader("{}"), &output, &stderr)
	if exitCode != exitFailure || err == nil || !strings.Contains(err.Error(), "Invalid runner payload") {
		t.Fatal("runner tried to open a catalog or ignored its payload", exitCode, err)
	}
}

type failingWriter struct {
	err error
}

func (writer failingWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func TestCLIReportsOutputFailures(t *testing.T) {
	store, _ := testutil.FixtureStore(t)
	outputError := errors.New("output fixture failed")

	for _, arguments := range [][]string{{"list"}, {"list", "--json"}, {"context", "sample"}, {"check-port", "3000"}} {
		arguments = append([]string{"--root", store.Root}, arguments...)
		exitCode, err := Run(arguments, nil, failingWriter{err: outputError}, io.Discard)
		if exitCode != exitFailure || !errors.Is(err, outputError) {
			t.Fatalf("%v hid output error: exit %d, %v", arguments, exitCode, err)
		}
	}
}
