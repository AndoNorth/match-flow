//go:build integration

package matchstate_test

import (
	"context"
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_ "github.com/jackc/pgx/v5/stdlib"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Migrate", func() {
	It("creates matches and match_events", func() {
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

		db, err := sql.Open("pgx", dsn)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = db.Close() })

		var tableCount int
		err = db.QueryRowContext(ctx,
			`SELECT count(*) FROM information_schema.tables WHERE table_name IN ('matches', 'match_events')`,
		).Scan(&tableCount)
		Expect(err).NotTo(HaveOccurred())
		Expect(tableCount).To(Equal(2))
	})

	It("is safe to call twice (idempotent)", func() {
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
		Expect(matchstate.Migrate(ctx, dsn)).To(Succeed())
	})
})
