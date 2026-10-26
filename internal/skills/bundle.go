package skills

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"workbench/internal/fsutil"
)

const (
	maxBundleBytes = 16 << 20
	maxFileBytes   = 4 << 20
	maxBundleFiles = 400
	maxBundleDepth = 10
)

type bundle struct {
	files    []File
	contents map[string][]byte
	modes    map[string]os.FileMode
	warnings []string
	revision string
	complete bool
}

func readBundle(folder string) (bundle, error) {
	result := bundle{
		files:    []File{},
		contents: map[string][]byte{},
		modes:    map[string]os.FileMode{},
		warnings: []string{},
		complete: true,
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return result, err
	}
	defer root.Close()

	totalBytes := 0
	visitedEntries := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		visitedEntries++
		if visitedEntries > 4000 {
			result.complete = false
			return fs.SkipAll
		}

		if excluded(name) || entry.Type()&os.ModeSymlink != 0 {
			result.warnings = append(result.warnings, "Excluded from copies: "+name)
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if strings.Count(name, "/") >= maxBundleDepth {
				result.complete = false
				return fs.SkipDir
			}
			return nil
		}

		if len(result.files) >= maxBundleFiles {
			result.complete = false
			return nil
		}
		contents, info, err := readRegular(root, name, maxFileBytes)
		if err != nil {
			result.complete = false
			result.warnings = append(result.warnings, "Unavailable file: "+name)
			return nil
		}
		if totalBytes+len(contents) > maxBundleBytes {
			result.complete = false
			return nil
		}

		totalBytes += len(contents)
		result.files = append(result.files, File{Path: name, Size: info.Size(), Readable: readable(contents)})
		result.contents[name] = contents
		result.modes[name] = info.Mode().Perm()
		return nil
	})
	if err != nil {
		return result, err
	}
	if !readable(result.contents["SKILL.md"]) || result.contents["SKILL.md"] == nil {
		return result, fmt.Errorf("%w SKILL.md must be readable UTF-8 text up to 500 KB.", ErrInvalid)
	}
	if !result.complete {
		result.warnings = append(result.warnings, "This bundle exceeds copy limits or has unreadable files. Viewing remains available; bundle copying is disabled.")
	}

	hash := sha256.New()
	for _, file := range result.files {
		fmt.Fprintf(hash, "%s\x00%d\x00", file.Path, result.modes[file.Path])
		hash.Write(result.contents[file.Path])
		hash.Write([]byte{0})
	}
	result.revision = hex.EncodeToString(hash.Sum(nil))

	return result, nil
}

func excluded(name string) bool {
	if !fs.ValidPath(name) || fsutil.SecretPath(name) {
		return true
	}
	for _, component := range strings.Split(name, "/") {
		lower := strings.ToLower(component)
		if strings.HasPrefix(component, ".") || lower == "node_modules" || lower == "vendor" ||
			lower == "credentials.json" || lower == "secrets.json" || strings.HasPrefix(lower, "secrets.") {
			return true
		}
	}

	return false
}

func (library *Library) currentBundle(id, revision string) (*os.Root, bundle, error) {
	skill, err := library.resolve(id)
	if err != nil {
		return nil, bundle{}, err
	}
	folder := filepath.Dir(skill.Path)
	contents, err := readBundle(folder)
	if err != nil {
		return nil, bundle{}, err
	}
	if revision == "" || contents.revision != revision {
		return nil, bundle{}, ErrConflict
	}
	if !contents.complete {
		return nil, bundle{}, fmt.Errorf("%w This bundle is not available for copying.", ErrInvalid)
	}
	root, err := os.OpenRoot(folder)
	return root, contents, err
}

func (library *Library) Export(id, revision string) ([]byte, error) {
	root, contents, err := library.currentBundle(id, revision)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, file := range contents.files {
		header := &zip.FileHeader{Name: path.Join("skill", file.Path), Method: zip.Deflate}
		header.SetMode(contents.modes[file.Path])
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write(contents.contents[file.Path]); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func (library *Library) Duplicate(request CopyRequest) (CopyResult, error) {
	if !validName.MatchString(request.Name) {
		return CopyResult{}, fmt.Errorf("%w Use a folder name with lowercase letters, numbers and hyphens (up to 64 characters).", ErrInvalid)
	}
	destinations, err := library.destinations()
	if err != nil {
		return CopyResult{}, err
	}
	var destination *Destination
	for index := range destinations {
		if destinations[index].ID == request.Destination {
			destination = &destinations[index]
		}
	}
	if destination == nil {
		return CopyResult{}, fmt.Errorf("%w Choose a listed personal or project destination.", ErrInvalid)
	}

	source, contents, err := library.currentBundle(request.ID, request.Revision)
	if err != nil {
		return CopyResult{}, err
	}
	defer source.Close()
	if fsutil.Within(filepath.Join(destination.Path, request.Name), source.Name()) {
		return CopyResult{}, fmt.Errorf("%w Choose a destination outside the source skill folder.", ErrInvalid)
	}

	if err := os.MkdirAll(destination.Path, 0755); err != nil {
		return CopyResult{}, err
	}
	target := filepath.Join(fsutil.Canonical(destination.Path), request.Name)
	if managedPath(target) || fsutil.SecretPath(target) {
		return CopyResult{}, ErrReadOnly
	}
	destinationRoot, err := os.OpenRoot(destination.Path)
	if err != nil {
		return CopyResult{}, err
	}
	defer destinationRoot.Close()
	// Mkdir reserves a new folder exclusively. Existing folders are never merged or replaced.
	if err := destinationRoot.Mkdir(request.Name, 0755); err != nil {
		return CopyResult{}, fmt.Errorf("Could not create a new skill folder; choose another name: %w", err)
	}
	root, err := destinationRoot.OpenRoot(request.Name)
	if err != nil {
		return CopyResult{}, err
	}
	defer root.Close()

	files := append([]File{}, contents.files...)
	// Publish the entry point last so discovery cannot expose a half-written skill.
	sort.SliceStable(files, func(first, second int) bool {
		return files[first].Path != "SKILL.md" && files[second].Path == "SKILL.md"
	})
	for _, file := range files {
		parent := path.Dir(file.Path)
		if parent != "." {
			prefix := ""
			for _, component := range strings.Split(parent, "/") {
				prefix = path.Join(prefix, component)
				if err := root.Mkdir(prefix, 0755); err != nil && !os.IsExist(err) {
					return CopyResult{}, fmt.Errorf("Copy incomplete in %s: %w", target, err)
				}
			}
		}
		if err := createFile(root, file.Path, contents.contents[file.Path], contents.modes[file.Path]|0600); err != nil {
			return CopyResult{}, fmt.Errorf("Copy incomplete in %s: %w", target, err)
		}
	}

	return CopyResult{
		OK:   true,
		ID:   digest([]byte(filepath.Join(target, "SKILL.md")))[:24],
		Path: target,
	}, nil
}

func createFile(root *os.Root, name string, contents []byte, mode os.FileMode) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}
