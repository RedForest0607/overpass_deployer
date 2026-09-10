package middleware

import (
	"net"
	"net/http"
	"slices"
	"strings"

	"studio-backend/internal/httpx"
)

func Security(allowedHosts, allowedOrigins []string, next http.Handler) http.Handler {
	hosts := normalizeValues(allowedHosts)
	origins := normalizeValues(allowedOrigins)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !slices.Contains(hosts, requestHost(r.Host)) {
			writeSecurityError(w, r, "HOST_NOT_ALLOWED", "요청 Host를 허용하지 않습니다.")
			return
		}
		origin := strings.ToLower(strings.TrimSpace(r.Header.Get("Origin")))
		if origin != "" && !slices.Contains(origins, origin) {
			writeSecurityError(w, r, "ORIGIN_NOT_ALLOWED", "요청 Origin을 허용하지 않습니다.")
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
			w.Header().Add("Vary", "Origin")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requestHost(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.ToLower(strings.Trim(host, "[]"))
	}
	return strings.ToLower(strings.Trim(value, "[]"))
}

func normalizeValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(strings.Trim(value, "[]")))
		if value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func writeSecurityError(w http.ResponseWriter, r *http.Request, code, message string) {
	httpx.WriteError(w, http.StatusForbidden, httpx.ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: RequestIDFromContext(r.Context()),
	})
}
