package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidationRoute(t *testing.T) {
	handler := testHandler(t, nil)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, jsonRequest("POST", "/api/v1/projects", `{"name":"design","description":""}`))
	target := created.Header().Get("Location") + "/validations"
	body := `{"environment":"dev","clientRequestId":"edit-12","purpose":"design","blueprint":{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[]}}`
	for _, tc := range []struct {
		path, body string
		status     int
		want       string
	}{
		{target, body, 200, "EMPTY_TOPOLOGY"},
		{"/api/v1/projects/missing/validations", body, 404, "NOT_FOUND"},
		{target, strings.Replace(body, `"design"`, `"package"`, 1), 422, "UNSUPPORTED_GENERATOR"},
		{target, strings.Replace(body, `"dev"`, `"invalid"`, 1), 400, "INVALID_REQUEST"},
		{target, strings.Replace(body, `"nodes":[]`, `"nodes":null`, 1), 400, "INVALID_REQUEST"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, jsonRequest(http.MethodPost, tc.path, tc.body))
		if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), tc.want) {
			t.Fatalf("%d: %s", recorder.Code, recorder.Body.String())
		}
	}
}
