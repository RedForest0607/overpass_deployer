package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareCreatesPrivateStorageTree(t *testing.T) {
	root := filepath.Join(t.TempDir(), "studio")
	paths, err := Prepare(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Root, paths.Artifacts, paths.Packages, paths.Temporary} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %o, want 700", path, info.Mode().Perm())
		}
	}
}
