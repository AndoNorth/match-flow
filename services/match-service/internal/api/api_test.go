// services/match-service/internal/api/api_test.go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/AndoNorth/match-flow/services/match-service/internal/api"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

type fakeStore struct {
	matches []matchstate.MatchRecord
	events  map[string][]matchstate.EventRecord
}

func (f *fakeStore) ListMatches(_ context.Context, status string) ([]matchstate.MatchRecord, error) {
	if status == "" {
		return f.matches, nil
	}
	var out []matchstate.MatchRecord
	for _, m := range f.matches {
		if m.Status == status {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStore) GetMatch(_ context.Context, matchID string) (matchstate.MatchRecord, error) {
	for _, m := range f.matches {
		if m.MatchID == matchID {
			return m, nil
		}
	}
	return matchstate.MatchRecord{}, matchstate.ErrNotFound
}

func (f *fakeStore) ListEvents(_ context.Context, matchID string) ([]matchstate.EventRecord, error) {
	if _, err := f.GetMatch(context.Background(), matchID); err != nil {
		return nil, err
	}
	return f.events[matchID], nil
}

func newTestServer(store *fakeStore) *httptest.Server {
	mux := http.NewServeMux()
	humaAPI := humago.New(mux, huma.DefaultConfig("test", "0.0.0"))
	api.Register(humaAPI, store)
	return httptest.NewServer(mux)
}

var _ = Describe("Register", func() {
	var store *fakeStore

	BeforeEach(func() {
		store = &fakeStore{
			matches: []matchstate.MatchRecord{
				{MatchID: "match-1", Sport: "football", Status: "live", HomeScore: 1},
			},
			events: map[string][]matchstate.EventRecord{
				"match-1": {{Type: "kickoff", Sequence: 1}, {Type: "goal", Sequence: 2}},
			},
		}
	})

	It("GET /matches lists every match", func() {
		server := newTestServer(store)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("GET /matches/{id} returns the match", func() {
		server := newTestServer(store)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/match-1")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("GET /matches/{id} returns 404 for an unknown match", func() {
		server := newTestServer(store)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/no-such-match")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("GET /matches/{id}/events returns the timeline", func() {
		server := newTestServer(store)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/match-1/events")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("GET /matches/{id}/events returns 404 for an unknown match", func() {
		server := newTestServer(store)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/no-such-match/events")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})
})
