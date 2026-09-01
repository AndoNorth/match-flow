// Package manager runs zero or more matches concurrently, each its
// own domain.MatchEngine + simulator.Runner, and owns the three
// operations an operator needs over HTTP: Start (turn on an auto-loop
// that keeps a match running forever, respawning a new one - by the
// same template - each time one finishes), Stop (halt every running
// match and turn the auto-loop off), and Trigger (start exactly one
// additional match right now, independent of the auto-loop).
package manager

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/domain"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/football"
	"github.com/AndoNorth/match-flow/services/feed-simulator/internal/simulator/template"
)

// Manager tracks every currently-running match by ID, guarded by mu -
// Start/Stop/Trigger/the auto-respawn callback all touch running and
// auto/autoTemplate together, so every read or write to them happens
// under the same lock.
//
// Every spawned match derives its context from baseCtx, set once at
// construction - never from an HTTP handler's request context. A
// request's context is canceled the instant its handler returns, so a
// match parented on one would die (and, if auto-respawn was on,
// immediately spawn its replacement from that same already-canceled
// context) within microseconds of the response being sent - a tight
// respawn loop bounded only by CPU speed, discovered exactly this way
// against a real running instance.
type Manager struct {
	logger       *slog.Logger
	submitter    simulator.Submitter
	baseCtx      context.Context
	running      map[string]context.CancelFunc
	autoTemplate string
	templatesDir string
	routes       []simulator.ProviderRoute
	nextID       atomic.Int64
	tickInterval time.Duration
	mu           sync.Mutex
	auto         bool
}

// New builds a Manager. baseCtx is the service's own long-lived
// lifetime context (e.g. one tied to process signals in main) - every
// match Start/Trigger spawns, and every auto-respawned replacement,
// derives from it, so a match's lifetime is never tied to whichever
// HTTP request happened to ask for it. tickInterval is production's
// real per-minute tick rate (domain.NewRealTicker(tickInterval)) -
// tests pass a much smaller duration so a 90-minute match plays out in
// milliseconds, not 90 real seconds.
func New(
	baseCtx context.Context,
	routes []simulator.ProviderRoute,
	submitter simulator.Submitter,
	logger *slog.Logger,
	templatesDir string,
	tickInterval time.Duration,
) *Manager {
	return &Manager{
		baseCtx:      baseCtx,
		running:      make(map[string]context.CancelFunc),
		routes:       routes,
		submitter:    submitter,
		logger:       logger,
		templatesDir: templatesDir,
		tickInterval: tickInterval,
	}
}

// RunningCount reports how many matches are currently in progress.
func (m *Manager) RunningCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.running)
}

// Start turns the auto-respawn loop on: every future match completion
// immediately starts a new one using templateName (""  means the
// default, unbounded-random mode). If nothing is currently running, it
// also spawns the first match immediately - calling Start again just
// changes the template future respawns use.
func (m *Manager) Start(templateName string) (string, error) {
	m.mu.Lock()
	m.auto = true
	m.autoTemplate = templateName
	needsSpawn := len(m.running) == 0
	m.mu.Unlock()

	if !needsSpawn {
		return "", nil
	}
	return m.spawn(templateName)
}

// Stop turns the auto-respawn loop off and immediately cancels every
// currently-running match - a hard stop, not "let the current one
// finish naturally."
func (m *Manager) Stop() {
	m.mu.Lock()
	m.auto = false
	cancels := make([]context.CancelFunc, 0, len(m.running))
	for _, cancel := range m.running {
		cancels = append(cancels, cancel)
	}
	m.running = make(map[string]context.CancelFunc)
	m.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

// Trigger starts exactly one additional match right now, running
// concurrently alongside anything else already in progress, using
// templateName ("" for the default random mode). Independent of
// Start/Stop's auto-loop state.
func (m *Manager) Trigger(templateName string) (string, error) {
	return m.spawn(templateName)
}

func (m *Manager) spawn(templateName string) (string, error) {
	sport, err := m.buildSport(templateName)
	if err != nil {
		return "", err
	}

	matchID := fmt.Sprintf("match-%d", m.nextID.Add(1))
	matchCtx, cancel := context.WithCancel(m.baseCtx)

	m.mu.Lock()
	m.running[matchID] = cancel
	m.mu.Unlock()

	ticker := domain.NewRealTicker(m.tickInterval)
	engine := domain.NewMatchEngine(sport, ticker, matchID)
	runner := simulator.NewRunner(engine, m.routes, m.submitter, m.logger)

	go func() {
		runner.Run(matchCtx)
		m.onComplete(matchID)
	}()

	return matchID, nil
}

// onComplete removes matchID from running and, if the auto-loop is
// still on, immediately spawns its replacement - this is what makes
// Start's "keep feeding live matches forever" promise actually hold.
func (m *Manager) onComplete(matchID string) {
	m.mu.Lock()
	delete(m.running, matchID)
	shouldRespawn := m.auto
	nextTemplate := m.autoTemplate
	m.mu.Unlock()

	// Also guards against respawning from an already-canceled baseCtx
	// during service shutdown - the same tight-loop failure mode this
	// package exists to avoid, just triggered by the process exiting
	// instead of by a request context.
	if !shouldRespawn || m.baseCtx.Err() != nil {
		return
	}
	if _, err := m.spawn(nextTemplate); err != nil {
		m.logger.Error("auto-respawn failed", "error", err, "template", nextTemplate)
	}
}

func (m *Manager) buildSport(templateName string) (domain.Sport, error) {
	if templateName == "" {
		return football.New(time.Now().UnixNano()), nil //nolint:gosec // seed only needs to differ per match
	}

	templates, err := template.LoadAll(m.templatesDir)
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}
	tmpl, ok := templates[templateName]
	if !ok {
		return nil, fmt.Errorf("unknown template %q", templateName)
	}
	sport, err := football.NewFromTemplate(
		time.Now().UnixNano(),
		tmpl,
	) //nolint:gosec // seed only needs to differ per match
	if err != nil {
		return nil, fmt.Errorf("build sport from template %q: %w", templateName, err)
	}
	return sport, nil
}

// TemplatesDir returns the directory Manager loads templates from -
// exposed only so internal/api can list available template names.
func (m *Manager) TemplatesDir() string {
	return filepath.Clean(m.templatesDir)
}
