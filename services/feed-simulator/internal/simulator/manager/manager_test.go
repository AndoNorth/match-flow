package manager_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/manager"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/providers"
)

// noopRoutes gives Runner at least one route to pick from - it panics
// (modulo by zero) if given an empty slice.
var noopRoutes = []simulator.ProviderRoute{{Encode: providers.EncodeProviderA, Route: "/events/provider-a"}}

// fastTickInterval keeps every test in the low milliseconds - a
// 90-minute match plays out in 90 * fastTickInterval.
const fastTickInterval = time.Millisecond

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(GinkgoWriter, nil))
}

// newManager builds a Manager against a base context tied to this
// test's own lifetime (canceled on test cleanup, not on any simulated
// HTTP request), matching how main.go builds the real one.
func newManager(dir string) *manager.Manager {
	ctx, cancel := context.WithCancel(context.Background())
	DeferCleanup(cancel)
	return manager.New(ctx, noopRoutes, nil, discardLogger(), dir, fastTickInterval)
}

// writeInstantFullTimeTemplate writes a literal template that reaches
// full_time on the very first tick - the fastest possible way to force
// a match completion (and therefore an auto-respawn) in a test.
func writeInstantFullTimeTemplate(dir string) {
	Expect(os.WriteFile(filepath.Join(dir, "instant.json"), []byte(`{
		"name": "instant_full_time", "kind": "literal",
		"events": [{"type": "full_time", "minute": 1}]
	}`), 0o600)).To(Succeed())
}

var _ = Describe("Manager", func() {
	It("Start spawns a match immediately", func() {
		m := newManager(GinkgoT().TempDir())

		matchID, err := m.Start("")
		Expect(err).NotTo(HaveOccurred())
		Expect(matchID).NotTo(BeEmpty())
		Expect(m.RunningCount()).To(Equal(1))
	})

	It("Trigger starts an additional match alongside one already running", func() {
		m := newManager(GinkgoT().TempDir())

		_, err := m.Trigger("")
		Expect(err).NotTo(HaveOccurred())
		_, err = m.Trigger("")
		Expect(err).NotTo(HaveOccurred())

		Expect(m.RunningCount()).To(Equal(2))
	})

	// Regression test for a real bug found against a running instance:
	// Start/Trigger used to take a caller-supplied context and parent
	// every spawned match on it. Called from an HTTP handler with the
	// request's own context, that context is canceled the instant the
	// handler returns - killing the match (and, with auto-respawn on,
	// immediately respawning its replacement from that same
	// already-canceled context, in a tight loop bounded only by CPU
	// speed) within microseconds of the response being sent. Neither
	// Start nor Trigger take a context parameter at all now - only the
	// Manager's own long-lived base context (fixed at construction)
	// may parent a match, so this failure mode can't come back through
	// this API. This test proves the match a call spawns actually
	// outlives that call returning, by many tick intervals.
	It("keeps a match running long after the call that started it has returned", func() {
		m := newManager(GinkgoT().TempDir())

		_, err := m.Trigger("")
		Expect(err).NotTo(HaveOccurred())
		Expect(m.RunningCount()).To(Equal(1))

		time.Sleep(50 * time.Millisecond) // 50 tick intervals at fastTickInterval
		Expect(m.RunningCount()).To(Equal(1))
	})

	It("Stop cancels every running match and halts the auto-loop", func() {
		dir := GinkgoT().TempDir()
		writeInstantFullTimeTemplate(dir)
		m := newManager(dir)

		_, err := m.Start("instant_full_time")
		Expect(err).NotTo(HaveOccurred())

		// Let a few auto-respawn cycles happen - each completes in ~1
		// tick, so this window covers several.
		time.Sleep(50 * time.Millisecond)

		m.Stop()
		Expect(m.RunningCount()).To(Equal(0))

		// If Stop had only cancelled the current match without
		// disabling auto-respawn, a new one would appear here.
		time.Sleep(50 * time.Millisecond)
		Expect(m.RunningCount()).To(Equal(0))
	})

	It("Trigger returns an error for an unknown template", func() {
		m := newManager(GinkgoT().TempDir())
		_, err := m.Trigger("does-not-exist")
		Expect(err).To(HaveOccurred())
	})
})
