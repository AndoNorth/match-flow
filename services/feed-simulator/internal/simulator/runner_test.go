package simulator_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func testRoutes() []simulator.ProviderRoute {
	return []simulator.ProviderRoute{
		{Encode: providers.EncodeProviderA, Route: "/events/provider-a"},
		{Encode: providers.EncodeProviderB, Route: "/events/provider-b"},
	}
}

var _ = Describe("Runner", func() {
	It("logs each encoded payload alternating providers, then logs match_complete", func() {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		ticker := domain.NewFakeTicker()
		engine := domain.NewMatchEngine(&fakeSport{}, ticker, "match-1")
		runner := simulator.NewRunner(engine, testRoutes(), nil, logger)

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

	It("submits each encoded payload to the correct Ingestion route", func() {
		type gotRequest struct {
			path string
			body string
		}
		var requests []gotRequest

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			requests = append(requests, gotRequest{path: r.URL.Path, body: string(body)})
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		ticker := domain.NewFakeTicker()
		engine := domain.NewMatchEngine(&fakeSport{}, ticker, "match-1")
		submitter := simulator.NewHTTPSubmitter(server.URL)
		runner := simulator.NewRunner(engine, testRoutes(), submitter, logger)

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

		Expect(requests).To(HaveLen(2))
		Expect(requests[0].path).To(Equal("/events/provider-a"))
		Expect(requests[0].body).To(ContainSubstring(`"mid"`))
		Expect(requests[1].path).To(Equal("/events/provider-b"))
		Expect(requests[1].body).To(ContainSubstring(`"match_id"`))
	})
})
