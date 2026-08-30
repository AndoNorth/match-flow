package simulator_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator"
)

var _ = Describe("HTTPSubmitter", func() {
	It("POSTs the payload to baseURL+route", func() {
		var gotPath string
		var gotBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		submitter := simulator.NewHTTPSubmitter(server.URL)
		err := submitter.Submit(context.Background(), "/events/provider-a", []byte(`{"hello":"world"}`))

		Expect(err).NotTo(HaveOccurred())
		Expect(gotPath).To(Equal("/events/provider-a"))
		Expect(gotBody).To(MatchJSON(`{"hello":"world"}`))
	})

	It("returns an error when the server responds with a non-2xx status", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		submitter := simulator.NewHTTPSubmitter(server.URL)
		err := submitter.Submit(context.Background(), "/events/provider-a", []byte(`{}`))

		Expect(err).To(HaveOccurred())
	})
})
