package domain

import "time"

// Ticker is the seam that lets MatchEngine be driven by a real clock in
// production and a manually-fired fake in tests, with no wall-clock
// wait either way.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

// RealTicker wraps time.Ticker for production use.
type RealTicker struct {
	t *time.Ticker
}

func NewRealTicker(d time.Duration) *RealTicker {
	return &RealTicker{t: time.NewTicker(d)}
}

func (r *RealTicker) Chan() <-chan time.Time { return r.t.C }
func (r *RealTicker) Stop()                  { r.t.Stop() }

// FakeTicker is a manually-fired tick source for tests - no timers, no
// wall-clock waiting.
type FakeTicker struct {
	ch chan time.Time
}

func NewFakeTicker() *FakeTicker {
	return &FakeTicker{ch: make(chan time.Time, 1)}
}

func (f *FakeTicker) Chan() <-chan time.Time { return f.ch }
func (f *FakeTicker) Stop()                  {}

// Fire sends one tick. Buffered size 1, so it never blocks waiting for
// the engine to be mid-select as long as the engine reads ticks faster
// than the test fires them (true for every test in this package).
func (f *FakeTicker) Fire() { f.ch <- time.Now() }
