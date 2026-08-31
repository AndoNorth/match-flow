// services/match-service/internal/matchstate/pool_integration_test.go
//go:build integration

package matchstate_test

import (
	"context"
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Pool", func() {
	var (
		pool  *matchstate.Pool
		store *matchstate.Store
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

		dbPool, err := pgxpool.New(ctx, dsn)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(dbPool.Close)

		store = matchstate.NewStore(dbPool, "football")
		pool = matchstate.NewPool(2, store, slog.Default())
	})

	It("applies every buffered item before Close returns, even after all sends are done", func() {
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
					Payload:  map[string]any{"team": "home"},
				},
				Rule: matchstate.Rule{Category: matchstate.ScoreEvent},
			},
			{
				Event: matchstate.Event{MatchID: "match-2", Sequence: 1, Type: "kickoff"},
				Rule:  matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
			},
		}
		for _, item := range items {
			idx := 0
			if item.Event.MatchID == "match-2" {
				idx = 1
			}
			pool.Send(idx, item)
		}

		pool.Close() // blocks until every buffered item is applied

		m1, err := store.GetMatch(ctx, "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(m1.HomeScore).To(Equal(1))

		m2, err := store.GetMatch(ctx, "match-2")
		Expect(err).NotTo(HaveOccurred())
		Expect(m2.Status).To(Equal("live"))
	})

	It("reports NumWorkers", func() {
		Expect(pool.NumWorkers()).To(Equal(2))
	})
})
