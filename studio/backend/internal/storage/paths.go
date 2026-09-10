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
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return Paths{}, fmt.Errorf("create data directory %q: %w", directory, err)
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return Paths{}, fmt.Errorf("secure data directory %q: %w", directory, err)
		}
	}
	return paths, nil
}
