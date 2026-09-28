package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"studio-backend/internal/domain"
	"studio-backend/internal/generator/crosswalkcompose"
	"studio-backend/internal/generator/crosswalkdocker"
	"studio-backend/internal/generator/crosswalkvm"
	"studio-backend/internal/repository"
)

func (s *Delivery) ValidatePackage(ctx context.Context, projectID string, blueprint json.RawMessage, environment, generatorID string, result domain.Validation) domain.Validation {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, domain.Issue{Code: "AUTHENTICATION_REQUIRED", Severity: "error", Stage: "request", NodeIDs: []string{}, ConnectionIDs: []string{}, Path: "/", Message: "authenticated owner is missing"})
		return result
	}
	var generationErr error
	switch generatorID {
	case crosswalkvm.ID:
		resolver := artifactResolver{repo: s.repo, store: s.artifacts, ownerID: ownerID, projectID: projectID}
		_, generationErr = crosswalkvm.Generate(ctx, blueprint, environment, resolver)
	case crosswalkdocker.ID:
		_, generationErr = crosswalkdocker.Generate(blueprint, environment)
	case crosswalkcompose.ID:
		_, generationErr = crosswalkcompose.Generate(blueprint, environment)
	default:
		generationErr = fmt.Errorf("unsupported generator %q", generatorID)
	}
	if generationErr != nil {
		stage, code := "generator", "GENERATOR_INVALID"
		if strings.Contains(generationErr.Error(), "artifact") || errors.Is(generationErr, repository.ErrNotFound) {
			stage, code = "artifact", "ARTIFACT_INVALID"
		}
		result.Valid = false
		result.Issues = append(result.Issues, domain.Issue{Code: code, Severity: "error", Stage: stage, NodeIDs: []string{}, ConnectionIDs: []string{}, Path: "/", Message: generationErr.Error()})
	}
	return result
}

type GenerationInput struct {
	Environment                       domain.Environment
	Revision                          int64
	Generator, Output, IdempotencyKey string
}

type GenerationNotRetryable struct{ Status string }

func (e *GenerationNotRetryable) Error() string {
	return fmt.Sprintf("generation with status %q cannot be retried", e.Status)
}

func (s *Delivery) Generate(ctx context.Context, projectID string, input GenerationInput) (domain.Generation, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Generation{}, err
	}
	if input.Revision < 1 || !supportedGenerator(input.Generator) || (input.Output != "draft" && input.Output != "package") || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		return domain.Generation{}, &InvalidInput{Err: errors.New("generation request is invalid")}
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", input.Environment, input.Revision, input.Generator, input.Output)))
	requestHash := hex.EncodeToString(hash[:])
	if existing, err := s.repo.FindGenerationByKey(ctx, ownerID, projectID, input.IdempotencyKey, requestHash); err == nil {
		return existing, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return domain.Generation{}, err
	}
	topology, err := s.projects.GetTopology(ctx, projectID, input.Environment)
	if err != nil {
		return domain.Generation{}, err
	}
	if topology.Revision != input.Revision {
		return domain.Generation{}, &repository.RevisionConflict{Expected: input.Revision, Actual: topology.Revision}
	}
	generationID, err := newID("gen_")
	if err != nil {
		return domain.Generation{}, err
	}
	now := s.now().UTC()
	snapshot, err := json.Marshal(map[string]any{
		"environment": input.Environment,
		"revision":    topology.Revision,
		"blueprint":   json.RawMessage(topology.Blueprint),
		"generator":   input.Generator,
		"output":      input.Output,
	})
	if err != nil {
		return domain.Generation{}, fmt.Errorf("marshal generation snapshot: %w", err)
	}
	generation := domain.Generation{ID: generationID, ProjectID: projectID, Environment: input.Environment, Revision: input.Revision, Generator: input.Generator, Output: input.Output, Status: "queued", Snapshot: snapshot, CreatedAt: now, UpdatedAt: now}
	created, err := s.repo.CreateGeneration(ctx, ownerID, input.IdempotencyKey, requestHash, generation)
	if err != nil {
		if existing, findErr := s.repo.FindGenerationByKey(ctx, ownerID, projectID, input.IdempotencyKey, requestHash); findErr == nil {
			return existing, nil
		}
		return domain.Generation{}, err
	}
	return created, nil
}

func (s *Delivery) ListGenerations(ctx context.Context, projectID, environment string, limit int, cursor string) (repository.GenerationPage, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return repository.GenerationPage{}, err
	}
	if limit < 1 || limit > 100 {
		return repository.GenerationPage{}, &InvalidInput{Err: errors.New("limit must be between 1 and 100")}
	}
	if environment != "" {
		if _, err := domain.ParseEnvironment(environment); err != nil {
			return repository.GenerationPage{}, &InvalidInput{Err: errors.New("environment is invalid")}
		}
	}
	return s.repo.ListGenerations(ctx, ownerID, projectID, environment, limit, cursor)
}

func (s *Delivery) RetryGeneration(ctx context.Context, projectID, sourceID, idempotencyKey string) (domain.Generation, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Generation{}, err
	}
	key := strings.TrimSpace(idempotencyKey)
	if sourceID == "" || key == "" || len(key) > 128 {
		return domain.Generation{}, &InvalidInput{Err: errors.New("generation retry request is invalid")}
	}
	hash := sha256.Sum256([]byte("generation-retry\n" + sourceID))
	requestHash := hex.EncodeToString(hash[:])
	if existing, err := s.repo.FindGenerationByKey(ctx, ownerID, projectID, key, requestHash); err == nil {
		return existing, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return domain.Generation{}, err
	}
	source, err := s.repo.GetGeneration(ctx, ownerID, projectID, sourceID)
	if err != nil {
		return domain.Generation{}, err
	}
	if source.Status != "failed" {
		return domain.Generation{}, &GenerationNotRetryable{Status: source.Status}
	}
	id, err := newID("gen_")
	if err != nil {
		return domain.Generation{}, err
	}
	now := s.now().UTC()
	retry := domain.Generation{
		ID:                  id,
		ProjectID:           source.ProjectID,
		Environment:         source.Environment,
		Revision:            source.Revision,
		Status:              "queued",
		Generator:           source.Generator,
		Output:              source.Output,
		RetryOfGenerationID: &source.ID,
		Snapshot:            append(json.RawMessage(nil), source.Snapshot...),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	created, err := s.repo.CreateGeneration(ctx, ownerID, key, requestHash, retry)
	if err != nil {
		if existing, findErr := s.repo.FindGenerationByKey(ctx, ownerID, projectID, key, requestHash); findErr == nil {
			return existing, nil
		}
		return domain.Generation{}, err
	}
	return created, nil
}

func (s *Delivery) GetGeneration(ctx context.Context, projectID, id string) (domain.Generation, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Generation{}, err
	}
	return s.repo.GetGeneration(ctx, ownerID, projectID, id)
}
