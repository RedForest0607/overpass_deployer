package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"studio-backend/internal/identity"
)

func TestRequestIDIsGeneratedAndAddedToContext(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, RequestIDFromContext(r.Context()))
	}))
	request := httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	id := recorder.Header().Get("X-Request-ID")
	if !strings.HasPrefix(id, "req-") || recorder.Body.String() != id {
		t.Fatalf("request id header=%q body=%q", id, recorder.Body.String())
	}
}

func TestSecurityRejectsUnknownHostAndOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequestID(Security([]string{"localhost"}, []string{"http://localhost:3000"}, next))

	tests := []struct {
		name   string
		host   string
		origin string
	}{
		{name: "host", host: "attacker.example"},
		{name: "origin", host: "localhost:8080", origin: "https://attacker.example"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil)
			request.Host = test.host
			request.Header.Set("Origin", test.origin)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "requestId") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSecurityAllowsConfiguredPreflight(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	handler := Security([]string{"localhost"}, []string{"http://localhost:3000"}, next)
	request := httptest.NewRequest(http.MethodOptions, "http://localhost:8080/api/v1/capabilities", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "GET")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if nextCalled || recorder.Code != http.StatusNoContent {
		t.Fatalf("nextCalled=%t status=%d", nextCalled, recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("allow origin=%q", got)
	}
}

func TestBodyLimitStopsOversizedRead(t *testing.T) {
	handler := BodyLimit(3, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected body read error")
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	request := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewBufferString("large"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestRecoveryReturnsOpaqueError(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := RequestID(Recovery(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive detail")
	})))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost", nil))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "sensitive") || strings.Contains(logs.String(), "sensitive") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAccessLogOmitsQueryHeadersAndBody(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := RequestID(AccessLog(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/projects?token=query-secret", strings.NewReader("body-secret"))
	request.Header.Set("Authorization", "Bearer header-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	for _, secret := range []string{"query-secret", "body-secret", "header-secret"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("access log exposed %q: %s", secret, logs.String())
		}
	}
	if !strings.Contains(logs.String(), `"path":"/api/v1/projects"`) {
		t.Fatalf("access log has no safe path: %s", logs.String())
	}
}

func TestAuthenticationModes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := identity.OwnerID(r.Context())
		if !ok {
			t.Fatal("owner was not added to context")
		}
		_, _ = io.WriteString(w, ownerID)
	})

	local := Authentication(AuthenticationOptions{Mode: "local"}, next)
	localRecorder := httptest.NewRecorder()
	local.ServeHTTP(localRecorder, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects", nil))
	if localRecorder.Code != http.StatusOK || localRecorder.Body.String() != "local-user" {
		t.Fatalf("local status=%d body=%q", localRecorder.Code, localRecorder.Body.String())
	}

	options := AuthenticationOptions{Mode: "trusted-header", IdentityHeader: "X-Verified-User", ProxySecretHeader: "X-Proxy-Secret", ProxySecret: strings.Repeat("s", 32)}
	trusted := RequestID(Authentication(options, next))
	for _, test := range []struct {
		name, user, secret string
		status             int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong secret", user: "alice", secret: "wrong", status: http.StatusUnauthorized},
		{name: "invalid user", user: "../alice", secret: options.ProxySecret, status: http.StatusUnauthorized},
		{name: "valid", user: "alice@example.com", secret: options.ProxySecret, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/projects", nil)
			request.Header.Set(options.IdentityHeader, test.user)
			request.Header.Set(options.ProxySecretHeader, test.secret)
			recorder := httptest.NewRecorder()
			trusted.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAuthenticationLeavesHealthCheckPublic(t *testing.T) {
	handler := Authentication(AuthenticationOptions{Mode: "trusted-header"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestCSRFRejectsBrowserCrossSiteMutations(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequestID(Security([]string{"localhost"}, []string{"https://studio.example.com"}, CSRF(next)))

	tests := []struct {
		name, fetchSite, origin string
		status                  int
	}{
		{name: "cross site", fetchSite: "cross-site", origin: "https://studio.example.com", status: http.StatusForbidden},
		{name: "browser origin missing", fetchSite: "same-origin", status: http.StatusForbidden},
		{name: "same origin", fetchSite: "same-origin", origin: "https://studio.example.com", status: http.StatusNoContent},
		{name: "non browser client", status: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/projects", nil)
			request.Header.Set("Sec-Fetch-Site", test.fetchSite)
			request.Header.Set("Origin", test.origin)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
