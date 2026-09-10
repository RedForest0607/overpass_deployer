package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := RequestID(Recovery(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive detail")
	})))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost", nil))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "sensitive") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
