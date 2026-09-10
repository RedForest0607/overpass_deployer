package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"studio-backend/internal/domain"
	"studio-backend/internal/repository"
)

func TestProjectPaginationAndOwnerScope(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewProjectRepository(db)
	base := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	for index, id := range []string{"first", "second", "third"} {
		created := base.Add(time.Duration(index) * time.Second)
		_, err := store.CreateProject(context.Background(), "owner-a", domain.Project{ID: id, Name: id, Revision: 1, CreatedAt: created, UpdatedAt: created})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = store.CreateProject(context.Background(), "owner-b", domain.Project{ID: "private", Name: "private", Revision: 1, CreatedAt: base, UpdatedAt: base})
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.ListProjects(context.Background(), "owner-a", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "third" || first.NextCursor == nil {
		t.Fatalf("unexpected first page: %+v", first)
	}
	second, err := store.ListProjects(context.Background(), "owner-a", 2, *first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "first" || second.NextCursor != nil {
		t.Fatalf("unexpected second page: %+v", second)
	}
	if _, err := store.GetProject(context.Background(), "owner-a", "private"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("cross-owner read error=%v", err)
	}
}

func TestConcurrentTopologyCreateAllowsOneRevision(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewProjectRepository(db)
	now := time.Now().UTC()
	_, err = store.CreateProject(context.Background(), "owner", domain.Project{ID: "project", Name: "project", Revision: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	topology := domain.Topology{ProjectID: "project", Environment: domain.EnvironmentDev, Blueprint: json.RawMessage(`{}`), Layout: json.RawMessage(`{}`), UpdatedAt: now}
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := store.SaveTopology(context.Background(), "owner", topology, 0)
			errorsChannel <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsChannel)
	var successes, conflicts int
	for err := range errorsChannel {
		if err == nil {
			successes++
			continue
		}
		var conflict *repository.RevisionConflict
		if errors.As(err, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestTopologySurvivesDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "studio.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	store := NewProjectRepository(db)
	now := time.Now().UTC()
	_, err = store.CreateProject(ctx, "owner", domain.Project{ID: "persist", Name: "persist", Revision: 1, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":12,"y":34,"zoom":1}}`)
	_, err = store.SaveTopology(ctx, "owner", domain.Topology{ProjectID: "persist", Environment: domain.EnvironmentDev, Blueprint: blueprint, Layout: layout, UpdatedAt: now}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := NewProjectRepository(reopened).GetTopology(ctx, "owner", "persist", domain.EnvironmentDev)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 1 || string(restored.Blueprint) != string(blueprint) || string(restored.Layout) != string(layout) {
		t.Fatalf("persisted state changed: %+v", restored)
	}
	if _, err = NewProjectRepository(reopened).GetTopology(ctx, "owner", "persist", domain.EnvironmentQA); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("environment leaked: %v", err)
	}
}
