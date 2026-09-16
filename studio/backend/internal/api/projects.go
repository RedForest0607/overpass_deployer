package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"studio-backend/internal/domain"
	"studio-backend/internal/httpx"
	"studio-backend/internal/middleware"
	"studio-backend/internal/repository"
	"studio-backend/internal/service"
)

type projectHandlers struct {
	service  *service.Projects
	delivery *service.Delivery
}

type createProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type updateProjectRequest struct {
	ExpectedRevision int64   `json:"expectedRevision"`
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
}

type saveTopologyRequest struct {
	ExpectedRevision int64           `json:"expectedRevision"`
	Blueprint        json.RawMessage `json:"blueprint"`
	Layout           json.RawMessage `json:"layout"`
}

func (h projectHandlers) create(w http.ResponseWriter, r *http.Request) {
	var request createProjectRequest
	if !decodeJSONRequest(w, r, &request) {
		return
	}
	if request.Name == nil || request.Description == nil {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "name과 description은 필수입니다.", nil)
		return
	}
	project, err := h.service.Create(r.Context(), *request.Name, *request.Description)
	if err != nil {
		writeServiceError(w, r, err, "INVALID_REQUEST")
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+project.ID)
	httpx.WriteJSON(w, http.StatusCreated, project)
}

func (h projectHandlers) list(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "limit은 1부터 100 사이여야 합니다.", nil)
			return
		}
		limit = parsed
	}
	page, err := h.service.List(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeServiceError(w, r, err, "INVALID_REQUEST")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": page.Items, "nextCursor": page.NextCursor})
}

func (h projectHandlers) get(w http.ResponseWriter, r *http.Request) {
	project, err := h.service.Get(r.Context(), r.PathValue("projectId"))
	if err != nil {
		writeServiceError(w, r, err, "NOT_FOUND")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, project)
}

func (h projectHandlers) update(w http.ResponseWriter, r *http.Request) {
	var request updateProjectRequest
	if !decodeJSONRequest(w, r, &request) {
		return
	}
	if request.ExpectedRevision < 1 {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "expectedRevision은 1 이상이어야 합니다.", nil)
		return
	}
	project, err := h.service.Update(r.Context(), r.PathValue("projectId"), request.ExpectedRevision, repository.ProjectChanges{Name: request.Name, Description: request.Description})
	if err != nil {
		writeServiceError(w, r, err, "INVALID_REQUEST")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, project)
}

func (h projectHandlers) getTopology(w http.ResponseWriter, r *http.Request) {
	environment, err := domain.ParseEnvironment(r.PathValue("environment"))
	if err != nil {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "지원하지 않는 환경입니다.", nil)
		return
	}
	topology, err := h.service.GetTopology(r.Context(), r.PathValue("projectId"), environment)
	if err != nil {
		writeServiceError(w, r, err, "TOPOLOGY_NOT_FOUND")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, topology)
}

func (h projectHandlers) saveTopology(w http.ResponseWriter, r *http.Request) {
	environment, err := domain.ParseEnvironment(r.PathValue("environment"))
	if err != nil {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "지원하지 않는 환경입니다.", nil)
		return
	}
	var request saveTopologyRequest
	if !decodeJSONRequest(w, r, &request) {
		return
	}
	if request.ExpectedRevision < 0 {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "expectedRevision은 0 이상이어야 합니다.", nil)
		return
	}
	topology, err := h.service.SaveTopology(r.Context(), r.PathValue("projectId"), environment, request.ExpectedRevision, request.Blueprint, request.Layout)
	if err != nil {
		writeServiceError(w, r, err, "INVALID_REQUEST")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, topology)
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeRequestError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type은 application/json이어야 합니다.", nil)
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeRequestError(w, r, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "요청 크기 제한을 초과했습니다.", nil)
		} else {
			writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "JSON 요청 형식이 올바르지 않습니다.", nil)
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "JSON 값은 하나만 전송할 수 있습니다.", nil)
		return false
	}
	return true
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error, fallbackCode string) {
	if errors.Is(err, repository.ErrNotFound) {
		writeRequestError(w, r, http.StatusNotFound, fallbackCode, "요청한 대상을 찾을 수 없습니다.", nil)
		return
	}
	if errors.Is(err, repository.ErrInvalidCursor) {
		writeRequestError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "cursor가 올바르지 않습니다.", nil)
		return
	}
	var conflict *repository.RevisionConflict
	if errors.As(err, &conflict) {
		writeRequestError(w, r, http.StatusConflict, "REVISION_CONFLICT", "서버에 더 최신 데이터가 있습니다.", map[string]any{"expectedRevision": conflict.Expected, "actualRevision": conflict.Actual})
		return
	}
	var invalid *service.InvalidInput
	if errors.As(err, &invalid) {
		writeRequestError(w, r, http.StatusBadRequest, fallbackCode, invalid.Error(), nil)
		return
	}
	writeRequestError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "요청을 처리하지 못했습니다.", nil)
}

func writeRequestError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	httpx.WriteError(w, status, httpx.ErrorBody{Code: code, Message: message, RequestID: middleware.RequestIDFromContext(r.Context()), Details: details})
}
