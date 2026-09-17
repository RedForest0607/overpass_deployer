package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"studio-backend/internal/httpx"
	"studio-backend/internal/identity"
)

const localOwnerID = "local-user"

type AuthenticationOptions struct {
	Mode              string
	IdentityHeader    string
	ProxySecretHeader string
	ProxySecret       string
}

func Authentication(options AuthenticationOptions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		ownerID := localOwnerID
		if options.Mode == "trusted-header" {
			if !secretsEqual(r.Header.Get(options.ProxySecretHeader), options.ProxySecret) {
				writeAuthenticationError(w, r)
				return
			}
			ownerID = strings.TrimSpace(r.Header.Get(options.IdentityHeader))
			if !validOwnerID(ownerID) {
				writeAuthenticationError(w, r)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(identity.WithOwner(r.Context(), ownerID)))
	})
}

func secretsEqual(provided, expected string) bool {
	providedHash := sha256.Sum256([]byte(provided))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1 && provided != ""
}

func validOwnerID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("._@:-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func writeAuthenticationError(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrorBody{
		Code:      "AUTHENTICATION_REQUIRED",
		Message:   "인증된 사용자 정보가 필요합니다.",
		RequestID: RequestIDFromContext(r.Context()),
	})
}
