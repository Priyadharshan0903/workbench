package launch

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"workbench/internal/catalog"
)

type LogOutput struct {
	Text    string         `json:"text"`
	Message string         `json:"message"`
	Run     map[string]any `json:"run,omitempty"`
}

var logFilename = regexp.MustCompile(`^[a-z0-9-]+-[a-f0-9]{16}\.log$`)

func (s *Supervisor) Logs(id string) (LogOutput, error) {
	output := LogOutput{Message: "No Workbench launch logs yet. Start the project here to capture its output."}
	r, e := s.Store.Read()
	if e != nil {
		return output, e
	}
	if _, e = catalog.Find(r, id); e != nil {
		return output, e
	}
	record := s.ReadRun(id)
	if record == nil {
		return output, nil
	}
	if filepath.Base(record.LogFile) != record.LogFile || !logFilename.MatchString(record.LogFile) {
		return output, errors.New("The launch log reference is invalid.")
	}
	f, e := os.Open(filepath.Join(s.Store.Root, ".runtime/logs", record.LogFile))
	if e != nil && !os.IsNotExist(e) {
		return output, e
	}
	text := ""
	if e == nil {
		defer f.Close()
		info, e := f.Stat()
		if e != nil {
			return output, e
		}
		if !info.Mode().IsRegular() {
			return output, errors.New("The launch log is not a regular file.")
		}
		offset := info.Size() - 24000
		if offset < 0 {
			offset = 0
		}
		if _, e = f.Seek(offset, io.SeekStart); e != nil {
			return output, e
		}
		b, e := io.ReadAll(io.LimitReader(f, 24000))
		if e != nil {
			return output, e
		}
		text = string(b)
	}
	lines := strings.Split(strings.TrimRight(text, "\r\n"), "\n")
	if len(lines) > 160 {
		lines = lines[len(lines)-160:]
	}
	output.Text = redact(strings.Join(lines, "\n"))
	output.Message = "Most recent output · last 160 lines · common credentials masked"
	output.Run = publicRun(record)
	return output, nil
}

var ansiPattern = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
var bearerPattern = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`)
var credentialPattern = regexp.MustCompile(`(?i)((?:api[_-]?key|token|password|secret)\s*[=:]\s*)\S+`)
var keyPattern = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9_]{12,}|sk-[A-Za-z0-9_-]{20,})\b`)

func redact(text string) string {
	text = ansiPattern.ReplaceAllString(text, "")
	text = bearerPattern.ReplaceAllString(text, "${1}[redacted]")
	text = credentialPattern.ReplaceAllString(text, "${1}[redacted]")
	return keyPattern.ReplaceAllString(text, "[redacted]")
}
func diagnose(text, code string) string {
	lower := strings.ToLower(text)
	for _, v := range []struct {
		keys    []string
		message string
	}{{[]string{"eaddrinuse", "address already in use"}, "The output reports a busy port. Inspect Diagnostics, or update the project's URL and port before retrying."}, {[]string{"modulenotfounderror", "cannot find module", "module not found", "command not found", "not found: vite"}, "The output reports a missing project dependency. Follow the project's setup instructions, then start it again."}, {[]string{"ebadengine", "unsupported engine", "requires node.js"}, "The output reports an unsupported runtime version. Check the version required by the project."}, {[]string{"no such file or directory", "enoent"}, "The output reports a missing file. Check the project folder and launch command in Settings."}, {[]string{"permission denied", "eacces"}, "The output reports a permission problem. Inspect the log for the affected file or port."}} {
		for _, key := range v.keys {
			if strings.Contains(lower, key) {
				return v.message
			}
		}
	}
	return "Launch exited with code " + code + ". Open logs for the failing step."
}
