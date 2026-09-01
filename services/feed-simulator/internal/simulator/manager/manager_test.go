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
		m := manager.New(noopRoutes, nil, discardLogger(), GinkgoT().TempDir(), fastTickInterval)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		matchID, err := m.Start(ctx, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(matchID).NotTo(BeEmpty())
		Expect(m.RunningCount()).To(Equal(1))
	})

	It("Trigger starts an additional match alongside one already running", func() {
		m := manager.New(noopRoutes, nil, discardLogger(), GinkgoT().TempDir(), fastTickInterval)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, err := m.Trigger(ctx, "")
		Expect(err).NotTo(HaveOccurred())
		_, err = m.Trigger(ctx, "")
		Expect(err).NotTo(HaveOccurred())

		Expect(m.RunningCount()).To(Equal(2))
	})

	It("Stop cancels every running match and halts the auto-loop", func() {
		dir := GinkgoT().TempDir()
		writeInstantFullTimeTemplate(dir)
		m := manager.New(noopRoutes, nil, discardLogger(), dir, fastTickInterval)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, err := m.Start(ctx, "instant_full_time")
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
		m := manager.New(noopRoutes, nil, discardLogger(), GinkgoT().TempDir(), fastTickInterval)
		_, err := m.Trigger(context.Background(), "does-not-exist")
		Expect(err).To(HaveOccurred())
	})
})
