package launch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"workbench/internal/fsutil"
)

func runtimeEnvironment() []string {
	paths := filepath.SplitList(os.Getenv("PATH"))
	paths = append(paths, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin")
	home, _ := os.UserHomeDir()
	versions, _ := os.ReadDir(filepath.Join(home, ".nvm/versions/node"))
	sort.Slice(versions, func(i, j int) bool { return versions[i].Name() > versions[j].Name() })
	for _, v := range versions {
		if v.IsDir() {
			paths = append(paths, filepath.Join(home, ".nvm/versions/node", v.Name(), "bin"))
		}
	}
	unique := []string{}
	for _, p := range paths {
		if p != "" && !slices.Contains(unique, p) {
			unique = append(unique, p)
		}
	}
	env := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "PATH=") && !strings.HasPrefix(v, "PYTHONUNBUFFERED=") && !strings.HasPrefix(v, "NO_COLOR=") {
			env = append(env, v)
		}
	}
	return append(env, "PATH="+strings.Join(unique, string(os.PathListSeparator)), "PYTHONUNBUFFERED=1", "NO_COLOR=1")
}
func executable(name, cwd string, env []string) (string, error) {
	check := func(path string) bool {
		i, e := os.Stat(path)
		return e == nil && i.Mode().IsRegular() && i.Mode().Perm()&0111 != 0
	}
	if strings.Contains(name, "/") {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		if check(path) {
			return filepath.Abs(path)
		}
	} else {
		for _, v := range env {
			if strings.HasPrefix(v, "PATH=") {
				for _, dir := range filepath.SplitList(strings.TrimPrefix(v, "PATH=")) {
					if !filepath.IsAbs(dir) {
						continue
					}
					p := filepath.Join(dir, name)
					if check(p) {
						return p, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("%s is not installed or is not on PATH. Install it, then retry.", name)
}

// A small POSIX argv lexer, not a shell. Only unquoted && separates steps.
func parseCommands(command, cwd string, env []string) ([][]string, error) {
	if strings.TrimSpace(command) == "" {
		return nil, errors.New("Add a launch command in project settings first.")
	}
	for _, v := range []string{"\n", "\r", "\x00", "`", "$(", "${"} {
		if strings.Contains(command, v) {
			return nil, errors.New("Use a plain launch command. Shell substitutions are not supported.")
		}
	}
	commands := [][]string{}
	argv := []string{}
	var token strings.Builder
	quote := byte(0)
	started := false
	flush := func() {
		if started {
			s := token.String()
			if s == "~" || strings.HasPrefix(s, "~/") {
				home, _ := os.UserHomeDir()
				s = home + strings.TrimPrefix(s, "~")
			}
			argv = append(argv, s)
			token.Reset()
			started = false
		}
	}
	for n := 0; n < len(command); n++ {
		c := command[n]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				token.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if n+1 >= len(command) {
				return nil, errors.New("The launch command ends with an escape.")
			}
			next := command[n+1]
			if quote == '"' && !strings.ContainsRune("$`\"\\", rune(next)) {
				token.WriteByte(c)
			} else {
				n++
				token.WriteByte(next)
			}
			started = true
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			} else {
				token.WriteByte(c)
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			started = true
			continue
		}
		if c == ' ' || c == '\t' {
			flush()
			continue
		}
		if strings.ContainsRune(";&|<>", rune(c)) {
			flush()
			end := n + 1
			for end < len(command) && strings.ContainsRune(";&|<>", rune(command[end])) {
				end++
			}
			op := command[n:end]
			if op != "&&" {
				return nil, errors.New("Pipes, backgrounding, and redirection are not supported. Use a project script instead.")
			}
			if len(argv) == 0 {
				return nil, errors.New("Each launch step needs a command.")
			}
			commands = append(commands, argv)
			argv = []string{}
			n = end - 1
			continue
		}
		token.WriteByte(c)
		started = true
	}
	if quote != 0 {
		return nil, errors.New("The launch command has an unclosed quote.")
	}
	flush()
	if len(argv) == 0 {
		return nil, errors.New("The launch command ends with an empty step.")
	}
	commands = append(commands, argv)
	if len(commands) > 6 {
		return nil, errors.New("Use at most six launch steps, or put them in a project script.")
	}
	if env == nil {
		env = runtimeEnvironment()
	}
	for _, args := range commands {
		if strings.Contains(args[0], "=") {
			return nil, errors.New("Put environment assignments in a project script.")
		}
		path, e := executable(args[0], cwd, env)
		if e != nil {
			return nil, e
		}
		args[0] = path
	}
	for n, args := range commands {
		if len(args) == 2 && args[1] == "start.py" {
			if b := fsutil.ReadText(filepath.Join(cwd, "start.py"), 65536); strings.Contains(b, "--no-browser") {
				commands[n] = append(args, "--no-browser")
			}
		}
	}
	return commands, nil
}
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\;&|<>$`()[]{}*?!#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
func displayCommand(commands [][]string) string {
	steps := []string{}
	for _, argv := range commands {
		words := []string{}
		for _, a := range argv {
			words = append(words, shellQuote(a))
		}
		steps = append(steps, strings.Join(words, " "))
	}
	return strings.Join(steps, " && ")
}
