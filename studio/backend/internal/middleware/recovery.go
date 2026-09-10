package middleware

import (
	"log/slog"
	"net/http"
	"studio-backend/internal/httpx"
)

func Recovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				logger.Error("recovered request panic",
					"request_id", RequestIDFromContext(r.Context()),
				)
				httpx.WriteError(w, http.StatusInternalServerError, httpx.ErrorBody{
					Code:      "INTERNAL_ERROR",
					Message:   "요청을 처리하지 못했습니다.",
					RequestID: RequestIDFromContext(r.Context()),
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
