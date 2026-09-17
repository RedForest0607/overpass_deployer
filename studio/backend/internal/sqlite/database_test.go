package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio-backend/migrations"
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
		if migrations != 4 {
			db.Close()
			t.Fatalf("migration count=%d, want 4", migrations)
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			info, statErr := os.Stat(path + suffix)
			if statErr != nil {
				db.Close()
				t.Fatal(statErr)
			}
			if info.Mode().Perm() != 0o600 {
				db.Close()
				t.Fatalf("sqlite file %s mode=%o, want 600", suffix, info.Mode().Perm())
			}
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

func TestPackageFileScopeMigrationPreservesExistingData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "studio.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := configure(ctx, db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	for _, version := range []string{"001_initial.sql", "002_artifact_kinds.sql"} {
		body, err := migrations.Files.ReadFile(version)
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `INSERT INTO projects(id,owner_id,name,description,revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, "project", "owner", "project", "", 1, now, now); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO packages(id,project_id,environment,topology_revision,output,generator,generator_version,chart_format,manifest_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, "package-one", "project", "dev", 1, "package", "crosswalk-vm", "1", "crosswalk/v1", `{}`, now); err != nil {
		db.Close()
		t.Fatal(err)
	}
	sha256 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := db.ExecContext(ctx, `INSERT INTO package_files(id,package_id,path,blob_key,media_type,size_bytes,sha256,previewable) VALUES(?,?,?,?,?,?,?,?)`, "file_deploy", "package-one", "deploy.yml", "blob-one", "application/yaml", 10, sha256, true); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var blobKey string
	if err := db.QueryRowContext(ctx, `SELECT blob_key FROM package_files WHERE package_id=? AND id=?`, "package-one", "file_deploy").Scan(&blobKey); err != nil {
		t.Fatal(err)
	}
	if blobKey != "blob-one" {
		t.Fatalf("blob key=%q, want blob-one", blobKey)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO packages(id,project_id,environment,topology_revision,output,generator,generator_version,chart_format,manifest_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, "package-two", "project", "dev", 2, "package", "crosswalk-vm", "1", "crosswalk/v1", `{}`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO package_files(id,package_id,path,blob_key,media_type,size_bytes,sha256,previewable) VALUES(?,?,?,?,?,?,?,?)`, "file_deploy", "package-two", "deploy.yml", "blob-two", "application/yaml", 10, sha256, true); err != nil {
		t.Fatalf("same file id in another package should be accepted: %v", err)
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
