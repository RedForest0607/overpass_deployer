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

type ProjectRepository struct{ db *sql.DB }

func NewProjectRepository(db *sql.DB) *ProjectRepository { return &ProjectRepository{db: db} }

func (r *ProjectRepository) CreateProject(ctx context.Context, ownerID string, project domain.Project) (domain.Project, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO projects(id, owner_id, name, description, revision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, project.ID, ownerID, project.Name, project.Description, project.Revision, formatTime(project.CreatedAt), formatTime(project.UpdatedAt))
	if err != nil {
		return domain.Project{}, fmt.Errorf("create project: %w", err)
	}
	return project, nil
}

type projectCursor struct {
	UpdatedAt string `json:"updatedAt"`
	ID        string `json:"id"`
}

func (r *ProjectRepository) ListProjects(ctx context.Context, ownerID string, limit int, encodedCursor string) (repository.ProjectPage, error) {
	if limit < 1 || limit > 100 {
		return repository.ProjectPage{}, errors.New("limit must be between 1 and 100")
	}
	query := `SELECT id, name, description, revision, created_at, updated_at FROM projects WHERE owner_id = ?`
	args := []any{ownerID}
	if encodedCursor != "" {
		cursor, err := decodeProjectCursor(encodedCursor)
		if err != nil {
			return repository.ProjectPage{}, err
		}
		query += ` AND (updated_at < ? OR (updated_at = ? AND id < ?))`
		args = append(args, cursor.UpdatedAt, cursor.UpdatedAt, cursor.ID)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return repository.ProjectPage{}, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Project, 0, limit+1)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return repository.ProjectPage{}, err
		}
		items = append(items, project)
	}
	if err := rows.Err(); err != nil {
		return repository.ProjectPage{}, fmt.Errorf("iterate projects: %w", err)
	}
	page := repository.ProjectPage{Items: items}
	if len(items) > limit {
		last := items[limit-1]
		cursor, err := encodeProjectCursor(last)
		if err != nil {
			return repository.ProjectPage{}, err
		}
		page.Items = items[:limit]
		page.NextCursor = &cursor
	}
	return page, nil
}

func (r *ProjectRepository) GetProject(ctx context.Context, ownerID, projectID string) (domain.Project, error) {
	project, err := scanProject(r.db.QueryRowContext(ctx, `SELECT id, name, description, revision, created_at, updated_at
		FROM projects WHERE owner_id = ? AND id = ?`, ownerID, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, repository.ErrNotFound
	}
	return project, err
}

func (r *ProjectRepository) UpdateProject(ctx context.Context, ownerID, projectID string, expected int64, changes repository.ProjectChanges, updatedAt time.Time) (domain.Project, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Project{}, fmt.Errorf("begin project update: %w", err)
	}
	defer tx.Rollback()
	var actual int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM projects WHERE owner_id = ? AND id = ?`, ownerID, projectID).Scan(&actual); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Project{}, repository.ErrNotFound
		}
		return domain.Project{}, fmt.Errorf("read project revision: %w", err)
	}
	if actual != expected {
		return domain.Project{}, &repository.RevisionConflict{Expected: expected, Actual: actual}
	}
	result, err := tx.ExecContext(ctx, `UPDATE projects SET name = ?, description = ?, revision = revision + 1, updated_at = ?
		WHERE owner_id = ? AND id = ? AND revision = ?`, *changes.Name, *changes.Description, formatTime(updatedAt), ownerID, projectID, expected)
	if err != nil {
		return domain.Project{}, fmt.Errorf("update project: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Project{}, fmt.Errorf("read affected project rows: %w", err)
	}
	if affected != 1 {
		return domain.Project{}, fmt.Errorf("update project affected %d rows", affected)
	}
	project, err := scanProject(tx.QueryRowContext(ctx, `SELECT id, name, description, revision, created_at, updated_at
		FROM projects WHERE owner_id = ? AND id = ?`, ownerID, projectID))
	if err != nil {
		return domain.Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Project{}, fmt.Errorf("commit project update: %w", err)
	}
	return project, nil
}

func (r *ProjectRepository) GetTopology(ctx context.Context, ownerID, projectID string, environment domain.Environment) (domain.Topology, error) {
	return scanTopology(r.db.QueryRowContext(ctx, `SELECT t.project_id, t.environment, t.revision, t.updated_at, t.blueprint_json, t.layout_json
		FROM topologies t JOIN projects p ON p.id = t.project_id
		WHERE p.owner_id = ? AND t.project_id = ? AND t.environment = ?`, ownerID, projectID, environment))
}

func (r *ProjectRepository) SaveTopology(ctx context.Context, ownerID string, topology domain.Topology, expected int64) (domain.Topology, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Topology{}, fmt.Errorf("begin topology save: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE owner_id = ? AND id = ?`, ownerID, topology.ProjectID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Topology{}, repository.ErrNotFound
		}
		return domain.Topology{}, fmt.Errorf("read topology project: %w", err)
	}
	var actual int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM topologies WHERE project_id = ? AND environment = ?`, topology.ProjectID, topology.Environment).Scan(&actual)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.Topology{}, fmt.Errorf("read topology revision: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		actual = 0
	}
	if actual != expected {
		return domain.Topology{}, &repository.RevisionConflict{Expected: expected, Actual: actual}
	}
	topology.Revision = actual + 1
	var result sql.Result
	if actual == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO topologies(project_id, environment, revision, blueprint_json, layout_json, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`, topology.ProjectID, topology.Environment, topology.Revision, string(topology.Blueprint), string(topology.Layout), formatTime(topology.UpdatedAt))
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE topologies SET revision = ?, blueprint_json = ?, layout_json = ?, updated_at = ?
			WHERE project_id = ? AND environment = ? AND revision = ?`, topology.Revision, string(topology.Blueprint), string(topology.Layout), formatTime(topology.UpdatedAt), topology.ProjectID, topology.Environment, expected)
	}
	if err != nil {
		return domain.Topology{}, fmt.Errorf("save topology: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Topology{}, fmt.Errorf("read affected topology rows: %w", err)
	}
	if affected != 1 {
		return domain.Topology{}, &repository.RevisionConflict{Expected: expected, Actual: actual}
	}
	if err := tx.Commit(); err != nil {
		return domain.Topology{}, fmt.Errorf("commit topology save: %w", err)
	}
	return topology, nil
}

type scanner interface{ Scan(...any) error }

func scanProject(row scanner) (domain.Project, error) {
	var project domain.Project
	var createdAt, updatedAt string
	if err := row.Scan(&project.ID, &project.Name, &project.Description, &project.Revision, &createdAt, &updatedAt); err != nil {
		return domain.Project{}, err
	}
	var err error
	if project.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Project{}, err
	}
	if project.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

func scanTopology(row scanner) (domain.Topology, error) {
	var topology domain.Topology
	var environment, updatedAt, blueprint, layout string
	if err := row.Scan(&topology.ProjectID, &environment, &topology.Revision, &updatedAt, &blueprint, &layout); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Topology{}, repository.ErrNotFound
		}
		return domain.Topology{}, err
	}
	topology.Environment = domain.Environment(environment)
	var err error
	if topology.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Topology{}, err
	}
	topology.Blueprint = json.RawMessage(blueprint)
	topology.Layout = json.RawMessage(layout)
	return topology, nil
}

func encodeProjectCursor(project domain.Project) (string, error) {
	body, err := json.Marshal(projectCursor{UpdatedAt: formatTime(project.UpdatedAt), ID: project.ID})
	if err != nil {
		return "", fmt.Errorf("encode project cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func decodeProjectCursor(value string) (projectCursor, error) {
	body, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return projectCursor{}, repository.ErrInvalidCursor
	}
	var cursor projectCursor
	if json.Unmarshal(body, &cursor) != nil || cursor.ID == "" {
		return projectCursor{}, repository.ErrInvalidCursor
	}
	if _, err := parseTime(cursor.UpdatedAt); err != nil {
		return projectCursor{}, repository.ErrInvalidCursor
	}
	return cursor, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored time: %w", err)
	}
	return parsed, nil
}
