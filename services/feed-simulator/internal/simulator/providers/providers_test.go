package providers_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

var _ = Describe("Providers", func() {
	// No ints in Payload - JSON round-trips numbers as float64, and a
	// mixed int/float64 comparison would make this test fight the
	// encoding instead of testing it. Use only strings and floats.
	testEvent := domain.DomainEvent{
		MatchID:   "match-1",
		Sequence:  7,
		Type:      "odds_update",
		Timestamp: time.Now().Truncate(time.Second), // both encoders are second-precision
		Payload: map[string]any{
			"market":    "match_winner",
			"selection": "home",
			"price":     2.4,
		},
	}

	It("ProviderA round-trips to the same logical event", func() {
		encoded, err := providers.EncodeProviderA(testEvent)
		Expect(err).NotTo(HaveOccurred())

		decoded, err := providers.DecodeProviderA(encoded)
		Expect(err).NotTo(HaveOccurred())

		Expect(decoded.MatchID).To(Equal(testEvent.MatchID))
		Expect(decoded.Sequence).To(Equal(testEvent.Sequence))
		Expect(decoded.Type).To(Equal(testEvent.Type))
		Expect(decoded.Timestamp.Unix()).To(Equal(testEvent.Timestamp.Unix()))
		Expect(decoded.Payload).To(Equal(testEvent.Payload))
	})

	It("ProviderB round-trips to the same logical event", func() {
		encoded, err := providers.EncodeProviderB(testEvent)
		Expect(err).NotTo(HaveOccurred())

		decoded, err := providers.DecodeProviderB(encoded)
		Expect(err).NotTo(HaveOccurred())

		Expect(decoded.MatchID).To(Equal(testEvent.MatchID))
		Expect(decoded.Sequence).To(Equal(testEvent.Sequence))
		Expect(decoded.Type).To(Equal(testEvent.Type))
		Expect(decoded.Timestamp.Unix()).To(Equal(testEvent.Timestamp.Unix()))
		Expect(decoded.Payload).To(Equal(testEvent.Payload))
	})

	It("produces structurally different JSON for the same event", func() {
		a, err := providers.EncodeProviderA(testEvent)
		Expect(err).NotTo(HaveOccurred())
		b, err := providers.EncodeProviderB(testEvent)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(a)).To(ContainSubstring(`"mid"`))
		Expect(string(a)).NotTo(ContainSubstring(`"match_id"`))
		Expect(string(b)).To(ContainSubstring(`"match_id"`))
		Expect(string(b)).NotTo(ContainSubstring(`"mid"`))
	})
})
