package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"studio-backend/internal/domain"
)

func TestGenerationQueueRecoversAndExhaustsInterruptedJobs(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO projects(id,owner_id,name,description,revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, "project", "owner", "project", "", 1, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	repo := NewDeliveryRepository(db)
	generation := domain.Generation{ID: "generation", ProjectID: "project", Environment: domain.EnvironmentDev, Revision: 1, Generator: "crosswalk-vm", Output: "package", Status: "queued", Snapshot: []byte(`{"blueprint":{}}`), CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateGeneration(ctx, "owner", "key", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", generation); err != nil {
		t.Fatal(err)
	}

	claimed, err := repo.ClaimNextGeneration(ctx, 2, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != "running" || claimed.Attempts != 1 {
		t.Fatalf("first claim=%+v", claimed)
	}
	if recovered, err := repo.RecoverGenerations(ctx, 2, now.Add(2*time.Second)); err != nil || recovered != 1 {
		t.Fatalf("recover=%d err=%v", recovered, err)
	}
	claimed, err = repo.ClaimNextGeneration(ctx, 2, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Attempts != 2 {
		t.Fatalf("second claim=%+v", claimed)
	}
	if recovered, err := repo.RecoverGenerations(ctx, 2, now.Add(4*time.Second)); err != nil || recovered != 1 {
		t.Fatalf("exhaust=%d err=%v", recovered, err)
	}
	failed, err := repo.GetGeneration(ctx, "owner", "project", "generation")
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.Error == nil || failed.Error.Code != "WORKER_RETRY_EXHAUSTED" {
		t.Fatalf("failed generation=%+v", failed)
	}
}
