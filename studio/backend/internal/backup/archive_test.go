package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio-backend/internal/sqlite"
	"studio-backend/internal/storage"
)

func TestBackupVerifyAndRestore(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source")
	paths, err := storage.Prepare(source)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(ctx, paths.Database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO projects(id,owner_id,name,description,revision,created_at,updated_at) VALUES('prj_backup','local-user','Backup','',1,'2026-09-16T00:00:00Z','2026-09-16T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(paths.Artifacts, "ab", "abcdef")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("artifact-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "studio-backup.tar.gz")
	manifest, err := Create(ctx, source, archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 2 {
		t.Fatalf("entries = %d", len(manifest.Entries))
	}
	if _, err := Verify(archive); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(ctx, source, archive); err == nil {
		t.Fatal("expected existing backup archive to be preserved")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "restored")
	if _, err := Restore(ctx, archive, target); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, archive, target); err == nil {
		t.Fatal("expected existing restore target to be preserved")
	}
	if err := storage.Inspect(target); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "artifacts", "ab", "abcdef")); err != nil || string(content) != "artifact-data" {
		t.Fatalf("artifact = %q, err = %v", content, err)
	}
	restored, err := sql.Open("sqlite", filepath.Join(target, "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var name string
	if err := restored.QueryRowContext(ctx, "SELECT name FROM projects WHERE id='prj_backup'").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Backup" {
		t.Fatalf("project name = %q", name)
	}
}

func TestManifestRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	created := time.Now().UTC()
	checksum := strings.Repeat("a", 64)
	for _, entries := range [][]Entry{
		{{Path: databasePath, Size: 1, SHA256: checksum}, {Path: "data/artifacts/../../escape", Size: 1, SHA256: checksum}},
		{{Path: databasePath, Size: 1, SHA256: checksum}, {Path: databasePath, Size: 1, SHA256: checksum}},
	} {
		if _, err := validateManifest(Manifest{Format: Format, CreatedAt: created, Entries: entries}); err == nil {
			t.Fatalf("expected entries to be rejected: %+v", entries)
		}
	}
}

func TestRestoreRefusesExistingTargetAndInvalidArchive(t *testing.T) {
	ctx := context.Background()
	target := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.tar.gz")
	if err := os.WriteFile(invalid, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(ctx, invalid, filepath.Join(t.TempDir(), "new")); err == nil {
		t.Fatal("expected invalid archive to be rejected")
	}
	if _, err := Restore(ctx, invalid, target); err == nil {
		t.Fatal("expected restore to fail")
	}
}
