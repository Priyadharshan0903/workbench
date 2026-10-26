package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"workbench/internal/fsutil"
)

func (library *Library) Save(request SaveRequest) (SaveResult, error) {
	if request.Content == nil {
		return SaveResult{}, fmt.Errorf("%w Instruction content is required.", ErrInvalid)
	}
	if _, err := Render(*request.Content); err != nil {
		return SaveResult{}, err
	}
	skill, err := library.resolve(request.ID)
	if err != nil {
		return SaveResult{}, err
	}
	destinations, err := library.destinations()
	if err != nil {
		return SaveResult{}, err
	}
	if managedSkill(skill) || !library.editable(skill.Path, destinations) {
		return SaveResult{}, ErrReadOnly
	}

	lockFolder := filepath.Join(library.Store.Root, ".runtime/skill-locks")
	if err := os.MkdirAll(lockFolder, 0700); err != nil {
		return SaveResult{}, err
	}
	lock, err := os.OpenFile(filepath.Join(lockFolder, request.ID+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return SaveResult{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return SaveResult{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	folder := filepath.Dir(skill.Path)
	root, err := os.OpenRoot(folder)
	if err != nil {
		return SaveResult{}, err
	}
	defer root.Close()
	previous, info, err := readRegular(root, "SKILL.md", MaxContentBytes)
	if err != nil {
		return SaveResult{}, err
	}
	if request.Revision == "" || digest(previous) != request.Revision {
		return SaveResult{}, ErrConflict
	}

	backupFolder := filepath.Join(library.Store.Root, ".runtime/skill-backups", request.ID)
	if err := os.MkdirAll(backupFolder, 0700); err != nil {
		return SaveResult{}, err
	}
	backup, err := os.CreateTemp(backupFolder, time.Now().UTC().Format("20060102T150405Z")+"-*.md")
	if err != nil {
		return SaveResult{}, err
	}
	if _, err := backup.Write(previous); err != nil {
		backup.Close()
		return SaveResult{}, err
	}
	if err := backup.Sync(); err != nil {
		backup.Close()
		return SaveResult{}, err
	}
	if err := backup.Close(); err != nil {
		return SaveResult{}, err
	}

	temporary, err := os.CreateTemp(folder, ".workbench-skill-*")
	if err != nil {
		return SaveResult{}, err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.WriteString(*request.Content); err != nil {
		temporary.Close()
		return SaveResult{}, err
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return SaveResult{}, err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return SaveResult{}, err
	}
	if err := temporary.Close(); err != nil {
		return SaveResult{}, err
	}

	// Recheck the target and its revision immediately before atomic replacement.
	current, currentInfo, err := readRegular(root, "SKILL.md", MaxContentBytes)
	pathInfo, pathErr := os.Lstat(skill.Path)
	if err != nil || !os.SameFile(info, currentInfo) || digest(current) != request.Revision ||
		pathErr != nil || !os.SameFile(info, pathInfo) ||
		fsutil.Canonical(skill.Path) != skill.Path || !library.editable(skill.Path, destinations) {
		return SaveResult{}, ErrConflict
	}
	if err := os.Rename(temporary.Name(), skill.Path); err != nil {
		return SaveResult{}, err
	}

	return SaveResult{OK: true, Revision: digest([]byte(*request.Content)), BackupPath: backup.Name()}, nil
}
