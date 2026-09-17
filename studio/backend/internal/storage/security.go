package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func Inspect(root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve data directory: %w", err)
	}
	paths := Paths{
		Root:      absolute,
		Database:  filepath.Join(absolute, "studio.db"),
		Artifacts: filepath.Join(absolute, "artifacts"),
		Packages:  filepath.Join(absolute, "packages"),
		Temporary: filepath.Join(absolute, "tmp"),
	}
	for _, directory := range []string{paths.Root, paths.Artifacts, paths.Packages, paths.Temporary} {
		if err := inspectPrivatePath(directory, true); err != nil {
			return err
		}
	}
	if err := inspectPrivatePath(paths.Database, false); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := inspectOptionalPrivateFile(paths.Database + suffix); err != nil {
			return err
		}
	}
	for _, root := range []string{paths.Artifacts, paths.Packages, paths.Temporary} {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == root {
				return nil
			}
			return inspectPrivatePath(path, entry.IsDir())
		}); err != nil {
			return fmt.Errorf("inspect storage tree %q: %w", root, err)
		}
	}
	return nil
}

func inspectOptionalPrivateFile(path string) error {
	err := inspectPrivatePath(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func inspectPrivatePath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q must not be a symbolic link", path)
	}
	if directory && !info.IsDir() {
		return fmt.Errorf("%q must be a directory", path)
	}
	if !directory && !info.Mode().IsRegular() {
		return fmt.Errorf("%q must be a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%q permissions %04o expose data to group or others", path, info.Mode().Perm())
	}
	return nil
}
