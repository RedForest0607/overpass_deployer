package sqlite

import (
	"context"
	"path/filepath"
	"strings"
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

func TestListGenerationsUsesProjectEnvironmentAndCursorScope(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	base := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	for _, project := range []struct{ id, owner string }{{"project", "owner"}, {"other", "owner"}, {"foreign", "someone-else"}} {
		if _, err := db.Exec(`INSERT INTO projects(id,owner_id,name,description,revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, project.id, project.owner, project.id, "", 1, formatTime(base), formatTime(base)); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewDeliveryRepository(db)
	fixtures := []domain.Generation{
		{ID: "dev-new", ProjectID: "project", Environment: domain.EnvironmentDev, Revision: 3, Generator: "crosswalk-vm", Output: "package", Snapshot: []byte(`{}`), CreatedAt: base.Add(3 * time.Minute), UpdatedAt: base.Add(3 * time.Minute)},
		{ID: "qa", ProjectID: "project", Environment: domain.EnvironmentQA, Revision: 2, Generator: "crosswalk-vm", Output: "package", Snapshot: []byte(`{}`), CreatedAt: base.Add(2 * time.Minute), UpdatedAt: base.Add(2 * time.Minute)},
		{ID: "dev-old", ProjectID: "project", Environment: domain.EnvironmentDev, Revision: 1, Generator: "crosswalk-vm", Output: "package", Snapshot: []byte(`{}`), CreatedAt: base.Add(time.Minute), UpdatedAt: base.Add(time.Minute)},
		{ID: "other", ProjectID: "other", Environment: domain.EnvironmentDev, Revision: 1, Generator: "crosswalk-vm", Output: "package", Snapshot: []byte(`{}`), CreatedAt: base.Add(4 * time.Minute), UpdatedAt: base.Add(4 * time.Minute)},
	}
	for _, generation := range fixtures {
		if _, err := repo.CreateGeneration(ctx, "owner", generation.ID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", generation); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.ListGenerations(ctx, "owner", "project", "dev", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "dev-new" || first.NextCursor == nil {
		t.Fatalf("first page=%+v", first)
	}
	if _, err := repo.ListGenerations(ctx, "owner", "project", "qa", 1, *first.NextCursor); err == nil {
		t.Fatal("cursor from another environment should be rejected")
	}
	second, err := repo.ListGenerations(ctx, "owner", "project", "dev", 1, *first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "dev-old" || second.NextCursor != nil {
		t.Fatalf("second page=%+v", second)
	}
	if _, err := repo.ListGenerations(ctx, "owner", "foreign", "", 10, ""); err == nil {
		t.Fatal("foreign owner should not list generations")
	}
	rows, err := db.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT g.id FROM generations g JOIN projects p ON p.id=g.project_id WHERE p.owner_id=? AND g.project_id=? AND g.environment=? ORDER BY g.created_at DESC,g.id DESC LIMIT ?`, "owner", "project", "dev", 10)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail)
		plan.WriteByte('\n')
	}
	if !strings.Contains(plan.String(), "generations_project_environment_created_idx") {
		t.Fatalf("generation history query does not use expected index:\n%s", plan.String())
	}
}
