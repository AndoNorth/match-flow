package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"connectrpc.com/connect"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/api"
)

type fakeClient struct {
	matches []*matchservicev1.Match
	events  map[string][]*matchservicev1.MatchEvent
}

func (f *fakeClient) ListMatches(_ context.Context, status string) (*matchservicev1.ListMatchesResponse, error) {
	if status == "" {
		return &matchservicev1.ListMatchesResponse{Matches: f.matches}, nil
	}
	var out []*matchservicev1.Match
	for _, m := range f.matches {
		if m.GetStatus() == status {
			out = append(out, m)
		}
	}
	return &matchservicev1.ListMatchesResponse{Matches: out}, nil
}

func (f *fakeClient) GetMatch(_ context.Context, matchID string) (*matchservicev1.Match, error) {
	for _, m := range f.matches {
		if m.GetMatchId() == matchID {
			return m, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, matchNotFound(matchID))
}

func (f *fakeClient) ListMatchEvents(
	ctx context.Context, matchID string,
) (*matchservicev1.ListMatchEventsResponse, error) {
	if _, err := f.GetMatch(ctx, matchID); err != nil {
		return nil, err
	}
	return &matchservicev1.ListMatchEventsResponse{Events: f.events[matchID]}, nil
}

func matchNotFound(id string) error { return &notFoundError{id: id} }

type notFoundError struct{ id string }

func (e *notFoundError) Error() string { return "no such match: " + e.id }

func humaAPIForTest(mux *http.ServeMux) huma.API {
	return humago.New(mux, huma.DefaultConfig("test", "0.0.0"))
}

func newTestServer(client *fakeClient) *httptest.Server {
	mux := http.NewServeMux()
	api.Register(humaAPIForTest(mux), client)
	return httptest.NewServer(mux)
}

var _ = Describe("Register", func() {
	var client *fakeClient

	BeforeEach(func() {
		client = &fakeClient{
			matches: []*matchservicev1.Match{
				{MatchId: "match-1", Sport: "football", Status: "live", HomeScore: 1},
			},
			events: map[string][]*matchservicev1.MatchEvent{
				"match-1": {{Type: "kickoff", Sequence: 1}, {Type: "goal", Sequence: 2}},
			},
		}
	})

	It("GET /matches lists every match", func() {
		server := newTestServer(client)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		var parsed struct {
			Matches []map[string]any `json:"matches"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&parsed)).To(Succeed())
		Expect(parsed.Matches).To(HaveLen(1))
		Expect(parsed.Matches[0]["match_id"]).To(Equal("match-1"))
	})

	It("GET /matches?status= filters", func() {
		server := newTestServer(client)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches?status=finished")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		var parsed struct {
			Matches []map[string]any `json:"matches"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&parsed)).To(Succeed())
		Expect(parsed.Matches).To(BeEmpty())
	})

	It("GET /matches/{id} returns the match", func() {
		server := newTestServer(client)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/match-1")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("GET /matches/{id} returns 404 for an unknown match", func() {
		server := newTestServer(client)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/no-such-match")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("GET /matches/{id}/events returns the timeline", func() {
		server := newTestServer(client)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/match-1/events")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("GET /matches/{id}/events returns 404 for an unknown match", func() {
		server := newTestServer(client)
		defer server.Close()

		resp, err := http.Get(server.URL + "/matches/no-such-match/events")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})
})
