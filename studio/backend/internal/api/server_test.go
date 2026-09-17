package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio-backend/internal/config"
	"studio-backend/internal/localstore"
	"studio-backend/internal/service"
	"studio-backend/internal/sqlite"
	"studio-backend/internal/storage"
)

type readinessFunc func(context.Context) error

func (f readinessFunc) PingContext(ctx context.Context) error { return f(ctx) }

func testHandler(t *testing.T, ready readinessFunc) http.Handler {
	t.Helper()
	dataDir := t.TempDir()
	cfg, err := config.Parse([]string{"--data-dir", dataDir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return testHandlerWithConfig(t, ready, cfg)
}

func testHandlerWithConfig(t *testing.T, ready readinessFunc, cfg config.Config) http.Handler {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	projects, err := service.NewProjects(sqlite.NewProjectRepository(db), cfg.TopologyNodes, cfg.TopologyEdges)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := storage.Prepare(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := localstore.New(paths.Artifacts, paths.Temporary)
	if err != nil {
		t.Fatal(err)
	}
	packageStore, err := localstore.New(paths.Packages, paths.Temporary)
	if err != nil {
		t.Fatal(err)
	}
	deliveryRepo := sqlite.NewDeliveryRepository(db)
	delivery, err := service.NewDelivery(projects, deliveryRepo, artifactStore, packageStore, cfg.JARBytes, cfg.TextArtifactBytes, cfg.PreviewBytes)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := service.NewGenerationWorker(delivery, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	workerContext, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan error, 1)
	go func() { workerDone <- worker.Run(workerContext) }()
	t.Cleanup(func() {
		stopWorker()
		if err := <-workerDone; err != nil {
			t.Errorf("stop generation worker: %v", err)
		}
	})
	var readiness Readiness = db
	if ready != nil {
		readiness = ready
	}
	handler, err := NewHandler(Dependencies{
		Config:   cfg,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:    readiness,
		Projects: projects,
		Delivery: delivery,
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestTrustedHeaderAuthenticationIsolatesProjectOwners(t *testing.T) {
	secret := strings.Repeat("s", 32)
	values := map[string]string{
		"STUDIO_DATA_DIR":          t.TempDir(),
		"STUDIO_AUTH_MODE":         "trusted-header",
		"STUDIO_AUTH_PROXY_SECRET": secret,
	}
	cfg, err := config.Parse(nil, func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := testHandlerWithConfig(t, nil, cfg)
	authenticated := func(request *http.Request, owner, providedSecret string) *http.Request {
		request.Header.Set(cfg.AuthHeader, owner)
		request.Header.Set(cfg.ProxySecretHeader, providedSecret)
		return request
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects", nil))
	if missing.Code != http.StatusUnauthorized || !strings.Contains(missing.Body.String(), `"code":"AUTHENTICATION_REQUIRED"`) {
		t.Fatalf("missing auth status=%d body=%s", missing.Code, missing.Body.String())
	}

	created := httptest.NewRecorder()
	handler.ServeHTTP(created, authenticated(jsonRequest(http.MethodPost, "/api/v1/projects", `{"name":"Alice project","description":""}`), "alice@example.com", secret))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	location := created.Header().Get("Location")
	projectID := strings.TrimPrefix(location, "/api/v1/projects/")

	aliceList := httptest.NewRecorder()
	handler.ServeHTTP(aliceList, authenticated(httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects", nil), "alice@example.com", secret))
	if aliceList.Code != http.StatusOK || !strings.Contains(aliceList.Body.String(), projectID) {
		t.Fatalf("alice list status=%d body=%s", aliceList.Code, aliceList.Body.String())
	}

	bobList := httptest.NewRecorder()
	handler.ServeHTTP(bobList, authenticated(httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects", nil), "bob@example.com", secret))
	if bobList.Code != http.StatusOK || strings.Contains(bobList.Body.String(), projectID) {
		t.Fatalf("bob list status=%d body=%s", bobList.Code, bobList.Body.String())
	}

	bobGet := httptest.NewRecorder()
	handler.ServeHTTP(bobGet, authenticated(httptest.NewRequest(http.MethodGet, "http://localhost"+location, nil), "bob@example.com", secret))
	if bobGet.Code != http.StatusNotFound {
		t.Fatalf("bob get status=%d body=%s", bobGet.Code, bobGet.Body.String())
	}

	wrongSecret := httptest.NewRecorder()
	handler.ServeHTTP(wrongSecret, authenticated(httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects", nil), "alice@example.com", "wrong"))
	if wrongSecret.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret status=%d body=%s", wrongSecret.Code, wrongSecret.Body.String())
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", health.Code, health.Body.String())
	}
}

func jsonRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, "http://localhost"+target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestProjectLifecycleAndRevisionConflict(t *testing.T) {
	handler := testHandler(t, nil)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, jsonRequest(http.MethodPost, "/api/v1/projects", `{"name":" Commerce ","description":"backend"}`))
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"name":"Commerce"`) {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	location := created.Header().Get("Location")
	projectID := strings.TrimPrefix(location, "/api/v1/projects/")
	if projectID == location || projectID == "" {
		t.Fatalf("invalid location %q", location)
	}

	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects?limit=1", nil))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), projectID) || !strings.Contains(listed.Body.String(), `"nextCursor":null`) {
		t.Fatalf("list status=%d body=%s", listed.Code, listed.Body.String())
	}

	updated := httptest.NewRecorder()
	handler.ServeHTTP(updated, jsonRequest(http.MethodPatch, location, `{"expectedRevision":1,"description":"updated"}`))
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"revision":2`) {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}

	conflict := httptest.NewRecorder()
	handler.ServeHTTP(conflict, jsonRequest(http.MethodPatch, location, `{"expectedRevision":1,"description":"stale"}`))
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"actualRevision":2`) {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func TestTopologyLifecycleIsEnvironmentScoped(t *testing.T) {
	handler := testHandler(t, nil)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, jsonRequest(http.MethodPost, "/api/v1/projects", `{"name":"project","description":""}`))
	projectID := strings.TrimPrefix(created.Header().Get("Location"), "/api/v1/projects/")
	body := `{"expectedRevision":0,"blueprint":{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[]},"layout":{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}}`
	target := "/api/v1/projects/" + projectID + "/topologies/dev"
	saved := httptest.NewRecorder()
	handler.ServeHTTP(saved, jsonRequest(http.MethodPut, target, body))
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), `"revision":1`) {
		t.Fatalf("save status=%d body=%s", saved.Code, saved.Body.String())
	}

	loaded := httptest.NewRecorder()
	handler.ServeHTTP(loaded, httptest.NewRequest(http.MethodGet, "http://localhost"+target, nil))
	if loaded.Code != http.StatusOK || !strings.Contains(loaded.Body.String(), `"environment":"dev"`) {
		t.Fatalf("get status=%d body=%s", loaded.Code, loaded.Body.String())
	}

	qa := httptest.NewRecorder()
	handler.ServeHTTP(qa, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects/"+projectID+"/topologies/qa", nil))
	if qa.Code != http.StatusNotFound || !strings.Contains(qa.Body.String(), `"code":"TOPOLOGY_NOT_FOUND"`) {
		t.Fatalf("qa status=%d body=%s", qa.Code, qa.Body.String())
	}

	stale := httptest.NewRecorder()
	handler.ServeHTTP(stale, jsonRequest(http.MethodPut, target, body))
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"actualRevision":1`) {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}
}

func TestTopologyRejectsUIOnlyFields(t *testing.T) {
	handler := testHandler(t, nil)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, jsonRequest(http.MethodPost, "/api/v1/projects", `{"name":"project","description":""}`))
	projectID := strings.TrimPrefix(created.Header().Get("Location"), "/api/v1/projects/")
	body := `{"expectedRevision":0,"blueprint":{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[],"selected":true},"layout":{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, jsonRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/topologies/dev", body))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"INVALID_REQUEST"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGeneratedRequestDTOsPreserveRequiredFieldPresence(t *testing.T) {
	handler := testHandler(t, nil)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create project without description",
			method: http.MethodPost,
			path:   "/api/v1/projects",
			body:   `{"name":"project"}`,
		},
		{
			name:   "save topology without layout",
			method: http.MethodPut,
			path:   "/api/v1/projects/missing/topologies/dev",
			body:   `{"expectedRevision":0,"blueprint":{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[]}}`,
		},
		{
			name:   "create generation without output",
			method: http.MethodPost,
			path:   "/api/v1/projects/missing/generations",
			body:   `{"environment":"dev","revision":1,"generator":"crosswalk-vm"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, jsonRequest(test.method, test.path, test.body))
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"INVALID_REQUEST"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestGeneratedLayoutDTOKeepsNestedMissingFieldsVisible(t *testing.T) {
	handler := testHandler(t, nil)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, jsonRequest(http.MethodPost, "/api/v1/projects", `{"name":"project","description":""}`))
	projectID := strings.TrimPrefix(created.Header().Get("Location"), "/api/v1/projects/")
	body := `{"expectedRevision":0,"blueprint":{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[]},"layout":{"nodes":[],"viewport":{"x":0,"y":0}}}`

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, jsonRequest(http.MethodPut, "/api/v1/projects/"+projectID+"/topologies/dev", body))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"INVALID_REQUEST"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHealthAndCapabilities(t *testing.T) {
	handler := testHandler(t, func(context.Context) error { return nil })

	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/healthz", want: `"status":"ok"`},
		{path: "/api/v1/capabilities", want: `"chartFormats":["crosswalk/v1"]`},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "http://localhost"+test.path, nil)
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
			t.Fatalf("%s status=%d body=%s", test.path, recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s has no request ID", test.path)
		}
	}
}

func TestHealthReportsStorageFailure(t *testing.T) {
	handler := testHandler(t, func(context.Context) error { return errors.New("database path") })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil))
	if recorder.Code != http.StatusServiceUnavailable || strings.Contains(recorder.Body.String(), "database path") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUnknownRouteUsesErrorContract(t *testing.T) {
	handler := testHandler(t, func(context.Context) error { return nil })
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/missing", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":"NOT_FOUND"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReadOnlyEndpointsConcurrentSmoke(t *testing.T) {
	handler := testHandler(t, nil)
	const requests = 64
	errors := make(chan string, requests)
	var wait sync.WaitGroup
	for index := 0; index < requests; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			path := "/healthz"
			if index%2 == 0 {
				path = "/api/v1/capabilities"
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("X-Request-ID") == "" {
				errors <- path
			}
		}(index)
	}
	wait.Wait()
	close(errors)
	for path := range errors {
		t.Errorf("concurrent request failed: %s", path)
	}
}
