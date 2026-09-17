package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotIncludesCommittedWALData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "studio.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE snapshot_test(value TEXT); INSERT INTO snapshot_test(value) VALUES ('kept')"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Snapshot(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	restored := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(restored, snapshot, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckIntegrity(ctx, restored); err != nil {
		t.Fatal(err)
	}
	copyDB, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var value string
	if err := copyDB.QueryRowContext(ctx, "SELECT value FROM snapshot_test").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "kept" {
		t.Fatalf("snapshot value = %q", value)
	}
}
