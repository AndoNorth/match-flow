package realtime_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/realtime"
)

type fakeMatchGetter struct {
	match *matchservicev1.Match
}

func (f *fakeMatchGetter) GetMatch(_ context.Context, _ string) (*matchservicev1.Match, error) {
	return f.match, nil
}

func (f *fakeMatchGetter) ListMatches(_ context.Context, _ string) (*matchservicev1.ListMatchesResponse, error) {
	return &matchservicev1.ListMatchesResponse{Matches: []*matchservicev1.Match{f.match}}, nil
}

var _ = Describe("Handler", func() {
	It("writes a snapshot frame, then an update frame when one is broadcast", func() {
		registry := realtime.NewRegistry()
		client := &fakeMatchGetter{match: &matchservicev1.Match{MatchId: "match-1", Status: "live"}}
		handler := realtime.Handler(registry, client)

		server := httptest.NewServer(handler)
		defer server.Close()

		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			server.URL+"?match_id=match-1",
			nil,
		)
		Expect(err).NotTo(HaveOccurred())
		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()

		reader := bufio.NewReader(resp.Body)

		line, err := reader.ReadString('\n')
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(line)).To(Equal("event: snapshot"))
		dataLine, err := reader.ReadString('\n')
		Expect(err).NotTo(HaveOccurred())
		Expect(dataLine).To(ContainSubstring(`"match_id":"match-1"`))
		blankLine, err := reader.ReadString('\n')
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(blankLine)).To(BeEmpty())

		done := make(chan struct{})
		go func() {
			defer close(done)
			eventLine, err := reader.ReadString('\n')
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(eventLine)).To(Equal("event: update"))
			updateLine, err := reader.ReadString('\n')
			Expect(err).NotTo(HaveOccurred())
			Expect(updateLine).To(ContainSubstring(`"type":"goal"`))
		}()

		Eventually(func() bool {
			registry.Broadcast("match-1", []byte(`{"type":"goal","sequence":1,"payload":null}`))
			select {
			case <-done:
				return true
			case <-time.After(10 * time.Millisecond):
				return false
			}
		}, "1s").Should(BeTrue())
	})
})
