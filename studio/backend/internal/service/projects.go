package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"studio-backend/internal/domain"
	"studio-backend/internal/identity"
	"studio-backend/internal/repository"
)

type Projects struct {
	store          repository.ProjectStore
	now            func() time.Time
	newID          func() (string, error)
	maxNodes       int
	maxConnections int
}

type InvalidInput struct{ Err error }

func (e *InvalidInput) Error() string { return e.Err.Error() }
func (e *InvalidInput) Unwrap() error { return e.Err }

func NewProjects(store repository.ProjectStore, maxNodes, maxConnections int) (*Projects, error) {
	if store == nil {
		return nil, errors.New("project store is required")
	}
	return &Projects{store: store, now: time.Now, newID: randomProjectID, maxNodes: maxNodes, maxConnections: maxConnections}, nil
}

func (s *Projects) Create(ctx context.Context, name, description string) (domain.Project, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	name = strings.TrimSpace(name)
	if err := domain.ValidateProject(name, description); err != nil {
		return domain.Project{}, &InvalidInput{Err: err}
	}
	id, err := s.newID()
	if err != nil {
		return domain.Project{}, err
	}
	now := s.now().UTC()
	return s.store.CreateProject(ctx, ownerID, domain.Project{ID: id, Name: name, Description: description, Revision: 1, CreatedAt: now, UpdatedAt: now})
}

func (s *Projects) List(ctx context.Context, limit int, cursor string) (repository.ProjectPage, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return repository.ProjectPage{}, err
	}
	return s.store.ListProjects(ctx, ownerID, limit, cursor)
}

func (s *Projects) Get(ctx context.Context, projectID string) (domain.Project, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	return s.store.GetProject(ctx, ownerID, projectID)
}

func (s *Projects) Update(ctx context.Context, projectID string, expected int64, changes repository.ProjectChanges) (domain.Project, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	current, err := s.store.GetProject(ctx, ownerID, projectID)
	if err != nil {
		return domain.Project{}, err
	}
	name, description := current.Name, current.Description
	if changes.Name != nil {
		name = strings.TrimSpace(*changes.Name)
	}
	if changes.Description != nil {
		description = *changes.Description
	}
	if changes.Name == nil && changes.Description == nil {
		return domain.Project{}, &InvalidInput{Err: errors.New("at least one project field is required")}
	}
	if err := domain.ValidateProject(name, description); err != nil {
		return domain.Project{}, &InvalidInput{Err: err}
	}
	return s.store.UpdateProject(ctx, ownerID, projectID, expected, repository.ProjectChanges{Name: &name, Description: &description}, s.now().UTC())
}

func (s *Projects) GetTopology(ctx context.Context, projectID string, environment domain.Environment) (domain.Topology, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Topology{}, err
	}
	return s.store.GetTopology(ctx, ownerID, projectID, environment)
}

func (s *Projects) SaveTopology(ctx context.Context, projectID string, environment domain.Environment, expected int64, blueprint, layout json.RawMessage) (domain.Topology, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Topology{}, err
	}
	if err := domain.ValidateTopologyStructure(blueprint, layout, s.maxNodes, s.maxConnections); err != nil {
		return domain.Topology{}, &InvalidInput{Err: err}
	}
	return s.store.SaveTopology(ctx, ownerID, domain.Topology{ProjectID: projectID, Environment: environment, Blueprint: cloneJSON(blueprint), Layout: cloneJSON(layout), UpdatedAt: s.now().UTC()}, expected)
}

func requestOwner(ctx context.Context) (string, error) {
	ownerID, ok := identity.OwnerID(ctx)
	if !ok {
		return "", errors.New("authenticated owner is missing from request context")
	}
	return ownerID, nil
}

func cloneJSON(value json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), value...) }

func randomProjectID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "prj_" + hex.EncodeToString(value[:]), nil
}
