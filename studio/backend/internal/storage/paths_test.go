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

func TestPrepareRejectsSymbolicLinkStorage(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "actual-artifacts")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "studio")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(data, "artifacts")); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(data); err == nil {
		t.Fatal("expected symbolic link storage to be rejected")
	}
}

func TestInspectRejectsPublicPermissions(t *testing.T) {
	paths, err := Prepare(filepath.Join(t.TempDir(), "studio"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Database, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(paths.Artifacts, "blob")
	if err := os.WriteFile(artifact, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Inspect(paths.Root); err == nil {
		t.Fatal("expected public artifact permissions to be rejected")
	}
}
