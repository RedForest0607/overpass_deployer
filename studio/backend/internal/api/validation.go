package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"studio-backend/internal/domain"
	"studio-backend/internal/httpx"
	"studio-backend/internal/repository"
)

func (h projectHandlers) validate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Environment     string          `json:"environment"`
		ClientRequestID string          `json:"clientRequestId"`
		Blueprint       json.RawMessage `json:"blueprint"`
		Purpose         string          `json:"purpose"`
		Generator       string          `json:"generator,omitempty"`
	}
	if !decodeJSONRequest(w, r, &request) {
		return
	}
	if _, err := domain.ParseEnvironment(request.Environment); err != nil || request.ClientRequestID == "" || len(request.ClientRequestID) > 128 || (request.Purpose != "design" && request.Purpose != "package") {
		writeRequestError(w, r, 400, "INVALID_REQUEST", "환경, 요청 ID, 검증 목적을 확인하세요.", nil)
		return
	}
	// Resolve project ownership before returning any validation results.
	result, err := h.service.Validate(r.Context(), r.PathValue("projectId"), request.Blueprint, request.ClientRequestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeServiceError(w, r, err, "NOT_FOUND")
			return
		}
		writeServiceError(w, r, err, "INVALID_REQUEST")
		return
	}
	if request.Purpose == "package" || request.Generator != "" {
		writeRequestError(w, r, 422, "UNSUPPORTED_GENERATOR", "현재 서버는 설계 검증만 지원합니다. 패키지 생성 검증은 후속 단계입니다.", nil)
		return
	}
	httpx.WriteJSON(w, 200, result)
}
