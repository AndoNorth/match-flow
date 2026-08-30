package healthz

import "net/http"

// Handler proves the Makefile/Air run-loop independently of the
// generator - this is the one externally-observable API surface this
// service has.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
