package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"studio-backend/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidCursor = errors.New("invalid cursor")
)

type RevisionConflict struct {
	Expected int64
	Actual   int64
}

func (e *RevisionConflict) Error() string {
	return fmt.Sprintf("revision conflict: expected %d, actual %d", e.Expected, e.Actual)
}

type ProjectPage struct {
	Items      []domain.Project
	NextCursor *string
}

type ProjectChanges struct {
	Name        *string
	Description *string
}

type ProjectStore interface {
	CreateProject(context.Context, string, domain.Project) (domain.Project, error)
	ListProjects(context.Context, string, int, string) (ProjectPage, error)
	GetProject(context.Context, string, string) (domain.Project, error)
	UpdateProject(context.Context, string, string, int64, ProjectChanges, time.Time) (domain.Project, error)
	GetTopology(context.Context, string, string, domain.Environment) (domain.Topology, error)
	SaveTopology(context.Context, string, domain.Topology, int64) (domain.Topology, error)
}

type TopologyInput struct {
	Blueprint json.RawMessage
	Layout    json.RawMessage
}
