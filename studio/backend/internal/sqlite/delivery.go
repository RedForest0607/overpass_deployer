package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"studio-backend/internal/domain"
	"studio-backend/internal/repository"
)

type DeliveryRepository struct{ db *sql.DB }

func NewDeliveryRepository(db *sql.DB) *DeliveryRepository { return &DeliveryRepository{db: db} }

func (r *DeliveryRepository) ProjectOwner(ctx context.Context, projectID string) (string, error) {
	var ownerID string
	if err := r.db.QueryRowContext(ctx, `SELECT owner_id FROM projects WHERE id=?`, projectID).Scan(&ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", repository.ErrNotFound
		}
		return "", err
	}
	return ownerID, nil
}

func (r *DeliveryRepository) CreateArtifact(ctx context.Context, ownerID string, value domain.Artifact) (domain.Artifact, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO artifacts(id,project_id,kind,original_name,blob_key,size_bytes,sha256,created_at)
		SELECT ?,?,?,?,?,?,?,? WHERE EXISTS (SELECT 1 FROM projects WHERE id=? AND owner_id=?)`, value.ID, value.ProjectID, value.Kind, value.OriginalName, value.BlobKey, value.SizeBytes, value.SHA256, formatTime(value.CreatedAt), value.ProjectID, ownerID)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("create artifact: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return domain.Artifact{}, repository.ErrNotFound
	}
	return value, nil
}

type artifactCursor struct{ CreatedAt, ID string }

func (r *DeliveryRepository) ListArtifacts(ctx context.Context, ownerID, projectID string, limit int, cursorValue string) (repository.ArtifactPage, error) {
	query := `SELECT a.id,a.project_id,a.kind,a.original_name,a.blob_key,a.size_bytes,a.sha256,a.created_at FROM artifacts a JOIN projects p ON p.id=a.project_id WHERE p.owner_id=? AND a.project_id=?`
	args := []any{ownerID, projectID}
	if cursorValue != "" {
		cursor, err := decodeArtifactCursor(cursorValue)
		if err != nil {
			return repository.ArtifactPage{}, err
		}
		query += ` AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	query += ` ORDER BY a.created_at DESC,a.id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return repository.ArtifactPage{}, err
	}
	defer rows.Close()
	items := make([]domain.Artifact, 0, limit+1)
	for rows.Next() {
		value, err := scanArtifact(rows)
		if err != nil {
			return repository.ArtifactPage{}, err
		}
		items = append(items, value)
	}
	page := repository.ArtifactPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		encoded, _ := encodeArtifactCursor(last)
		page.Items = items[:limit]
		page.NextCursor = &encoded
	}
	if len(items) == 0 {
		if _, err := r.projectExists(ctx, ownerID, projectID); err != nil {
			return repository.ArtifactPage{}, err
		}
	}
	return page, rows.Err()
}

func (r *DeliveryRepository) GetArtifact(ctx context.Context, ownerID, projectID, artifactID string) (domain.Artifact, error) {
	value, err := scanArtifact(r.db.QueryRowContext(ctx, `SELECT a.id,a.project_id,a.kind,a.original_name,a.blob_key,a.size_bytes,a.sha256,a.created_at FROM artifacts a JOIN projects p ON p.id=a.project_id WHERE p.owner_id=? AND a.project_id=? AND a.id=?`, ownerID, projectID, artifactID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Artifact{}, repository.ErrNotFound
	}
	return value, err
}

func (r *DeliveryRepository) CreateGeneration(ctx context.Context, ownerID, key, requestHash string, generation domain.Generation) (domain.Generation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Generation{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO generations(id,project_id,environment,topology_revision,snapshot_json,generator,output,status,attempts,package_id,error_json,created_at,updated_at,retry_of_generation_id)
		SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,? WHERE EXISTS (SELECT 1 FROM projects WHERE id=? AND owner_id=?)`, generation.ID, generation.ProjectID, generation.Environment, generation.Revision, string(generation.Snapshot), generation.Generator, generation.Output, "queued", 0, nil, nil, formatTime(generation.CreatedAt), formatTime(generation.UpdatedAt), generation.RetryOfGenerationID, generation.ProjectID, ownerID); err != nil {
		return domain.Generation{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO idempotency_keys(owner_id,project_id,idempotency_key,request_hash,generation_id,expires_at) VALUES(?,?,?,?,?,?)`, ownerID, generation.ProjectID, key, requestHash, generation.ID, formatTime(generation.CreatedAt.Add(24*time.Hour))); err != nil {
		return domain.Generation{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.Generation{}, err
	}
	generation.Status = "queued"
	return generation, nil
}

func (r *DeliveryRepository) FindGenerationByKey(ctx context.Context, ownerID, projectID, key, requestHash string) (domain.Generation, error) {
	var storedHash string
	row := r.db.QueryRowContext(ctx, `SELECT g.id,g.project_id,g.environment,g.topology_revision,g.snapshot_json,g.generator,g.output,g.status,g.attempts,g.package_id,g.error_json,g.created_at,g.updated_at,g.retry_of_generation_id,k.request_hash FROM idempotency_keys k JOIN generations g ON g.id=k.generation_id JOIN projects p ON p.id=g.project_id WHERE k.owner_id=? AND k.project_id=? AND k.idempotency_key=? AND p.owner_id=?`, ownerID, projectID, key, ownerID)
	value, err := scanGeneration(row, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Generation{}, repository.ErrNotFound
	}
	if err != nil {
		return domain.Generation{}, err
	}
	if storedHash != requestHash {
		return domain.Generation{}, repository.ErrIdempotencyConflict
	}
	return value, nil
}
func (r *DeliveryRepository) GetGeneration(ctx context.Context, ownerID, projectID, id string) (domain.Generation, error) {
	value, err := scanGeneration(r.db.QueryRowContext(ctx, `SELECT g.id,g.project_id,g.environment,g.topology_revision,g.snapshot_json,g.generator,g.output,g.status,g.attempts,g.package_id,g.error_json,g.created_at,g.updated_at,g.retry_of_generation_id FROM generations g JOIN projects p ON p.id=g.project_id WHERE p.owner_id=? AND g.project_id=? AND g.id=?`, ownerID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Generation{}, repository.ErrNotFound
	}
	return value, err
}

type generationCursor struct{ CreatedAt, ID, Environment string }

func (r *DeliveryRepository) ListGenerations(ctx context.Context, ownerID, projectID, environment string, limit int, cursorValue string) (repository.GenerationPage, error) {
	query := `SELECT g.id,g.project_id,g.environment,g.topology_revision,g.snapshot_json,g.generator,g.output,g.status,g.attempts,g.package_id,g.error_json,g.created_at,g.updated_at,g.retry_of_generation_id FROM generations g JOIN projects p ON p.id=g.project_id WHERE p.owner_id=? AND g.project_id=?`
	args := []any{ownerID, projectID}
	if environment != "" {
		query += ` AND g.environment=?`
		args = append(args, environment)
	}
	if cursorValue != "" {
		cursor, err := decodeGenerationCursor(cursorValue)
		if err != nil {
			return repository.GenerationPage{}, err
		}
		if cursor.Environment != environment {
			return repository.GenerationPage{}, repository.ErrInvalidCursor
		}
		query += ` AND (g.created_at < ? OR (g.created_at = ? AND g.id < ?))`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	query += ` ORDER BY g.created_at DESC,g.id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return repository.GenerationPage{}, err
	}
	defer rows.Close()
	items := make([]domain.Generation, 0, limit+1)
	for rows.Next() {
		value, err := scanGeneration(rows)
		if err != nil {
			return repository.GenerationPage{}, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return repository.GenerationPage{}, err
	}
	page := repository.GenerationPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		encoded, err := encodeGenerationCursor(last, environment)
		if err != nil {
			return repository.GenerationPage{}, err
		}
		page.Items = items[:limit]
		page.NextCursor = &encoded
	}
	if len(items) == 0 {
		if _, err := r.projectExists(ctx, ownerID, projectID); err != nil {
			return repository.GenerationPage{}, err
		}
	}
	return page, nil
}

func (r *DeliveryRepository) ClaimNextGeneration(ctx context.Context, maxAttempts int, now time.Time) (domain.Generation, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE generations SET status='running',attempts=attempts+1,updated_at=?
		WHERE id=(SELECT id FROM generations WHERE status='queued' AND attempts<? ORDER BY created_at,id LIMIT 1) AND status='queued'
		RETURNING id,project_id,environment,topology_revision,snapshot_json,generator,output,status,attempts,package_id,error_json,created_at,updated_at,retry_of_generation_id`, formatTime(now), maxAttempts)
	value, err := scanGeneration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Generation{}, repository.ErrNotFound
	}
	return value, err
}

func (r *DeliveryRepository) CompleteGeneration(ctx context.Context, generationID string, pkg domain.Package, now time.Time) (domain.Generation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Generation{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO packages(id,project_id,environment,topology_revision,output,generator,generator_version,chart_format,manifest_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, pkg.ID, pkg.ProjectID, pkg.Environment, pkg.Revision, pkg.Output, pkg.Generator, pkg.GeneratorVersion, pkg.ChartFormat, mustJSON(map[string]string{"zipBlobKey": pkg.ZIPBlobKey}), formatTime(pkg.CreatedAt)); err != nil {
		return domain.Generation{}, err
	}
	for _, file := range pkg.Files {
		if _, err = tx.ExecContext(ctx, `INSERT INTO package_files(id,package_id,path,blob_key,media_type,size_bytes,sha256,previewable) VALUES(?,?,?,?,?,?,?,?)`, file.ID, pkg.ID, file.Path, file.BlobKey, file.MediaType, file.SizeBytes, file.SHA256, file.Previewable); err != nil {
			return domain.Generation{}, err
		}
	}
	value, err := scanGeneration(tx.QueryRowContext(ctx, `UPDATE generations SET status='succeeded',package_id=?,error_json=NULL,updated_at=? WHERE id=? AND status='running'
		RETURNING id,project_id,environment,topology_revision,snapshot_json,generator,output,status,attempts,package_id,error_json,created_at,updated_at,retry_of_generation_id`, pkg.ID, formatTime(now), generationID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Generation{}, repository.ErrNotFound
		}
		return domain.Generation{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.Generation{}, err
	}
	return value, nil
}

func (r *DeliveryRepository) FailGeneration(ctx context.Context, generationID string, failure domain.Failure, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE generations SET status='failed',package_id=NULL,error_json=?,updated_at=? WHERE id=? AND status='running'`, mustJSON(failure), formatTime(now), generationID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *DeliveryRepository) RequeueGeneration(ctx context.Context, generationID string, failure domain.Failure, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE generations SET status='queued',package_id=NULL,error_json=?,updated_at=? WHERE id=? AND status='running'`, mustJSON(failure), formatTime(now), generationID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *DeliveryRepository) RecoverGenerations(ctx context.Context, maxAttempts int, now time.Time) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	failure := mustJSON(domain.Failure{Code: "WORKER_RETRY_EXHAUSTED", Message: "패키지 생성 작업이 재시도 한도를 초과했습니다."})
	failed, err := tx.ExecContext(ctx, `UPDATE generations SET status='failed',error_json=?,updated_at=? WHERE status IN ('queued','running') AND attempts>=?`, failure, formatTime(now), maxAttempts)
	if err != nil {
		return 0, err
	}
	requeued, err := tx.ExecContext(ctx, `UPDATE generations SET status='queued',error_json=NULL,updated_at=? WHERE status='running' AND attempts<?`, formatTime(now), maxAttempts)
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	failedCount, _ := failed.RowsAffected()
	requeuedCount, _ := requeued.RowsAffected()
	return failedCount + requeuedCount, nil
}

func (r *DeliveryRepository) ReferencedBlobKeys(ctx context.Context) (map[string]struct{}, map[string]struct{}, error) {
	artifactKeys, packageKeys := map[string]struct{}{}, map[string]struct{}{}
	rows, err := r.db.QueryContext(ctx, `SELECT blob_key FROM artifacts`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, nil, err
		}
		artifactKeys[key] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT manifest_json FROM packages`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var manifest string
		if err := rows.Scan(&manifest); err != nil {
			rows.Close()
			return nil, nil, err
		}
		var metadata map[string]string
		if json.Unmarshal([]byte(manifest), &metadata) == nil && metadata["zipBlobKey"] != "" {
			packageKeys[metadata["zipBlobKey"]] = struct{}{}
		}
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT blob_key FROM package_files WHERE blob_key<>''`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, nil, err
		}
		packageKeys[key] = struct{}{}
	}
	return artifactKeys, packageKeys, rows.Err()
}

func (r *DeliveryRepository) GetPackage(ctx context.Context, ownerID, projectID, id string) (domain.Package, error) {
	var value domain.Package
	var environment, createdAt, manifest string
	err := r.db.QueryRowContext(ctx, `SELECT x.id,x.project_id,x.environment,x.topology_revision,x.output,x.generator,x.generator_version,x.chart_format,x.manifest_json,x.created_at FROM packages x JOIN projects p ON p.id=x.project_id WHERE p.owner_id=? AND x.project_id=? AND x.id=?`, ownerID, projectID, id).Scan(&value.ID, &value.ProjectID, &environment, &value.Revision, &value.Output, &value.Generator, &value.GeneratorVersion, &value.ChartFormat, &manifest, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Package{}, repository.ErrNotFound
	}
	if err != nil {
		return domain.Package{}, err
	}
	value.Environment = domain.Environment(environment)
	value.CreatedAt, _ = parseTime(createdAt)
	var metadata map[string]string
	_ = json.Unmarshal([]byte(manifest), &metadata)
	value.ZIPBlobKey = metadata["zipBlobKey"]
	rows, err := r.db.QueryContext(ctx, `SELECT id,path,blob_key,media_type,size_bytes,sha256,previewable FROM package_files WHERE package_id=? ORDER BY path`, id)
	if err != nil {
		return domain.Package{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var f domain.PackageFile
		if err := rows.Scan(&f.ID, &f.Path, &f.BlobKey, &f.MediaType, &f.SizeBytes, &f.SHA256, &f.Previewable); err != nil {
			return domain.Package{}, err
		}
		value.Files = append(value.Files, f)
	}
	if value.Files == nil {
		value.Files = []domain.PackageFile{}
	}
	return value, rows.Err()
}
func (r *DeliveryRepository) GetPackageFile(ctx context.Context, ownerID, projectID, packageID, fileID string) (domain.PackageFile, error) {
	var f domain.PackageFile
	err := r.db.QueryRowContext(ctx, `SELECT f.id,f.path,f.blob_key,f.media_type,f.size_bytes,f.sha256,f.previewable FROM package_files f JOIN packages x ON x.id=f.package_id JOIN projects p ON p.id=x.project_id WHERE p.owner_id=? AND x.project_id=? AND x.id=? AND f.id=?`, ownerID, projectID, packageID, fileID).Scan(&f.ID, &f.Path, &f.BlobKey, &f.MediaType, &f.SizeBytes, &f.SHA256, &f.Previewable)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PackageFile{}, repository.ErrNotFound
	}
	return f, err
}

func scanArtifact(row scanner) (domain.Artifact, error) {
	var v domain.Artifact
	var kind, created string
	err := row.Scan(&v.ID, &v.ProjectID, &kind, &v.OriginalName, &v.BlobKey, &v.SizeBytes, &v.SHA256, &created)
	v.Kind = domain.ArtifactKind(kind)
	if err == nil {
		v.CreatedAt, err = parseTime(created)
	}
	return v, err
}
func scanGeneration(row scanner, extra ...*string) (domain.Generation, error) {
	var v domain.Generation
	var env, snapshot, createdAt, updatedAt string
	var packageID sql.NullString
	var retryOfGenerationID sql.NullString
	var failure sql.NullString
	args := []any{&v.ID, &v.ProjectID, &env, &v.Revision, &snapshot, &v.Generator, &v.Output, &v.Status, &v.Attempts, &packageID, &failure, &createdAt, &updatedAt, &retryOfGenerationID}
	if len(extra) > 0 {
		args = append(args, extra[0])
	}
	err := row.Scan(args...)
	if err != nil {
		return v, err
	}
	v.Environment = domain.Environment(env)
	v.Snapshot = json.RawMessage(snapshot)
	v.CreatedAt, _ = parseTime(createdAt)
	v.UpdatedAt, _ = parseTime(updatedAt)
	if packageID.Valid {
		v.PackageID = &packageID.String
	}
	if retryOfGenerationID.Valid {
		v.RetryOfGenerationID = &retryOfGenerationID.String
	}
	if failure.Valid {
		var f domain.Failure
		_ = json.Unmarshal([]byte(failure.String), &f)
		v.Error = &f
	}
	return v, nil
}

func encodeGenerationCursor(v domain.Generation, environment string) (string, error) {
	body, err := json.Marshal(generationCursor{CreatedAt: formatTime(v.CreatedAt), ID: v.ID, Environment: environment})
	return base64.RawURLEncoding.EncodeToString(body), err
}

func decodeGenerationCursor(value string) (generationCursor, error) {
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return generationCursor{}, repository.ErrInvalidCursor
	}
	var cursor generationCursor
	if json.Unmarshal(body, &cursor) != nil || cursor.CreatedAt == "" || cursor.ID == "" {
		return generationCursor{}, repository.ErrInvalidCursor
	}
	return cursor, nil
}
func (r *DeliveryRepository) projectExists(ctx context.Context, owner, id string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE owner_id=? AND id=?`, owner, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, repository.ErrNotFound
	}
	return err == nil, err
}
func encodeArtifactCursor(v domain.Artifact) (string, error) {
	b, e := json.Marshal(artifactCursor{formatTime(v.CreatedAt), v.ID})
	return base64.RawURLEncoding.EncodeToString(b), e
}
func decodeArtifactCursor(v string) (artifactCursor, error) {
	b, e := base64.RawURLEncoding.DecodeString(v)
	if e != nil {
		return artifactCursor{}, repository.ErrInvalidCursor
	}
	var c artifactCursor
	if json.Unmarshal(b, &c) != nil || c.ID == "" {
		return c, repository.ErrInvalidCursor
	}
	return c, nil
}
func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
