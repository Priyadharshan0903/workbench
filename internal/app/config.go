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
	executablePath, _ := os.Executable()
	executableDirectory := filepath.Dir(executablePath)
	if strings.Contains(executableDirectory, "go-build") {
		workingDirectory, _ := os.Getwd()
		return workingDirectory
	}

	return executableDirectory
}

func configuredPort(root string) (int, error) {
	contents, err := fsutil.ReadBounded(filepath.Join(root, ".runtime/local.json"), 4096)
	if os.IsNotExist(err) {
		return 8730, nil
	}
	if err != nil {
		return 0, err
	}

	var config struct {
		Port int `json:"port"`
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		return 0, err
	}

	if config.Port < 1024 || config.Port > 65535 {
		return 0, errors.New("Invalid local engine port in .runtime/local.json.")
	}

	return config.Port, nil
}
