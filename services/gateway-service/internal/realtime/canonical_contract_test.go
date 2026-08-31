package realtime_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/realtime"
)

// This fixture is duplicated byte-for-byte in
// services/ingestion-service/internal/normalize/canonical_contract_test.go
// and services/match-service/internal/eventstream/canonical_contract_test.go.
const canonicalEventFixture = `{"payload":{"team":"home","minute":42},"timestamp":"2026-08-30T12:00:00Z","ingested_at":"2026-08-30T12:00:01Z","match_id":"match-1","type":"goal","provider":"provider-a","sequence":7}` //nolint:lll // fixture must stay byte-identical to the other two copies

var _ = Describe("CanonicalEvent contract", func() {
	It("decodes the shared fixture with the expected field values", func() {
		var event realtime.CanonicalEvent
		Expect(json.Unmarshal([]byte(canonicalEventFixture), &event)).To(Succeed())

		Expect(event.MatchID).To(Equal("match-1"))
		Expect(event.Sequence).To(Equal(7))
		Expect(event.Type).To(Equal("goal"))
		Expect(event.Payload).To(Equal(map[string]any{"team": "home", "minute": float64(42)}))
	})
})
