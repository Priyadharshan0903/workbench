package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"workbench/internal/fsutil"
)

func defaultRoot() string {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	if strings.Contains(dir, "go-build") {
		cwd, _ := os.Getwd()
		return cwd
	}
	return dir
}
func configuredPort(root string) (int, error) {
	b, e := fsutil.ReadBounded(filepath.Join(root, ".runtime/local.json"), 4096)
	if os.IsNotExist(e) {
		return 8730, nil
	}
	if e != nil {
		return 0, e
	}
	var config struct {
		Port int `json:"port"`
	}
	if e = json.Unmarshal(b, &config); e != nil {
		return 0, e
	}
	if config.Port < 1024 || config.Port > 65535 {
		return 0, errors.New("Invalid local engine port in .runtime/local.json.")
	}
	return config.Port, nil
}
