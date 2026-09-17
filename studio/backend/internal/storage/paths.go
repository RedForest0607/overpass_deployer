package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	Root      string
	Database  string
	Artifacts string
	Packages  string
	Temporary string
}

func Prepare(root string) (Paths, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve data directory: %w", err)
	}
	paths := Paths{
		Root:      absolute,
		Database:  filepath.Join(absolute, "studio.db"),
		Artifacts: filepath.Join(absolute, "artifacts"),
		Packages:  filepath.Join(absolute, "packages"),
		Temporary: filepath.Join(absolute, "tmp"),
	}
	for _, directory := range []string{paths.Root, paths.Artifacts, paths.Packages, paths.Temporary} {
		if err := preparePrivateDirectory(directory); err != nil {
			return Paths{}, err
		}
	}
	if info, err := os.Lstat(paths.Database); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return Paths{}, fmt.Errorf("database path %q must be a regular file, not a symbolic link", paths.Database)
		}
	} else if !os.IsNotExist(err) {
		return Paths{}, fmt.Errorf("inspect database path %q: %w", paths.Database, err)
	}
	return paths, nil
}

func preparePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create data directory %q: %w", path, err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return fmt.Errorf("inspect data directory %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("data directory %q must be a directory, not a symbolic link", path)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure data directory %q: %w", path, err)
	}
	return nil
}
