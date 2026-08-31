package eventstream_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/match-service/internal/eventstream"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Decode", func() {
	It("parses a well-formed canonical event", func() {
		data := []byte(
			`{"match_id":"match-1","sequence":1,"type":"goal","payload":{"team":"home"},"timestamp":"2026-08-30T12:00:00Z","ingested_at":"2026-08-30T12:00:01Z","provider":"provider-a"}`, //nolint:lll
		)

		event, ok := eventstream.Decode(data)

		Expect(ok).To(BeTrue())
		Expect(event.MatchID).To(Equal("match-1"))
		Expect(event.Sequence).To(Equal(1))
		Expect(event.Type).To(Equal("goal"))
	})

	It("rejects malformed JSON", func() {
		_, ok := eventstream.Decode([]byte("not json"))
		Expect(ok).To(BeFalse())
	})
})

var _ = Describe("Route", func() {
	It("drops odds_update events", func() {
		event := eventstream.CanonicalEvent{MatchID: "match-1", Type: "odds_update"}

		_, _, ok := eventstream.Route(event, 4)

		Expect(ok).To(BeFalse())
	})

	It("looks up the Rule for a known type and preserves the event fields", func() {
		event := eventstream.CanonicalEvent{
			MatchID:  "match-1",
			Sequence: 3,
			Type:     "goal",
			Payload:  map[string]any{"team": "home"},
		}

		item, _, ok := eventstream.Route(event, 4)

		Expect(ok).To(BeTrue())
		Expect(item.Event.MatchID).To(Equal("match-1"))
		Expect(item.Event.Sequence).To(Equal(3))
		Expect(item.Event.Type).To(Equal("goal"))
		Expect(item.Event.Payload).To(Equal(map[string]any{"team": "home"}))
		Expect(item.Rule.Category).To(Equal(matchstate.ScoreEvent))
	})

	It("resolves an unrecognized type to Rule{Category: Unknown} rather than dropping it", func() {
		event := eventstream.CanonicalEvent{MatchID: "match-1", Type: "some_future_type"}

		item, _, ok := eventstream.Route(event, 4)

		Expect(ok).To(BeTrue())
		Expect(item.Rule.Category).To(Equal(matchstate.Unknown))
	})

	It("routes every event for the same MatchID to the same worker index", func() {
		a := eventstream.CanonicalEvent{MatchID: "match-1", Type: "goal"}
		b := eventstream.CanonicalEvent{MatchID: "match-1", Type: "card"}

		_, idxA, _ := eventstream.Route(a, 8)
		_, idxB, _ := eventstream.Route(b, 8)

		Expect(idxA).To(Equal(idxB))
	})
})

var _ = Describe("WorkerIndex", func() {
	It("is always within [0, numWorkers)", func() {
		for _, matchID := range []string{"match-1", "match-2", "another-match", ""} {
			idx := eventstream.WorkerIndex(matchID, 4)
			Expect(idx).To(BeNumerically(">=", 0))
			Expect(idx).To(BeNumerically("<", 4))
		}
	})
})
