package middleware

import "context"

type contextKey string

const requestIDKey contextKey = "request-id"

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
