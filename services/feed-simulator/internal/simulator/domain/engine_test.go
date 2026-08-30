package domain_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
)

// fakeSport is a minimal Sport test double: quiet for the first two
// ticks, emits one event on the third tick, then reports done on the
// fourth.
type fakeSport struct{ calls int }

func (f *fakeSport) NextEvent(state domain.MatchState) (domain.DomainEvent, bool, bool) {
	f.calls++
	switch f.calls {
	case 1, 2:
		return domain.DomainEvent{}, false, false // quiet ticks
	case 3:
		return domain.DomainEvent{Type: "test_event"}, true, false
	default:
		return domain.DomainEvent{Type: "test_event"}, true, true // done
	}
}

var _ = Describe("MatchEngine", func() {
	It("skips quiet ticks, emits events, fills MatchID/Sequence/Timestamp, and stops on done", func() {
		ticker := domain.NewFakeTicker()
		engine := domain.NewMatchEngine(&fakeSport{}, ticker, "match-1")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		events := engine.Run(ctx)

		var received []domain.DomainEvent
		done := make(chan struct{})
		go func() {
			for e := range events {
				received = append(received, e)
			}
			close(done)
		}()

		for i := 0; i < 4; i++ {
			ticker.Fire()
		}

		Eventually(done, 2*time.Second).Should(BeClosed())

		Expect(received).To(HaveLen(2))
		Expect(received[0].Sequence).To(Equal(3))
		Expect(received[0].MatchID).To(Equal("match-1"))
		Expect(received[0].Timestamp).NotTo(BeZero())
		Expect(received[1].Sequence).To(Equal(4))
	})
})
