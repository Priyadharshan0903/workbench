package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"workbench/internal/catalog"
	"workbench/internal/discovery"
	"workbench/internal/distribution"
	"workbench/internal/fsutil"
	"workbench/internal/inventory"
	"workbench/internal/launch"
)

func printJSON(out io.Writer, v any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
func Run(args []string, input io.Reader, out, stderr io.Writer) (int, error) {
	if len(args) > 0 && args[0] == "runner" {
		return 0, launch.RunDetached(input)
	}
	global := flag.NewFlagSet("workbench", flag.ContinueOnError)
	global.SetOutput(stderr)
	root := global.String("root", defaultRoot(), "Installation/catalog folder (before subcommand)")
	global.Usage = func() {
		fmt.Fprintln(stderr, "Usage: workbench [--root FOLDER] <start|serve|list|status|register|lifecycle|context|skills|check-port|package> [options]")
	}
	if e := global.Parse(args); e != nil {
		if e == flag.ErrHelp {
			return 0, nil
		}
		return 1, e
	}
	args = global.Args()
	if len(args) == 0 {
		global.Usage()
		return 0, nil
	}
	absolute, e := filepath.Abs(*root)
	if e != nil {
		return 1, e
	}
	store := catalog.Store{Root: fsutil.Canonical(absolute)}
	command := args[0]
	args = args[1:]
	if command == "start" || command == "serve" {
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(stderr)
		port := flags.Int("port", 0, "Override configured local port")
		noBrowser := flags.Bool("no-browser", false, "Do not open a browser")
		if e = flags.Parse(args); e != nil {
			if e == flag.ErrHelp {
				return 0, nil
			}
			return 1, e
		}
		if flags.NArg() > 0 {
			return 1, errors.New("Unexpected launcher arguments.")
		}
		if *port == 0 {
			*port, e = configuredPort(store.Root)
			if e != nil {
				return 1, e
			}
		}
		if *port < 1024 || *port > 65535 {
			return 1, errors.New("Use a port from 1024 to 65535.")
		}
		if e = store.Initialize(*port); e != nil {
			return 1, e
		}
		if command == "serve" {
			return 0, serve(store, *port, out)
		}
		return 0, startWorkbench(store, *port, *noBrowser, out)
	}
	if command == "package" {
		return 0, packageCLI(store, args, out, stderr)
	}
	r, e := store.Read()
	if e != nil {
		return 1, e
	}
	switch command {
	case "list", "status":
		if len(args) > 1 || (len(args) == 1 && args[0] != "--json") {
			return 1, errors.New("Use list/status [--json].")
		}
		jsonOutput := slices.Contains(args, "--json")
		if command == "list" {
			if jsonOutput {
				return 0, printJSON(out, r)
			}
			for _, p := range r.Projects {
				fmt.Fprintf(out, "%-24s %-10s %s\n", p.ID, p.Lifecycle, p.Name)
			}
			return 0, nil
		}
		snapshot, e := inventory.Capture(store, inventory.NewInspector())
		if e != nil {
			return 1, e
		}
		if jsonOutput {
			return 0, printJSON(out, snapshot)
		}
		for _, p := range snapshot.Projects {
			fmt.Fprintf(out, "%-24s %-10s %s\n", p.ID, p.Status, p.Name)
		}
		if snapshot.InspectionError != nil {
			return 1, errors.New(*snapshot.InspectionError)
		}
		return 0, nil
	case "register":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(stderr)
		file := flags.String("file", "", "Project JSON file, or - for stdin")
		if e = flags.Parse(args); e != nil {
			return 1, e
		}
		if *file == "" || flags.NArg() != 0 {
			return 1, errors.New("Use register --file FILE (or - for stdin).")
		}
		var b []byte
		if *file == "-" {
			b, e = io.ReadAll(io.LimitReader(input, 32769))
		} else {
			if fsutil.SecretPath(*file) {
				return 1, errors.New("Secret files cannot be read as project records.")
			}
			b, e = fsutil.ReadBounded(*file, 32768)
		}
		if e != nil {
			return 1, e
		}
		if len(b) > 32768 {
			return 1, errors.New("Project records are limited to 32 KB.")
		}
		var p catalog.Project
		if e = json.Unmarshal(b, &p); e != nil {
			return 1, e
		}
		result, e := store.Save(p, 0)
		if e != nil {
			return 1, e
		}
		return 0, printJSON(out, map[string]any{"ok": true, "id": p.ID, "revision": result.Revision})
	case "context", "lifecycle":
		expected := 1
		if command == "lifecycle" {
			expected = 2
		}
		if len(args) != expected {
			return 1, fmt.Errorf("Use %s ID%s.", command, map[bool]string{true: " active|paused|archived", false: ""}[command == "lifecycle"])
		}
		p, e := catalog.Find(r, args[0])
		if e != nil {
			return 1, e
		}
		if command == "lifecycle" {
			p.Lifecycle = args[1]
			if _, e = store.Save(p, r.Revision); e != nil {
				return 1, e
			}
			fmt.Fprintf(out, "%s: %s. This changes the catalog only; it does not stop services.\n", p.ID, p.Lifecycle)
			return 0, nil
		}
		fmt.Fprintf(out, "# %s (%s)\n%s\n\npath: %s\nurl: %s\nports: %v\nlifecycle: %s\nowner: %s\nlaunch: %s\nmaintenance: %s\ndocs: %s\n\nRead the project AGENTS.md / CLAUDE.md before changes. Check live ports before starting a service.\n", p.Name, p.ID, p.Description, p.Path, p.URL, p.Ports, p.Lifecycle, p.Owner, p.Launch, p.Maintenance, p.Docs)
		return 0, nil
	case "skills":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(stderr)
		provider := flags.String("provider", "", "codex or claude")
		jsonOutput := flags.Bool("json", false, "Print JSON")
		if e = flags.Parse(args); e != nil {
			return 1, e
		}
		if flags.NArg() != 0 || (*provider != "" && !slices.Contains([]string{"codex", "claude"}, *provider)) {
			return 1, errors.New("Use skills [--provider codex|claude] [--json].")
		}
		home, _ := os.UserHomeDir()
		skills := discovery.DiscoverSkills(r.Projects, home)
		if *provider != "" {
			filtered := []*discovery.Skill{}
			for _, s := range skills.Skills {
				if slices.Contains(s.Providers, *provider) {
					filtered = append(filtered, s)
				}
			}
			skills.Skills = filtered
		}
		if *jsonOutput {
			return 0, printJSON(out, skills)
		}
		fmt.Fprintln(out, skills.Note)
		for _, s := range skills.Skills {
			fmt.Fprintf(out, "%s [%s]\n  %s\n", s.Name, strings.Join(s.Providers, ", "), s.Path)
		}
		return 0, nil
	case "check-port":
		if len(args) != 1 {
			return 1, errors.New("Use check-port PORT.")
		}
		port, e := strconv.Atoi(args[0])
		if e != nil || port < 1 || port > 65535 {
			return 1, errors.New("Port must be between 1 and 65535.")
		}
		state, e := inventory.Capture(store, inventory.NewInspector())
		if e != nil {
			return 1, e
		}
		reserved := []string{}
		listeners := []*inventory.Service{}
		for _, p := range r.Projects {
			if slices.Contains(p.Ports, port) {
				reserved = append(reserved, p.Name)
			}
		}
		for _, s := range state.Services {
			if slices.Contains(s.Ports, port) {
				listeners = append(listeners, s)
			}
		}
		e = printJSON(out, map[string]any{"port": port, "reservedBy": reserved, "listeners": listeners, "inspectionError": state.InspectionError})
		if len(reserved) > 0 || len(listeners) > 0 || state.InspectionError != nil {
			return 2, e
		}
		return 0, e
	default:
		return 1, fmt.Errorf("Unknown command: %s", command)
	}
}

func packageCLI(store catalog.Store, args []string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtimeOnly := flags.Bool("runtime-only", false, "Package the current platform executable")
	output := flags.String("output", "", "Destination ZIP")
	if e := flags.Parse(args); e != nil {
		if e == flag.ErrHelp {
			return nil
		}
		return e
	}
	if flags.NArg() != 0 {
		return errors.New("Use package [--runtime-only] [--output FILE].")
	}
	if *output == "" {
		name := "Workbench-codebase.zip"
		if *runtimeOnly {
			name = "Workbench.zip"
		}
		*output = filepath.Join(store.Root, "dist", name)
	}
	if e := distribution.Create(store.Root, *output, *runtimeOnly); e != nil {
		return e
	}
	fmt.Fprintln(out, *output)
	return nil
}
