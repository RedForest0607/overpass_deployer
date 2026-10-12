package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type failingReferenceRows struct {
	yielded, closed        bool
	iterationErr, closeErr error
}

func (r *failingReferenceRows) Next() bool {
	if r.yielded {
		return false
	}
	r.yielded = true
	return true
}
func (r *failingReferenceRows) Scan(values ...any) error {
	*values[0].(*string) = "live-blob"
	return nil
}
func (r *failingReferenceRows) Err() error   { return r.iterationErr }
func (r *failingReferenceRows) Close() error { r.closed = true; return r.closeErr }

func TestBlobReferenceIterationAndCloseErrorsAreNotIgnored(t *testing.T) {
	for _, failIteration := range []bool{true, false} {
		t.Run(map[bool]string{true: "iteration", false: "close"}[failIteration], func(t *testing.T) {
			failure := errors.New("reference query interrupted")
			rows := &failingReferenceRows{}
			if failIteration {
				rows.iterationErr = failure
			} else {
				rows.closeErr = failure
			}
			err := collectBlobReferences(rows, map[string]struct{}{}, directBlobKey)
			if !errors.Is(err, failure) || !rows.closed {
				t.Fatalf("err=%v closed=%v", err, rows.closed)
			}
		})
	}
}

func TestReferencedBlobKeysRejectsInvalidManifest(t *testing.T) {
	for _, manifest := range []string{`invalid JSON`, `{}`, `{"zipBlobKey":42}`} {
		t.Run(manifest, func(t *testing.T) {
			ctx := context.Background()
			db, err := Open(ctx, filepath.Join(t.TempDir(), "studio.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`INSERT INTO projects(id,owner_id,name,description,revision,created_at,updated_at) VALUES('p','owner','p','',1,'2026-09-28T00:00:00Z','2026-09-28T00:00:00Z')`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO packages(id,project_id,environment,topology_revision,output,generator,generator_version,chart_format,manifest_json,created_at) VALUES('pkg','p','dev',1,'package','crosswalk-vm','1.0.0','crosswalk/v1',?,'2026-09-28T00:00:00Z')`, manifest); err != nil {
				t.Fatal(err)
			}
			artifacts, packages, err := NewDeliveryRepository(db).ReferencedBlobKeys(ctx)
			if err == nil || artifacts != nil || packages != nil {
				t.Fatalf("unsafe reference snapshot: artifacts=%v packages=%v err=%v", artifacts, packages, err)
			}
		})
	}
}
