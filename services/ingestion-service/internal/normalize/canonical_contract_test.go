package normalize_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/normalize"
)

// This fixture is duplicated byte-for-byte in
// services/match-service/internal/eventstream/canonical_contract_test.go -
// see that file's comment for why.
const canonicalEventFixture = `{"payload":{"team":"home","minute":42},"timestamp":"2026-08-30T12:00:00Z","ingested_at":"2026-08-30T12:00:01Z","match_id":"match-1","type":"goal","provider":"provider-a","sequence":7}` //nolint:lll // fixture must stay byte-identical to the match-service copy

var _ = Describe("CanonicalEvent contract", func() {
	It("decodes the shared fixture with the expected field values", func() {
		var event normalize.CanonicalEvent
		Expect(json.Unmarshal([]byte(canonicalEventFixture), &event)).To(Succeed())

		Expect(event.MatchID).To(Equal("match-1"))
		Expect(event.Sequence).To(Equal(7))
		Expect(event.Type).To(Equal("goal"))
		Expect(event.Provider).To(Equal("provider-a"))
		Expect(event.Payload).To(Equal(map[string]any{"team": "home", "minute": float64(42)}))
	})

	It("encodes back to the same JSON shape the fixture uses", func() {
		var want map[string]any
		Expect(json.Unmarshal([]byte(canonicalEventFixture), &want)).To(Succeed())

		var event normalize.CanonicalEvent
		Expect(json.Unmarshal([]byte(canonicalEventFixture), &event)).To(Succeed())

		encoded, err := json.Marshal(event)
		Expect(err).NotTo(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(encoded, &got)).To(Succeed())
		Expect(got).To(Equal(want))
	})
})
