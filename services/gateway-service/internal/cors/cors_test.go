package cors_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/cors"
)

var _ = Describe("Middleware", func() {
	It("sets Access-Control-Allow-Origin to the configured origin and still calls through to next", func() {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		handler := cors.Middleware("http://localhost:3000", next)
		req := httptest.NewRequest(http.MethodGet, "/matches", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		Expect(called).To(BeTrue())
		Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://localhost:3000"))
		Expect(rec.Code).To(Equal(http.StatusOK))
	})
})
