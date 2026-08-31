package realtime_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/realtime"
)

var _ = Describe("eventPayload", func() {
	It("includes match_id alongside type, sequence, and payload", func() {
		event := realtime.CanonicalEvent{
			MatchID:    "match-1",
			Type:       "goal",
			Sequence:   2,
			Payload:    map[string]any{"team": "home", "minute": float64(23)},
			Timestamp:  time.Unix(0, 0),
			IngestedAt: time.Unix(0, 0),
		}

		raw, err := realtime.EventPayload(event)
		Expect(err).NotTo(HaveOccurred())

		var decoded map[string]any
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		Expect(decoded).To(Equal(map[string]any{
			"match_id": "match-1",
			"type":     "goal",
			"sequence": float64(2),
			"payload":  map[string]any{"team": "home", "minute": float64(23)},
		}))
	})
})
