package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"studio-backend/internal/config"
	"studio-backend/internal/generator/crosswalkvm"
	"studio-backend/internal/httpx"
	"studio-backend/internal/middleware"
	"studio-backend/internal/service"
)

type Readiness interface {
	PingContext(context.Context) error
}

type Dependencies struct {
	Config   config.Config
	Logger   *slog.Logger
	Ready    Readiness
	Projects *service.Projects
	Delivery *service.Delivery
}

func NewHandler(deps Dependencies) (http.Handler, error) {
	if deps.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if deps.Ready == nil {
		return nil, errors.New("readiness dependency is required")
	}
	if deps.Projects == nil {
		return nil, errors.New("project service is required")
	}
	if deps.Delivery == nil {
		return nil, errors.New("delivery service is required")
	}

	mux := http.NewServeMux()
	projects := projectHandlers{service: deps.Projects, delivery: deps.Delivery}
	delivery := deliveryHandlers{service: deps.Delivery}
	mux.HandleFunc("GET /healthz", healthHandler(deps.Ready))
	mux.HandleFunc("GET /api/v1/capabilities", capabilitiesHandler(deps.Config))
	mux.HandleFunc("GET /api/v1/projects", projects.list)
	mux.HandleFunc("POST /api/v1/projects", projects.create)
	mux.HandleFunc("GET /api/v1/projects/{projectId}", projects.get)
	mux.HandleFunc("PATCH /api/v1/projects/{projectId}", projects.update)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/topologies/{environment}", projects.getTopology)
	mux.HandleFunc("PUT /api/v1/projects/{projectId}/topologies/{environment}", projects.saveTopology)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/validations", projects.validate)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/artifacts", delivery.listArtifacts)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/artifacts", delivery.createArtifact)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/artifacts/{artifactId}/content", delivery.getArtifactContent)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/generations", delivery.createGeneration)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/generations", delivery.listGenerations)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/generations/{generationId}", delivery.getGeneration)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/generations/{generationId}/retry", delivery.retryGeneration)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/packages/{packageId}", delivery.getPackage)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/packages/{packageId}/files/{fileId}", delivery.previewFile)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/packages/{packageId}/download", delivery.downloadPackage)
	mux.HandleFunc("/", notFoundHandler)

	var handler http.Handler = mux
	handler = middleware.BodyLimit(deps.Config.MaxRequestBytes, handler)
	handler = middleware.CSRF(handler)
	handler = middleware.Authentication(middleware.AuthenticationOptions{
		Mode:              deps.Config.AuthMode,
		IdentityHeader:    deps.Config.AuthHeader,
		ProxySecretHeader: deps.Config.ProxySecretHeader,
		ProxySecret:       deps.Config.ProxySecret,
	}, handler)
	handler = middleware.Security(deps.Config.AllowedHosts, deps.Config.AllowedOrigins, handler)
	handler = middleware.Recovery(deps.Logger, handler)
	handler = middleware.AccessLog(deps.Logger, handler)
	handler = middleware.RequestID(handler)
	return handler, nil
}

func healthHandler(readiness Readiness) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := readiness.PingContext(r.Context()); err != nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, httpx.ErrorBody{
				Code:      "SERVICE_UNAVAILABLE",
				Message:   "저장소를 사용할 수 없습니다.",
				RequestID: middleware.RequestIDFromContext(r.Context()),
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, Health{Status: "ok"})
	}
}

func capabilitiesHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, Capabilities{
			BlueprintVersions: []string{"studio.deployer/v1"},
			Generators:        []GeneratorCapability{{ID: crosswalkvm.ID, Outputs: []string{"draft", "package"}, ChartFormats: []string{crosswalkvm.ChartFormat}, SupportedKinds: []string{"vm", "java", "redis", "elasticsearch"}, SupportedModes: []string{"process"}}},
			Limits: Limits{
				JARBytes:            cfg.JARBytes,
				TextArtifactBytes:   cfg.TextArtifactBytes,
				PreviewBytes:        cfg.PreviewBytes,
				TopologyNodes:       cfg.TopologyNodes,
				TopologyConnections: cfg.TopologyEdges,
			},
		})
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusNotFound, httpx.ErrorBody{
		Code:      "NOT_FOUND",
		Message:   "요청한 API를 찾을 수 없습니다.",
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}
