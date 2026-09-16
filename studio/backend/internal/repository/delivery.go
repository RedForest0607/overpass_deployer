package repository

import (
	"context"
	"errors"
	"time"

	"studio-backend/internal/domain"
)

var ErrIdempotencyConflict = errors.New("idempotency key reused with different request")

type ArtifactPage struct {
	Items      []domain.Artifact
	NextCursor *string
}

type DeliveryStore interface {
	CreateArtifact(context.Context, string, domain.Artifact) (domain.Artifact, error)
	ListArtifacts(context.Context, string, string, int, string) (ArtifactPage, error)
	GetArtifact(context.Context, string, string, string) (domain.Artifact, error)
	CreateGeneration(context.Context, string, string, string, domain.Generation) (domain.Generation, error)
	FindGenerationByKey(context.Context, string, string, string, string) (domain.Generation, error)
	GetGeneration(context.Context, string, string, string) (domain.Generation, error)
	ClaimNextGeneration(context.Context, int, time.Time) (domain.Generation, error)
	CompleteGeneration(context.Context, string, domain.Package, time.Time) (domain.Generation, error)
	RequeueGeneration(context.Context, string, domain.Failure, time.Time) error
	FailGeneration(context.Context, string, domain.Failure, time.Time) error
	RecoverGenerations(context.Context, int, time.Time) (int64, error)
	ReferencedBlobKeys(context.Context) (map[string]struct{}, map[string]struct{}, error)
	GetPackage(context.Context, string, string, string) (domain.Package, error)
	GetPackageFile(context.Context, string, string, string, string) (domain.PackageFile, error)
}
