package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"studio-backend/internal/domain"
)

func TestCompleteGenerationScopesFileIDsToPackage(t *testing.T) {
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
	for _, generationID := range []string{"generation-one", "generation-two"} {
		if _, err := db.Exec(`INSERT INTO generations(id,project_id,environment,topology_revision,snapshot_json,generator,output,status,attempts,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, generationID, "project", "dev", 1, `{}`, "crosswalk-vm", "package", "running", 1, formatTime(now), formatTime(now)); err != nil {
			t.Fatal(err)
		}
	}

	repository := NewDeliveryRepository(db)
	for i, fixture := range []struct {
		generationID string
		packageID    string
		blobKey      string
	}{
		{generationID: "generation-one", packageID: "package-one", blobKey: "blob-one"},
		{generationID: "generation-two", packageID: "package-two", blobKey: "blob-two"},
	} {
		pkg := packageFixture(fixture.packageID, fixture.blobKey, int64(i+1), now)
		if _, err := repository.CompleteGeneration(ctx, fixture.generationID, pkg, now); err != nil {
			t.Fatalf("complete %s: %v", fixture.generationID, err)
		}
	}

	first, err := repository.GetPackageFile(ctx, "owner", "project", "package-one", "file_deploy")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.GetPackageFile(ctx, "owner", "project", "package-two", "file_deploy")
	if err != nil {
		t.Fatal(err)
	}
	if first.BlobKey != "blob-one" || second.BlobKey != "blob-two" {
		t.Fatalf("package file lookup leaked scope: first=%q second=%q", first.BlobKey, second.BlobKey)
	}
}

func TestCompleteGenerationRollsBackDuplicateFileIDWithinPackage(t *testing.T) {
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
	if _, err := db.Exec(`INSERT INTO generations(id,project_id,environment,topology_revision,snapshot_json,generator,output,status,attempts,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, "generation", "project", "dev", 1, `{}`, "crosswalk-vm", "package", "running", 1, formatTime(now), formatTime(now)); err != nil {
		t.Fatal(err)
	}

	pkg := packageFixture("package", "blob-one", 1, now)
	duplicate := pkg.Files[0]
	duplicate.Path = "another-deploy.yml"
	pkg.Files = append(pkg.Files, duplicate)
	if _, err := NewDeliveryRepository(db).CompleteGeneration(ctx, "generation", pkg, now); err == nil {
		t.Fatal("duplicate file id within one package should be rejected")
	}

	var packageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM packages WHERE id=?`, pkg.ID).Scan(&packageCount); err != nil {
		t.Fatal(err)
	}
	if packageCount != 0 {
		t.Fatalf("partial package was persisted after failure: count=%d", packageCount)
	}
}

func packageFixture(id, blobKey string, revision int64, now time.Time) domain.Package {
	return domain.Package{
		ID:               id,
		ProjectID:        "project",
		Environment:      domain.EnvironmentDev,
		Revision:         revision,
		Output:           "package",
		Generator:        "crosswalk-vm",
		GeneratorVersion: "1",
		ChartFormat:      "crosswalk/v1",
		ZIPBlobKey:       "zip-" + id,
		CreatedAt:        now,
		Files: []domain.PackageFile{{
			ID:          "file_deploy",
			Path:        "deploy.yml",
			BlobKey:     blobKey,
			MediaType:   "application/yaml",
			SizeBytes:   10,
			SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Previewable: true,
		}},
	}
}
