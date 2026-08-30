package simulator_test

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

// fakeSport emits two events then reports done - enough to exercise
// alternating encoders and the match_complete log line without waiting
// on real football timing.
type fakeSport struct{ calls int }

func (f *fakeSport) NextEvent(state domain.MatchState) (domain.DomainEvent, bool, bool) {
	f.calls++
	if f.calls >= 2 {
		return domain.DomainEvent{Type: "test_event"}, true, true
	}
	return domain.DomainEvent{Type: "test_event"}, true, false
}

var _ = Describe("Runner", func() {
	It("logs each encoded payload alternating providers, then logs match_complete", func() {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		ticker := domain.NewFakeTicker()
		engine := domain.NewMatchEngine(&fakeSport{}, ticker, "match-1")
		runner := simulator.NewRunner(engine, []providers.Encoder{
			providers.EncodeProviderA,
			providers.EncodeProviderB,
		}, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		runDone := make(chan struct{})
		go func() {
			runner.Run(ctx)
			close(runDone)
		}()

		ticker.Fire()
		ticker.Fire()

		Eventually(runDone, 2*time.Second).Should(BeClosed())

		output := buf.String()
		// slog's TextHandler backslash-escapes the quotes inside the
		// quoted provider_payload value, so the JSON keys appear as
		// \"mid\" / \"match_id\" rather than "mid" / "match_id".
		Expect(output).To(ContainSubstring(`\"mid\"`))      // ProviderA payload present
		Expect(output).To(ContainSubstring(`\"match_id\"`)) // ProviderB payload present
		Expect(output).To(ContainSubstring("match_complete"))
	})
})
