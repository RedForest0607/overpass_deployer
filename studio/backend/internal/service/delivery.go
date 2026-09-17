package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"studio-backend/internal/domain"
	"studio-backend/internal/generator/crosswalkvm"
	"studio-backend/internal/localstore"
	"studio-backend/internal/repository"
)

type Delivery struct {
	projects                          *Projects
	repo                              repository.DeliveryStore
	artifacts                         *localstore.Store
	packages                          *localstore.Store
	jarLimit, textLimit, previewLimit int64
	now                               func() time.Time
}

func NewDelivery(projects *Projects, repo repository.DeliveryStore, artifacts, packages *localstore.Store, jarLimit, textLimit, previewLimit int64) (*Delivery, error) {
	if projects == nil || repo == nil || artifacts == nil || packages == nil {
		return nil, errors.New("delivery dependencies are required")
	}
	return &Delivery{projects: projects, repo: repo, artifacts: artifacts, packages: packages, jarLimit: jarLimit, textLimit: textLimit, previewLimit: previewLimit, now: time.Now}, nil
}

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

func validateArtifact(kind domain.ArtifactKind, name, filePath string, size int64) error {
	if size == 0 {
		return errors.New("artifact must not be empty")
	}
	switch kind {
	case domain.ArtifactJAR:
		if strings.ToLower(filepath.Ext(name)) != ".jar" {
			return errors.New("JAR artifact must use .jar extension")
		}
		return validateZIP(filePath)
	case domain.ArtifactConfig, domain.ArtifactEnvExample, domain.ArtifactScriptValues:
		body, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
			return errors.New("text artifact must be UTF-8 without NUL bytes")
		}
		return validateEditableTextSyntax(name, body)
	case domain.ArtifactArchive:
		lower := strings.ToLower(name)
		if !(strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz")) {
			return errors.New("archive must be .zip, .tar, .tgz, or .tar.gz")
		}
		if strings.HasSuffix(lower, ".zip") {
			return validateZIP(filePath)
		}
		return validateTar(filePath, strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz"))
	}
	if isEditableTextName(name) {
		body, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
			return errors.New("editable text artifact must be UTF-8 without NUL bytes")
		}
		return validateEditableTextSyntax(name, body)
	}
	return nil
}

func validateEditableTextSyntax(name string, body []byte) error {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".yml"), strings.HasSuffix(lower, ".yaml"):
		decoder := yaml.NewDecoder(bytes.NewReader(body))
		for {
			var value any
			if err := decoder.Decode(&value); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return fmt.Errorf("YAML syntax is invalid: %w", err)
			}
		}
	case strings.HasSuffix(lower, ".xml"):
		decoder := xml.NewDecoder(bytes.NewReader(body))
		depth := 0
		roots := 0
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				if roots != 1 {
					return errors.New("XML syntax is invalid: exactly one root element is required")
				}
				return nil
			}
			if err != nil {
				return fmt.Errorf("XML syntax is invalid: %w", err)
			}
			switch value := token.(type) {
			case xml.StartElement:
				if depth == 0 {
					roots++
					if roots > 1 {
						return errors.New("XML syntax is invalid: multiple root elements")
					}
				}
				depth++
			case xml.EndElement:
				depth--
			case xml.CharData:
				if depth == 0 && strings.TrimSpace(string(value)) != "" {
					return errors.New("XML syntax is invalid: text outside the root element")
				}
			}
		}
	default:
		return nil
	}
}

func isEditableTextName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".xml")
}

func validateZIP(filePath string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return errors.New("artifact is not a valid ZIP/JAR")
	}
	defer reader.Close()
	if len(reader.File) == 0 {
		return errors.New("ZIP/JAR must contain at least one entry")
	}
	if len(reader.File) > 100000 {
		return errors.New("ZIP/JAR has too many entries")
	}
	var total uint64
	for _, entry := range reader.File {
		mode := entry.FileInfo().Mode()
		if strings.Contains(entry.Name, "\\") || (!mode.IsRegular() && !mode.IsDir()) {
			return errors.New("ZIP/JAR contains an unsafe entry type or path")
		}
		clean := path.Clean(entry.Name)
		if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return errors.New("ZIP/JAR contains an unsafe entry path")
		}
		if entry.UncompressedSize64 > 100<<20 && entry.CompressedSize64 > 0 && entry.UncompressedSize64/entry.CompressedSize64 > 100 {
			return errors.New("ZIP/JAR compression ratio is too high")
		}
		total += entry.UncompressedSize64
		if total > 1<<30 {
			return errors.New("ZIP/JAR uncompressed size is too large")
		}
	}
	return nil
}

func validateTar(filePath string, compressed bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	var source io.Reader = file
	if compressed {
		reader, err := gzip.NewReader(file)
		if err != nil {
			return errors.New("archive is not a valid gzip stream")
		}
		defer reader.Close()
		source = reader
	}
	reader := tar.NewReader(source)
	entries := 0
	var total int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("archive is not a valid TAR")
		}
		entries++
		if entries > 100000 {
			return errors.New("TAR has too many entries")
		}
		clean := path.Clean(header.Name)
		if strings.Contains(header.Name, "\\") || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return errors.New("TAR contains an unsafe entry path")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
			return errors.New("TAR contains an unsafe entry type")
		}
		if header.Size < 0 || total > 1<<30-header.Size {
			return errors.New("TAR uncompressed size is too large")
		}
		total += header.Size
	}
	if entries == 0 {
		return errors.New("TAR must contain at least one entry")
	}
	return nil
}

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

func (s *Delivery) ValidatePackage(ctx context.Context, projectID string, blueprint json.RawMessage, environment string, result domain.Validation) domain.Validation {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, domain.Issue{Code: "AUTHENTICATION_REQUIRED", Severity: "error", Stage: "request", NodeIDs: []string{}, ConnectionIDs: []string{}, Path: "/", Message: "authenticated owner is missing"})
		return result
	}
	resolver := artifactResolver{repo: s.repo, store: s.artifacts, ownerID: ownerID, projectID: projectID}
	if _, err := crosswalkvm.Generate(ctx, blueprint, environment, resolver); err != nil {
		stage, code := "generator", "GENERATOR_INVALID"
		if strings.Contains(err.Error(), "artifact") || errors.Is(err, repository.ErrNotFound) {
			stage, code = "artifact", "ARTIFACT_INVALID"
		}
		result.Valid = false
		result.Issues = append(result.Issues, domain.Issue{Code: code, Severity: "error", Stage: stage, NodeIDs: []string{}, ConnectionIDs: []string{}, Path: "/", Message: err.Error()})
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
	if input.Revision < 1 || input.Generator != crosswalkvm.ID || (input.Output != "draft" && input.Output != "package") || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
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
	if snapshot.Environment != generation.Environment || snapshot.Revision != generation.Revision || snapshot.Generator != generation.Generator || snapshot.Output != generation.Output || snapshot.Generator != crosswalkvm.ID {
		return domain.Package{}, errors.New("generation snapshot metadata does not match the queued job")
	}
	ownerID, err := s.repo.ProjectOwner(ctx, generation.ProjectID)
	if err != nil {
		return domain.Package{}, fmt.Errorf("load generation project owner: %w", err)
	}
	resolver := artifactResolver{repo: s.repo, store: s.artifacts, ownerID: ownerID, projectID: generation.ProjectID}
	bundle, err := crosswalkvm.Generate(ctx, snapshot.Blueprint, string(snapshot.Environment), resolver)
	if err != nil {
		return domain.Package{}, &InvalidArtifact{Err: err}
	}
	deployStage, err := s.packages.Stage(bytes.NewReader(bundle.DeployYAML), s.textLimit)
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
	for _, file := range bundle.Files {
		if file.Artifact.Size > 0 && maxZIP <= 1<<40-file.Artifact.Size {
			maxZIP += file.Artifact.Size
		}
	}
	_, zipKey, err := s.packages.PutGenerated(func(w io.Writer) error { return crosswalkvm.WriteZIP(w, bundle) }, maxZIP)
	if err != nil {
		return domain.Package{}, err
	}
	packageID, err := newID("pkg_")
	if err != nil {
		return domain.Package{}, err
	}
	now := s.now().UTC()
	files := []domain.PackageFile{{ID: "file_deploy", Path: "deploy.yml", BlobKey: deployKey, MediaType: "application/yaml", SizeBytes: deployStage.Size, SHA256: deployStage.SHA256, Previewable: true}}
	for i, file := range bundle.Files {
		files = append(files, domain.PackageFile{ID: fmt.Sprintf("file_%04d", i+1), Path: file.Path, BlobKey: "", MediaType: "application/octet-stream", SizeBytes: file.Artifact.Size, SHA256: file.Artifact.SHA256, Previewable: false})
	}
	return domain.Package{ID: packageID, ProjectID: generation.ProjectID, Environment: generation.Environment, Revision: generation.Revision, Output: generation.Output, Generator: crosswalkvm.ID, GeneratorVersion: crosswalkvm.Version, ChartFormat: crosswalkvm.ChartFormat, ZIPBlobKey: zipKey, CreatedAt: now, Files: files}, nil
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

func (s *Delivery) GetGeneration(ctx context.Context, projectID, id string) (domain.Generation, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Generation{}, err
	}
	return s.repo.GetGeneration(ctx, ownerID, projectID, id)
}
func (s *Delivery) GetPackage(ctx context.Context, projectID, id string) (domain.Package, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Package{}, err
	}
	return s.repo.GetPackage(ctx, ownerID, projectID, id)
}
func (s *Delivery) OpenPackage(ctx context.Context, projectID, id string) (io.ReadCloser, domain.Package, error) {
	pkg, err := s.GetPackage(ctx, projectID, id)
	if err != nil {
		return nil, domain.Package{}, err
	}
	reader, err := s.packages.Open(pkg.ZIPBlobKey)
	return reader, pkg, err
}
func (s *Delivery) PreviewFile(ctx context.Context, projectID, packageID, fileID string) (string, bool, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return "", false, err
	}
	file, err := s.repo.GetPackageFile(ctx, ownerID, projectID, packageID, fileID)
	if err != nil {
		return "", false, err
	}
	if !file.Previewable {
		return "", false, &InvalidInput{Err: errors.New("file is not previewable")}
	}
	reader, err := s.packages.Open(file.BlobKey)
	if err != nil {
		return "", false, err
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, s.previewLimit+1))
	if err != nil {
		return "", false, err
	}
	truncated := int64(len(body)) > s.previewLimit
	if truncated {
		body = body[:s.previewLimit]
	}
	return string(body), truncated, nil
}
func newID(prefix string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value[:]), nil
}
