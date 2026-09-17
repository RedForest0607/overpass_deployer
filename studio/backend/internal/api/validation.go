package api

import (
	"errors"
	"net/http"
	"studio-backend/internal/domain"
	"studio-backend/internal/generator/crosswalkvm"
	"studio-backend/internal/httpx"
	"studio-backend/internal/repository"
)

func (h projectHandlers) validate(w http.ResponseWriter, r *http.Request) {
	var request ValidationRequest
	if !decodeJSONRequest(w, r, &request) {
		return
	}
	if request.Environment == nil || request.ClientRequestID == nil || request.Blueprint == nil || request.Purpose == nil {
		writeRequestError(w, r, 400, "INVALID_REQUEST", "환경, 요청 ID, 검증 목적을 확인하세요.", nil)
		return
	}
	environment := string(*request.Environment)
	purpose := *request.Purpose
	if _, err := domain.ParseEnvironment(environment); err != nil || *request.ClientRequestID == "" || len(*request.ClientRequestID) > 128 || (purpose != "design" && purpose != "package") {
		writeRequestError(w, r, 400, "INVALID_REQUEST", "환경, 요청 ID, 검증 목적을 확인하세요.", nil)
		return
	}
	// Resolve project ownership before returning any validation results.
	result, err := h.service.Validate(r.Context(), r.PathValue("projectId"), *request.Blueprint, *request.ClientRequestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeServiceError(w, r, err, "NOT_FOUND")
			return
		}
		writeServiceError(w, r, err, "INVALID_REQUEST")
		return
	}
	if request.Generator != nil && *request.Generator != "" && *request.Generator != crosswalkvm.ID {
		writeRequestError(w, r, 422, "UNSUPPORTED_GENERATOR", "지원하지 않는 generator입니다.", nil)
		return
	}
	if purpose == "package" {
		result = h.delivery.ValidatePackage(r.Context(), r.PathValue("projectId"), *request.Blueprint, environment, result)
	}
	httpx.WriteJSON(w, 200, result)
}
