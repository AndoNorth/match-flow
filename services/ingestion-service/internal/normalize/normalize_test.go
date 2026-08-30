package normalize_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/normalize"
)

var _ = Describe("Build", func() {
	It("attaches provenance to an Input, unchanged otherwise", func() {
		input := normalize.Input{
			MatchID:   "match-1",
			Sequence:  7,
			Type:      "odds_update",
			Timestamp: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC),
			Payload: map[string]any{
				"market": "match_winner",
			},
		}
		ingestedAt := time.Date(2026, 8, 30, 12, 0, 1, 0, time.UTC)

		event := normalize.Build(input, "provider-a", ingestedAt)

		Expect(event.MatchID).To(Equal(input.MatchID))
		Expect(event.Sequence).To(Equal(input.Sequence))
		Expect(event.Type).To(Equal(input.Type))
		Expect(event.Timestamp).To(Equal(input.Timestamp))
		Expect(event.Payload).To(Equal(input.Payload))
		Expect(event.Provider).To(Equal("provider-a"))
		Expect(event.IngestedAt).To(Equal(ingestedAt))
	})

	It("produces an identical event for both providers apart from Provider", func() {
		input := normalize.Input{
			MatchID:   "match-1",
			Sequence:  3,
			Type:      "goal",
			Timestamp: time.Date(2026, 8, 30, 12, 5, 0, 0, time.UTC),
			Payload:   map[string]any{"scorer": "home"},
		}
		ingestedAt := time.Date(2026, 8, 30, 12, 5, 1, 0, time.UTC)

		eventA := normalize.Build(input, "provider-a", ingestedAt)
		eventB := normalize.Build(input, "provider-b", ingestedAt)

		eventA.Provider = ""
		eventB.Provider = ""
		Expect(eventA).To(Equal(eventB))
	})
})
