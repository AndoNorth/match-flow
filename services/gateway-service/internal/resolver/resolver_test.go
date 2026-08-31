package resolver_test

import (
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/danielgtaylor/huma/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/resolver"
)

var _ = Describe("Match", func() {
	It("converts every field", func() {
		createdAt := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
		body := resolver.Match(&matchservicev1.Match{
			MatchId: "match-1", Sport: "football", Status: "live",
			HomeTeam: "Ashford United", AwayTeam: "Denbury City",
			HomeScore: 1, AwayScore: 2, ClockMins: 45,
			CreatedAt: timestamppb.New(createdAt),
		})
		Expect(body).To(Equal(resolver.MatchBody{
			MatchID: "match-1", Sport: "football", Status: "live",
			HomeTeam: "Ashford United", AwayTeam: "Denbury City",
			HomeScore: 1, AwayScore: 2, ClockMins: 45,
			CreatedAt: createdAt,
		}))
	})
})

var _ = Describe("Events", func() {
	It("converts payload from a Struct to a map and returns empty, not nil, for no events", func() {
		payload, err := structpb.NewStruct(map[string]any{"minute": float64(1)})
		Expect(err).NotTo(HaveOccurred())

		events := resolver.Events([]*matchservicev1.MatchEvent{
			{Type: "goal", Sequence: 2, Payload: payload},
		})
		Expect(events).To(Equal([]resolver.EventBody{
			{Type: "goal", Sequence: 2, Payload: map[string]any{"minute": float64(1)}},
		}))

		Expect(resolver.Events(nil)).NotTo(BeNil())
		Expect(resolver.Events(nil)).To(BeEmpty())
	})
})

var _ = Describe("HTTPError", func() {
	It("maps CodeNotFound to a 404", func() {
		err := resolver.HTTPError(connect.NewError(connect.CodeNotFound, errors.New("no such match")))
		var statusErr huma.StatusError
		Expect(errors.As(err, &statusErr)).To(BeTrue())
		Expect(statusErr.GetStatus()).To(Equal(http.StatusNotFound))
	})

	It("maps CodeInvalidArgument to a 400", func() {
		err := resolver.HTTPError(connect.NewError(connect.CodeInvalidArgument, errors.New("bad request")))
		var statusErr huma.StatusError
		Expect(errors.As(err, &statusErr)).To(BeTrue())
		Expect(statusErr.GetStatus()).To(Equal(http.StatusBadRequest))
	})

	It("maps CodeUnavailable to a 503", func() {
		err := resolver.HTTPError(connect.NewError(connect.CodeUnavailable, errors.New("down")))
		var statusErr huma.StatusError
		Expect(errors.As(err, &statusErr)).To(BeTrue())
		Expect(statusErr.GetStatus()).To(Equal(http.StatusServiceUnavailable))
	})

	It("maps anything else to a 500", func() {
		err := resolver.HTTPError(errors.New("boom"))
		var statusErr huma.StatusError
		Expect(errors.As(err, &statusErr)).To(BeTrue())
		Expect(statusErr.GetStatus()).To(Equal(http.StatusInternalServerError))
	})
})
