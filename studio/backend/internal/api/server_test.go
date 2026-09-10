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
	"testing"

	"studio-backend/internal/config"
	"studio-backend/internal/service"
	"studio-backend/internal/sqlite"
)

type readinessFunc func(context.Context) error

func (f readinessFunc) PingContext(ctx context.Context) error { return f(ctx) }

func testHandler(t *testing.T, ready readinessFunc) http.Handler {
	t.Helper()
	cfg, err := config.Parse([]string{"--data-dir", t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	projects, err := service.NewProjects(sqlite.NewProjectRepository(db), "local-user", cfg.TopologyNodes, cfg.TopologyEdges)
	if err != nil {
		t.Fatal(err)
	}
	var readiness Readiness = db
	if ready != nil {
		readiness = ready
	}
	handler, err := NewHandler(Dependencies{
		Config:   cfg,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:    readiness,
		Projects: projects,
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
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

func TestHealthAndCapabilities(t *testing.T) {
	handler := testHandler(t, func(context.Context) error { return nil })

	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/healthz", want: `"status":"ok"`},
		{path: "/api/v1/capabilities", want: `"generators":[]`},
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
