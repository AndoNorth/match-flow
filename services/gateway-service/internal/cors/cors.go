// Package cors lets the Gateway's REST and SSE endpoints be called from a
// browser running on a different origin - the frontend, per
// docs/ARCHITECTURE.md, is the only client the Gateway serves.
package cors

import "net/http"

// Middleware sets Access-Control-Allow-Origin to allowedOrigin on every
// response before delegating to next. A single configured origin, not a
// wildcard, since the Gateway has exactly one intended caller.
func Middleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		next.ServeHTTP(w, r)
	})
}
