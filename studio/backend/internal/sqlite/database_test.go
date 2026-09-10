package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "studio.db")
	for attempt := 0; attempt < 2; attempt++ {
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		var migrations int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if migrations != 1 {
			db.Close()
			t.Fatalf("migration count=%d, want 1", migrations)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("database mode=%o, want 600", info.Mode().Perm())
	}
}

func TestForeignKeysAreEnabled(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO topologies(project_id, environment, revision, blueprint_json, layout_json, updated_at)
		VALUES ('missing', 'dev', 1, '{}', '{}', '2026-09-10T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected foreign key violation")
	}
}
