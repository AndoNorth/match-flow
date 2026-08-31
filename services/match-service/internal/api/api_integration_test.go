// services/match-service/internal/api/api_integration_test.go
//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/AndoNorth/match-flow/services/match-service/internal/api"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Register against a real Store", func() {
	It("round-trips a match applied via the Store through the REST API", func() {
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

		pool, err := pgxpool.New(ctx, dsn)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(pool.Close)

		store := matchstate.NewStore(pool, "football")
		Expect(store.Apply(ctx, matchstate.WorkItem{
			Event: matchstate.Event{MatchID: "match-1", Sequence: 1, Type: "kickoff"},
			Rule:  matchstate.Rule{Category: matchstate.MatchStart, Status: "live"},
		})).To(Succeed())

		mux := http.NewServeMux()
		humaAPI := humago.New(mux, huma.DefaultConfig("test", "0.0.0"))
		api.Register(humaAPI, store)
		server := httptest.NewServer(mux)
		DeferCleanup(server.Close)

		resp, err := http.Get(server.URL + "/matches/match-1")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
