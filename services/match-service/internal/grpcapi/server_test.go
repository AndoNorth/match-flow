package grpcapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1/matchservicev1connect"
	"github.com/AndoNorth/match-flow/services/match-service/internal/grpcapi"
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

func newTestClient(store *fakeStore) matchservicev1connect.MatchServiceClient {
	mux := http.NewServeMux()
	path, handler := matchservicev1connect.NewMatchServiceHandler(grpcapi.NewServer(store))
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	DeferCleanup(server.Close)
	return matchservicev1connect.NewMatchServiceClient(server.Client(), server.URL)
}

var _ = Describe("Server", func() {
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

	It("ListMatches returns every match", func() {
		client := newTestClient(store)
		resp, err := client.ListMatches(context.Background(), connect.NewRequest(&matchservicev1.ListMatchesRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetMatches()).To(HaveLen(1))
		Expect(resp.Msg.GetMatches()[0].GetMatchId()).To(Equal("match-1"))
	})

	It("GetMatch returns the match", func() {
		client := newTestClient(store)
		resp, err := client.GetMatch(
			context.Background(),
			connect.NewRequest(&matchservicev1.GetMatchRequest{MatchId: "match-1"}),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetHomeScore()).To(Equal(int32(1)))
	})

	It("GetMatch returns CodeNotFound for an unknown match", func() {
		client := newTestClient(store)
		_, err := client.GetMatch(
			context.Background(),
			connect.NewRequest(&matchservicev1.GetMatchRequest{MatchId: "no-such-match"}),
		)
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
	})

	It("ListMatchEvents returns the timeline with payload as a Struct", func() {
		store.events["match-1"][0].Payload = map[string]any{"minute": float64(1)}
		client := newTestClient(store)
		resp, err := client.ListMatchEvents(
			context.Background(),
			connect.NewRequest(&matchservicev1.ListMatchEventsRequest{MatchId: "match-1"}),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetEvents()).To(HaveLen(2))
		Expect(resp.Msg.GetEvents()[0].GetType()).To(Equal("kickoff"))
		Expect(resp.Msg.GetEvents()[0].GetPayload().AsMap()).To(
			Equal(map[string]any{"minute": float64(1)}),
		)
	})

	It("ListMatchEvents returns CodeNotFound for an unknown match", func() {
		client := newTestClient(store)
		_, err := client.ListMatchEvents(
			context.Background(),
			connect.NewRequest(&matchservicev1.ListMatchEventsRequest{MatchId: "no-such-match"}),
		)
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
	})
})
