package domain

import (
	"encoding/json"
	"time"
)

type ArtifactKind string

const (
	ArtifactJAR          ArtifactKind = "jar"
	ArtifactConfig       ArtifactKind = "config-template"
	ArtifactEnvExample   ArtifactKind = "env-example"
	ArtifactFile         ArtifactKind = "file"
	ArtifactArchive      ArtifactKind = "archive"
	ArtifactScriptValues ArtifactKind = "script-values"
)

func ParseArtifactKind(value string) (ArtifactKind, bool) {
	kind := ArtifactKind(value)
	switch kind {
	case ArtifactJAR, ArtifactConfig, ArtifactEnvExample, ArtifactFile, ArtifactArchive, ArtifactScriptValues:
		return kind, true
	default:
		return "", false
	}
}

type Artifact struct {
	ID           string       `json:"id"`
	ProjectID    string       `json:"projectId"`
	Kind         ArtifactKind `json:"kind"`
	OriginalName string       `json:"originalName"`
	BlobKey      string       `json:"-"`
	SizeBytes    int64        `json:"sizeBytes"`
	SHA256       string       `json:"sha256"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type Generation struct {
	ID                  string          `json:"id"`
	ProjectID           string          `json:"projectId"`
	Environment         Environment     `json:"environment"`
	Revision            int64           `json:"revision"`
	Status              string          `json:"status"`
	Generator           string          `json:"generator"`
	Output              string          `json:"output"`
	Attempts            int             `json:"attempts"`
	PackageID           *string         `json:"packageId"`
	RetryOfGenerationID *string         `json:"retryOfGenerationId"`
	Error               *Failure        `json:"error"`
	Snapshot            json.RawMessage `json:"-"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PackageFile struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	BlobKey     string `json:"-"`
	MediaType   string `json:"mediaType"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	Previewable bool   `json:"previewable"`
}

type Package struct {
	ID               string        `json:"id"`
	ProjectID        string        `json:"projectId"`
	Environment      Environment   `json:"environment"`
	Revision         int64         `json:"revision"`
	Output           string        `json:"output"`
	Generator        string        `json:"generator"`
	GeneratorVersion string        `json:"generatorVersion"`
	ChartFormat      string        `json:"chartFormat"`
	ZIPBlobKey       string        `json:"-"`
	CreatedAt        time.Time     `json:"createdAt"`
	Files            []PackageFile `json:"files"`
}
