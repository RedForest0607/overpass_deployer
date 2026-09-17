package middleware

import (
	"net/http"
	"strings"
)

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if fetchSite == "cross-site" || (fetchSite != "" && origin == "") {
			writeSecurityError(w, r, "CSRF_REJECTED", "교차 사이트 변경 요청을 허용하지 않습니다.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
