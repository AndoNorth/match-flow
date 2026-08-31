package healthz_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/match-service/internal/healthz"
)

var _ = Describe("Handler", func() {
	It("returns 200", func() {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		healthz.Handler().ServeHTTP(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
	})
})
