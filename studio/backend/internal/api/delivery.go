package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"studio-backend/internal/domain"
	"studio-backend/internal/httpx"
	"studio-backend/internal/repository"
	"studio-backend/internal/service"
)

type deliveryHandlers struct{ service *service.Delivery }

func (h deliveryHandlers) createArtifact(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		writeRequestError(w, r, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type은 multipart/form-data여야 합니다.", nil)
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeRequestError(w, r, 413, "REQUEST_TOO_LARGE", "요청 크기 제한을 초과했습니다.", nil)
		} else {
			writeRequestError(w, r, 400, "INVALID_REQUEST", "multipart 요청을 읽을 수 없습니다.", nil)
		}
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	kind, ok := domain.ParseArtifactKind(r.FormValue("kind"))
	if !ok {
		writeRequestError(w, r, 400, "INVALID_REQUEST", "지원하지 않는 artifact kind입니다.", nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeRequestError(w, r, 400, "INVALID_REQUEST", "file은 필수입니다.", nil)
		return
	}
	defer file.Close()
	name := r.FormValue("displayName")
	if name == "" {
		name = header.Filename
	}
	value, err := h.service.CreateArtifact(r.Context(), r.PathValue("projectId"), kind, name, file)
	if err != nil {
		writeDeliveryError(w, r, err, "INVALID_ARTIFACT")
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+value.ProjectID+"/artifacts/"+value.ID)
	httpx.WriteJSON(w, 201, value)
}

func (h deliveryHandlers) listArtifacts(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeRequestError(w, r, 400, "INVALID_REQUEST", "limit이 올바르지 않습니다.", nil)
			return
		}
		limit = value
	}
	page, err := h.service.ListArtifacts(r.Context(), r.PathValue("projectId"), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeDeliveryError(w, r, err, "NOT_FOUND")
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"items": page.Items, "nextCursor": page.NextCursor})
}

func (h deliveryHandlers) getArtifactContent(w http.ResponseWriter, r *http.Request) {
	content, err := h.service.ReadArtifactText(r.Context(), r.PathValue("projectId"), r.PathValue("artifactId"))
	if err != nil {
		writeDeliveryError(w, r, err, "ARTIFACT_NOT_READABLE")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":       r.PathValue("artifactId"),
		"content":  content,
		"encoding": "utf-8",
	})
}

func (h deliveryHandlers) createGeneration(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var request struct {
		Environment string `json:"environment"`
		Revision    int64  `json:"revision"`
		Generator   string `json:"generator"`
		Output      string `json:"output"`
	}
	if !decodeJSONRequest(w, r, &request) {
		return
	}
	environment, err := domain.ParseEnvironment(request.Environment)
	if err != nil {
		writeRequestError(w, r, 400, "INVALID_REQUEST", "지원하지 않는 환경입니다.", nil)
		return
	}
	value, err := h.service.Generate(r.Context(), r.PathValue("projectId"), service.GenerationInput{Environment: environment, Revision: request.Revision, Generator: request.Generator, Output: request.Output, IdempotencyKey: key})
	if err != nil {
		writeDeliveryError(w, r, err, "GENERATION_FAILED")
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/projects/%s/generations/%s", value.ProjectID, value.ID))
	httpx.WriteJSON(w, 202, value)
}
func (h deliveryHandlers) getGeneration(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetGeneration(r.Context(), r.PathValue("projectId"), r.PathValue("generationId"))
	if err != nil {
		writeDeliveryError(w, r, err, "NOT_FOUND")
		return
	}
	httpx.WriteJSON(w, 200, value)
}
func (h deliveryHandlers) getPackage(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetPackage(r.Context(), r.PathValue("projectId"), r.PathValue("packageId"))
	if err != nil {
		writeDeliveryError(w, r, err, "NOT_FOUND")
		return
	}
	httpx.WriteJSON(w, 200, value)
}
func (h deliveryHandlers) previewFile(w http.ResponseWriter, r *http.Request) {
	content, truncated, err := h.service.PreviewFile(r.Context(), r.PathValue("projectId"), r.PathValue("packageId"), r.PathValue("fileId"))
	if err != nil {
		writeDeliveryError(w, r, err, "NOT_FOUND")
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"id": r.PathValue("fileId"), "content": content, "encoding": "utf-8", "truncated": truncated})
}
func (h deliveryHandlers) downloadPackage(w http.ResponseWriter, r *http.Request) {
	reader, pkg, err := h.service.OpenPackage(r.Context(), r.PathValue("projectId"), r.PathValue("packageId"))
	if err != nil {
		writeDeliveryError(w, r, err, "NOT_FOUND")
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="overpass-%s-%s.zip"`, pkg.Environment, pkg.ID))
	w.WriteHeader(200)
	_, _ = io.Copy(w, reader)
}

func writeDeliveryError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	var tooLarge *service.TooLarge
	if errors.As(err, &tooLarge) {
		writeRequestError(w, r, 413, "REQUEST_TOO_LARGE", tooLarge.Error(), map[string]any{"limit": tooLarge.Limit})
		return
	}
	var invalidArtifact *service.InvalidArtifact
	if errors.As(err, &invalidArtifact) {
		writeRequestError(w, r, 422, fallback, invalidArtifact.Error(), nil)
		return
	}
	if errors.Is(err, repository.ErrIdempotencyConflict) {
		writeRequestError(w, r, 409, "IDEMPOTENCY_CONFLICT", "같은 Idempotency-Key가 다른 요청에 사용됐습니다.", nil)
		return
	}
	writeServiceError(w, r, err, fallback)
}
