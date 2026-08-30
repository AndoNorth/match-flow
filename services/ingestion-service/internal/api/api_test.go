package api_test

import (
	"context"
	"errors"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/api"
	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/normalize"
)

var errPublishBoom = errors.New("boom")

type fakePublisher struct {
	published []normalize.CanonicalEvent
	failWith  error
}

func (f *fakePublisher) Publish(_ context.Context, event normalize.CanonicalEvent) error {
	if f.failWith != nil {
		return f.failWith
	}
	f.published = append(f.published, event)
	return nil
}

var _ = Describe("Register", func() {
	var (
		pub     *fakePublisher
		testAPI humatest.TestAPI
	)

	BeforeEach(func() {
		pub = &fakePublisher{}
		_, testAPI = humatest.New(GinkgoT(), huma.DefaultConfig("test", "0.0.0"))
		api.Register(testAPI, pub)
	})

	Describe("POST /events/provider-a", func() {
		It("accepts a valid payload and publishes the canonical event", func() {
			resp := testAPI.Post("/events/provider-a", map[string]any{
				"data": map[string]any{
					"mid": "match-1",
					"seq": 7,
					"typ": "odds_update",
					"ts":  1735689600,
					"pl":  map[string]any{"market": "match_winner"},
				},
			})

			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(pub.published).To(HaveLen(1))
			Expect(pub.published[0].MatchID).To(Equal("match-1"))
			Expect(pub.published[0].Sequence).To(Equal(7))
			Expect(pub.published[0].Type).To(Equal("odds_update"))
			Expect(pub.published[0].Provider).To(Equal("provider-a"))
			Expect(pub.published[0].Payload).To(Equal(map[string]any{"market": "match_winner"}))
		})

		It("rejects a missing match id with 422", func() {
			resp := testAPI.Post("/events/provider-a", map[string]any{
				"data": map[string]any{
					"seq": 7,
					"typ": "odds_update",
					"ts":  1735689600,
				},
			})

			Expect(resp.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(pub.published).To(BeEmpty())
		})

		It("rejects malformed JSON with 400", func() {
			resp := testAPI.Post("/events/provider-a", "Content-Type: application/json", strings.NewReader("not json"))

			Expect(resp.Code).To(Equal(http.StatusBadRequest))
			Expect(pub.published).To(BeEmpty())
		})
	})

	Describe("POST /events/provider-b", func() {
		It("accepts a valid payload and publishes the canonical event", func() {
			resp := testAPI.Post("/events/provider-b", map[string]any{
				"match_id":    "match-1",
				"sequence":    3,
				"event_type":  "goal",
				"occurred_at": "2026-08-30T12:00:00Z",
				"details":     map[string]any{"scorer": "home"},
			})

			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(pub.published).To(HaveLen(1))
			Expect(pub.published[0].MatchID).To(Equal("match-1"))
			Expect(pub.published[0].Provider).To(Equal("provider-b"))
		})

		It("rejects an unparseable timestamp with 400", func() {
			resp := testAPI.Post("/events/provider-b", map[string]any{
				"match_id":    "match-1",
				"sequence":    3,
				"event_type":  "goal",
				"occurred_at": "not-a-timestamp",
			})

			Expect(resp.Code).To(Equal(http.StatusBadRequest))
			Expect(pub.published).To(BeEmpty())
		})

		It("rejects a missing event type with 422", func() {
			resp := testAPI.Post("/events/provider-b", map[string]any{
				"match_id":    "match-1",
				"sequence":    3,
				"occurred_at": "2026-08-30T12:00:00Z",
			})

			Expect(resp.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(pub.published).To(BeEmpty())
		})
	})

	It("surfaces a publish failure as 500", func() {
		pub.failWith = errPublishBoom

		resp := testAPI.Post("/events/provider-a", map[string]any{
			"data": map[string]any{
				"mid": "match-1",
				"seq": 7,
				"typ": "odds_update",
				"ts":  1735689600,
			},
		})

		Expect(resp.Code).To(Equal(http.StatusInternalServerError))
	})
})
