package eventstream_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/match-service/internal/eventstream"
)

// This fixture is duplicated byte-for-byte in
// services/ingestion-service/internal/normalize/canonical_contract_test.go.
// Both sides asserting the same field values against the same JSON is
// the cheap mitigation for the two services' independently-defined
// CanonicalEvent structs drifting apart silently - see the design
// spec's Non-Goal on extracting a shared module and its Validation
// section. If a future change to either struct's field names or JSON
// tags breaks this test, that's the drift this test exists to catch -
// keep both fixtures identical when fixing it.
const canonicalEventFixture = `{"payload":{"team":"home","minute":42},"timestamp":"2026-08-30T12:00:00Z","ingested_at":"2026-08-30T12:00:01Z","match_id":"match-1","type":"goal","provider":"provider-a","sequence":7}` //nolint:lll // fixture must stay byte-identical to the ingestion-service copy

var _ = Describe("CanonicalEvent contract", func() {
	It("decodes the shared fixture with the expected field values", func() {
		event, ok := eventstream.Decode([]byte(canonicalEventFixture))

		Expect(ok).To(BeTrue())
		Expect(event.MatchID).To(Equal("match-1"))
		Expect(event.Sequence).To(Equal(7))
		Expect(event.Type).To(Equal("goal"))
		Expect(event.Provider).To(Equal("provider-a"))
		Expect(event.Payload).To(Equal(map[string]any{"team": "home", "minute": float64(42)}))
	})

	It("encodes back to the same JSON shape the fixture uses", func() {
		var want map[string]any
		Expect(json.Unmarshal([]byte(canonicalEventFixture), &want)).To(Succeed())

		event, ok := eventstream.Decode([]byte(canonicalEventFixture))
		Expect(ok).To(BeTrue())

		encoded, err := json.Marshal(event)
		Expect(err).NotTo(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(encoded, &got)).To(Succeed())
		Expect(got).To(Equal(want))
	})
})
