// Package fsutil provides bounded file reads, resolved path checks and atomic replacement.
package fsutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SecretPath(path string) bool {
	parts := strings.Split(filepath.Clean(path), string(os.PathSeparator))
	for index, part := range parts {
		part = strings.ToLower(part)
		if part == ".env" || strings.HasPrefix(part, ".env.") || part == ".ssh" || part == ".aws" ||
			part == ".gnupg" || part == ".azure" ||
			(part == "gcloud" && index > 0 && strings.EqualFold(parts[index-1], ".config")) {
			return true
		}
	}
	return false
}
func Canonical(path string) string {
	p, e := filepath.EvalSymlinks(path)
	if e == nil {
		return p
	}
	p, _ = filepath.Abs(path)
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(Canonical(parent), filepath.Base(p))
}
func Within(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	rel, e := filepath.Rel(Canonical(root), Canonical(path))
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func IsDir(path string) bool  { i, e := os.Stat(path); return e == nil && i.IsDir() }
func Exists(path string) bool { _, e := os.Stat(path); return e == nil }
func ReadBounded(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	i, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > limit {
		return nil, fmt.Errorf("File is not a bounded regular file: %s", filepath.Base(path))
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("File grew beyond its read limit.")
	}
	return b, e
}
func AtomicWrite(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func WriteJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return AtomicWrite(path, append(b, '\n'))
}

func ReadText(path string, limit int64) string {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		return ""
	}
	b, e := ReadBounded(path, limit)
	if e != nil {
		return ""
	}
	return string(b)
}
