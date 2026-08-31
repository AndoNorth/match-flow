//go:build integration

package matchstate_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Store", func() {
	var (
		store *matchstate.Store
		pool  *pgxpool.Pool
	)

	BeforeEach(func() {
		ctx := context.Background()

		container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("matchflow"),
			tcpostgres.WithUsername("matchflow"),
			tcpostgres.WithPassword("matchflow"),
		)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = container.Terminate(context.Background()) })

		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())
		Expect(matchstate.Migrate(ctx, dsn)).To(Succeed())

		pool, err = pgxpool.New(ctx, dsn)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(pool.Close)

		store = matchstate.NewStore(pool, "football")
	})

	It("creates a match on first-seen MatchID with the configured sport", func() {
		ctx := context.Background()
		item := matchstate.WorkItem{
			Event: matchstate.Event{MatchID: "match-1", Sequence: 1, Type: "kickoff"},
			Rule:  matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
		}

		Expect(store.Apply(ctx, item)).To(Succeed())

		record, err := store.GetMatch(ctx, "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(record.Sport).To(Equal("football"))
		Expect(record.Status).To(Equal("live"))
	})

	It("applies a full sequence and reflects the final state", func() {
		ctx := context.Background()
		items := []matchstate.WorkItem{
			{
				Event: matchstate.Event{MatchID: "match-1", Sequence: 1, Type: "kickoff"},
				Rule:  matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
			},
			{
				Event: matchstate.Event{
					MatchID:  "match-1",
					Sequence: 2,
					Type:     "goal",
					Payload:  map[string]any{"team": "home", "minute": 10},
				},
				Rule: matchstate.Rule{Category: matchstate.ScoreEvent},
			},
			{
				Event: matchstate.Event{
					MatchID:  "match-1",
					Sequence: 3,
					Type:     "card",
					Payload:  map[string]any{"team": "away", "minute": 20},
				},
				Rule: matchstate.Rule{Category: matchstate.Unknown},
			},
			{
				Event: matchstate.Event{MatchID: "match-1", Sequence: 4, Type: "full_time"},
				Rule:  matchstate.Rule{Category: matchstate.MatchEnd, Status: "full_time"},
			},
		}
		for _, item := range items {
			Expect(store.Apply(ctx, item)).To(Succeed())
		}

		record, err := store.GetMatch(ctx, "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(record.Status).To(Equal("full_time"))
		Expect(record.HomeScore).To(Equal(1))
		Expect(record.AwayScore).To(Equal(0))
		Expect(record.ClockMins).To(Equal(10)) // card never updates the clock

		events, err := store.ListEvents(ctx, "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(4))
		Expect(events[2].Type).To(Equal("card")) // ordered by sequence
	})

	It("skips a duplicate/stale Sequence without changing state or adding an event row", func() {
		ctx := context.Background()
		first := matchstate.WorkItem{
			Event: matchstate.Event{
				MatchID:  "match-1",
				Sequence: 1,
				Type:     "goal",
				Payload:  map[string]any{"team": "home"},
			},
			Rule: matchstate.Rule{Category: matchstate.ScoreEvent},
		}
		Expect(store.Apply(ctx, first)).To(Succeed())

		duplicate := matchstate.WorkItem{
			Event: matchstate.Event{
				MatchID:  "match-1",
				Sequence: 1,
				Type:     "goal",
				Payload:  map[string]any{"team": "home"},
			},
			Rule: matchstate.Rule{Category: matchstate.ScoreEvent},
		}
		Expect(store.Apply(ctx, duplicate)).To(Succeed())

		record, err := store.GetMatch(ctx, "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(record.HomeScore).To(Equal(1)) // not double-counted

		events, err := store.ListEvents(ctx, "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(1))
	})

	It("returns ErrNotFound for an unknown match", func() {
		ctx := context.Background()
		_, err := store.GetMatch(ctx, "no-such-match")
		Expect(errors.Is(err, matchstate.ErrNotFound)).To(BeTrue())

		_, err = store.ListEvents(ctx, "no-such-match")
		Expect(errors.Is(err, matchstate.ErrNotFound)).To(BeTrue())
	})

	It("lists matches filtered by status", func() {
		ctx := context.Background()
		Expect(store.Apply(ctx, matchstate.WorkItem{
			Event: matchstate.Event{MatchID: "live-1", Sequence: 1, Type: "kickoff"},
			Rule:  matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
		})).To(Succeed())
		Expect(store.Apply(ctx, matchstate.WorkItem{
			Event: matchstate.Event{MatchID: "done-1", Sequence: 1, Type: "kickoff"},
			Rule:  matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
		})).To(Succeed())
		Expect(store.Apply(ctx, matchstate.WorkItem{
			Event: matchstate.Event{MatchID: "done-1", Sequence: 2, Type: "full_time"},
			Rule:  matchstate.Rule{Category: matchstate.MatchEnd, Status: "full_time"},
		})).To(Succeed())

		live, err := store.ListMatches(ctx, "live")
		Expect(err).NotTo(HaveOccurred())
		Expect(live).To(HaveLen(1))
		Expect(live[0].MatchID).To(Equal("live-1"))

		all, err := store.ListMatches(ctx, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(all).To(HaveLen(2))
	})
})
