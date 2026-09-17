// Code generated from schemas/openapi-v1.yaml by scripts/generate-api-contracts.mjs. DO NOT EDIT.
package api

import (
	"encoding/json"
)

type CreateProject struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type UpdateProject struct {
	ExpectedRevision *int64  `json:"expectedRevision"`
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
}

type SaveTopology struct {
	ExpectedRevision *int64           `json:"expectedRevision"`
	Blueprint        *json.RawMessage `json:"blueprint"`
	Layout           *json.RawMessage `json:"layout"`
}

type ValidationRequest struct {
	Environment     *string          `json:"environment"`
	ClientRequestID *string          `json:"clientRequestId"`
	Blueprint       *json.RawMessage `json:"blueprint"`
	Generator       *string          `json:"generator,omitempty"`
	Purpose         *string          `json:"purpose"`
}

type CreateGeneration struct {
	Environment *string `json:"environment"`
	Revision    *int64  `json:"revision"`
	Generator   *string `json:"generator"`
	Output      *string `json:"output"`
}
