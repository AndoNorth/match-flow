package healthz

import "net/http"

// Handler returns 200 while the service is running, independent of
// Redis, Postgres, or route-handling logic - proves the process itself
// is alive.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
