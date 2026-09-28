package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"studio-backend/internal/domain"
	"studio-backend/internal/localstore"
	"studio-backend/internal/repository"
	"unicode/utf8"
)

func (s *Delivery) CreateArtifact(ctx context.Context, projectID string, kind domain.ArtifactKind, originalName string, reader io.Reader) (domain.Artifact, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Artifact{}, err
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return domain.Artifact{}, err
	}
	name := strings.TrimSpace(path.Base(strings.ReplaceAll(originalName, "\\", "/")))
	if name == "" || name == "." || len(name) > 255 {
		return domain.Artifact{}, &InvalidInput{Err: errors.New("artifact filename is invalid")}
	}
	limit := s.textLimit
	if kind == domain.ArtifactJAR || kind == domain.ArtifactFile || kind == domain.ArtifactArchive {
		limit = s.jarLimit
	}
	staged, err := s.artifacts.Stage(reader, limit)
	if errors.Is(err, localstore.ErrTooLarge) {
		return domain.Artifact{}, &TooLarge{Limit: limit}
	}
	if err != nil {
		return domain.Artifact{}, err
	}
	defer s.artifacts.Abort(staged)
	if err := validateArtifact(kind, name, staged.Path, staged.Size); err != nil {
		return domain.Artifact{}, &InvalidArtifact{Err: err}
	}
	key, err := s.artifacts.Commit(staged)
	if err != nil {
		return domain.Artifact{}, err
	}
	staged.Path = ""
	id, err := newID("art_")
	if err != nil {
		return domain.Artifact{}, err
	}
	value := domain.Artifact{ID: id, ProjectID: projectID, Kind: kind, OriginalName: name, BlobKey: key, SizeBytes: staged.Size, SHA256: staged.SHA256, CreatedAt: s.now().UTC()}
	return s.repo.CreateArtifact(ctx, ownerID, value)
}

type TooLarge struct{ Limit int64 }

func (e *TooLarge) Error() string { return fmt.Sprintf("artifact exceeds %d byte limit", e.Limit) }

type InvalidArtifact struct{ Err error }

func (e *InvalidArtifact) Error() string { return e.Err.Error() }
func (e *InvalidArtifact) Unwrap() error { return e.Err }

func (s *Delivery) ListArtifacts(ctx context.Context, projectID string, limit int, cursor string) (repository.ArtifactPage, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return repository.ArtifactPage{}, err
	}
	if limit < 1 || limit > 100 {
		return repository.ArtifactPage{}, &InvalidInput{Err: errors.New("limit must be between 1 and 100")}
	}
	return s.repo.ListArtifacts(ctx, ownerID, projectID, limit, cursor)
}

func (s *Delivery) ReadArtifactText(ctx context.Context, projectID, artifactID string) (string, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return "", err
	}
	artifact, err := s.repo.GetArtifact(ctx, ownerID, projectID, artifactID)
	if err != nil {
		return "", err
	}
	if artifact.Kind == domain.ArtifactJAR || artifact.Kind == domain.ArtifactArchive || !isEditableTextName(artifact.OriginalName) {
		return "", &InvalidInput{Err: errors.New("artifact is not an editable YAML or XML file")}
	}
	if artifact.SizeBytes > s.previewLimit {
		return "", &TooLarge{Limit: s.previewLimit}
	}
	reader, err := s.artifacts.Open(artifact.BlobKey)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, s.previewLimit+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > s.previewLimit {
		return "", &TooLarge{Limit: s.previewLimit}
	}
	if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		return "", &InvalidInput{Err: errors.New("artifact is not UTF-8 text")}
	}
	return string(body), nil
}
