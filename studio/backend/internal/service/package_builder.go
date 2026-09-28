package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"studio-backend/internal/domain"
	"studio-backend/internal/generator/crosswalkcompose"
	"studio-backend/internal/generator/crosswalkdocker"
	"studio-backend/internal/generator/crosswalkvm"
	"studio-backend/internal/localstore"
	"studio-backend/internal/repository"
)

func (s *Delivery) BuildGeneration(ctx context.Context, generation domain.Generation) (domain.Package, error) {
	var snapshot struct {
		Environment domain.Environment `json:"environment"`
		Revision    int64              `json:"revision"`
		Blueprint   json.RawMessage    `json:"blueprint"`
		Generator   string             `json:"generator"`
		Output      string             `json:"output"`
	}
	if err := json.Unmarshal(generation.Snapshot, &snapshot); err != nil {
		return domain.Package{}, fmt.Errorf("decode generation snapshot: %w", err)
	}
	if snapshot.Environment != generation.Environment || snapshot.Revision != generation.Revision || snapshot.Generator != generation.Generator || snapshot.Output != generation.Output || !supportedGenerator(snapshot.Generator) {
		return domain.Package{}, errors.New("generation snapshot metadata does not match the queued job")
	}
	ownerID, err := s.repo.ProjectOwner(ctx, generation.ProjectID)
	if err != nil {
		return domain.Package{}, fmt.Errorf("load generation project owner: %w", err)
	}
	var deployYAML []byte
	var packageFiles []crosswalkvm.PackageFile
	var inlineFiles []crosswalkcompose.File
	var writeZIP func(io.Writer) error
	var generatorVersion, chartFormat string
	switch snapshot.Generator {
	case crosswalkvm.ID:
		resolver := artifactResolver{repo: s.repo, store: s.artifacts, ownerID: ownerID, projectID: generation.ProjectID}
		bundle, buildErr := crosswalkvm.Generate(ctx, snapshot.Blueprint, string(snapshot.Environment), resolver)
		if buildErr != nil {
			return domain.Package{}, &InvalidArtifact{Err: buildErr}
		}
		deployYAML, packageFiles = bundle.DeployYAML, bundle.Files
		writeZIP = func(w io.Writer) error { return crosswalkvm.WriteZIP(w, bundle) }
		generatorVersion, chartFormat = crosswalkvm.Version, crosswalkvm.ChartFormat
	case crosswalkdocker.ID:
		bundle, buildErr := crosswalkdocker.Generate(snapshot.Blueprint, string(snapshot.Environment))
		if buildErr != nil {
			return domain.Package{}, &InvalidArtifact{Err: buildErr}
		}
		deployYAML = bundle.DeployYAML
		writeZIP = func(w io.Writer) error { return crosswalkdocker.WriteZIP(w, bundle) }
		generatorVersion, chartFormat = crosswalkdocker.Version, crosswalkdocker.ChartFormat
	case crosswalkcompose.ID:
		bundle, buildErr := crosswalkcompose.Generate(snapshot.Blueprint, string(snapshot.Environment))
		if buildErr != nil {
			return domain.Package{}, &InvalidArtifact{Err: buildErr}
		}
		deployYAML, inlineFiles = bundle.DeployYAML, bundle.Files
		writeZIP = func(w io.Writer) error { return crosswalkcompose.WriteZIP(w, bundle) }
		generatorVersion, chartFormat = crosswalkcompose.Version, crosswalkcompose.ChartFormat
	}
	deployStage, err := s.packages.Stage(bytes.NewReader(deployYAML), s.textLimit)
	if err != nil {
		return domain.Package{}, err
	}
	defer s.packages.Abort(deployStage)
	deployKey, err := s.packages.Commit(deployStage)
	if err != nil {
		return domain.Package{}, err
	}
	deployStage.Path = ""
	maxZIP := int64(16 << 20)
	for _, file := range packageFiles {
		if file.Artifact.Size > 0 && maxZIP <= 1<<40-file.Artifact.Size {
			maxZIP += file.Artifact.Size
		}
	}
	for _, file := range inlineFiles {
		if int64(len(file.Body)) > 0 && maxZIP <= 1<<40-int64(len(file.Body)) {
			maxZIP += int64(len(file.Body))
		}
	}
	_, zipKey, err := s.packages.PutGenerated(writeZIP, maxZIP)
	if err != nil {
		return domain.Package{}, err
	}
	packageID, err := newID("pkg_")
	if err != nil {
		return domain.Package{}, err
	}
	now := s.now().UTC()
	files := []domain.PackageFile{{ID: "file_deploy", Path: "deploy.yml", BlobKey: deployKey, MediaType: "application/yaml", SizeBytes: deployStage.Size, SHA256: deployStage.SHA256, Previewable: true}}
	for i, file := range packageFiles {
		files = append(files, domain.PackageFile{ID: fmt.Sprintf("file_%04d", i+1), Path: file.Path, BlobKey: "", MediaType: "application/octet-stream", SizeBytes: file.Artifact.Size, SHA256: file.Artifact.SHA256, Previewable: false})
	}
	for i, file := range inlineFiles {
		stage, stageErr := s.packages.Stage(bytes.NewReader(file.Body), s.textLimit)
		if stageErr != nil {
			return domain.Package{}, stageErr
		}
		defer s.packages.Abort(stage)
		key, commitErr := s.packages.Commit(stage)
		if commitErr != nil {
			return domain.Package{}, commitErr
		}
		stage.Path = ""
		files = append(files, domain.PackageFile{ID: fmt.Sprintf("file_%04d", len(packageFiles)+i+1), Path: file.Path, BlobKey: key, MediaType: "application/yaml", SizeBytes: stage.Size, SHA256: stage.SHA256, Previewable: true})
	}
	return domain.Package{ID: packageID, ProjectID: generation.ProjectID, Environment: generation.Environment, Revision: generation.Revision, Output: generation.Output, Generator: snapshot.Generator, GeneratorVersion: generatorVersion, ChartFormat: chartFormat, ZIPBlobKey: zipKey, CreatedAt: now, Files: files}, nil
}

func supportedGenerator(value string) bool {
	return value == crosswalkvm.ID || value == crosswalkdocker.ID || value == crosswalkcompose.ID
}

type artifactResolver struct {
	repo               repository.DeliveryStore
	store              *localstore.Store
	ownerID, projectID string
}

func (r artifactResolver) Resolve(ctx context.Context, id string) (crosswalkvm.Artifact, error) {
	value, err := r.repo.GetArtifact(ctx, r.ownerID, r.projectID, id)
	if err != nil {
		return crosswalkvm.Artifact{}, err
	}
	return crosswalkvm.Artifact{ID: value.ID, OriginalName: value.OriginalName, Kind: string(value.Kind), Size: value.SizeBytes, SHA256: value.SHA256, Open: func() (io.ReadCloser, error) { return r.store.Open(value.BlobKey) }}, nil
}
