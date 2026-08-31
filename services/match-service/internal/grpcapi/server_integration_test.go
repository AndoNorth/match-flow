//go:build integration

package grpcapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	matchservicev1 "github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1"
	"github.com/AndoNorth/match-flow/gen/go/matchflow/match_service/v1/matchservicev1connect"
	"github.com/AndoNorth/match-flow/services/match-service/internal/grpcapi"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Server against a real Store", func() {
	It("round-trips a match applied via the Store through the gRPC API", func() {
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
		path, handler := matchservicev1connect.NewMatchServiceHandler(grpcapi.NewServer(store))
		mux.Handle(path, handler)
		server := httptest.NewServer(mux)
		DeferCleanup(server.Close)

		client := matchservicev1connect.NewMatchServiceClient(server.Client(), server.URL)
		resp, err := client.GetMatch(ctx, connect.NewRequest(&matchservicev1.GetMatchRequest{MatchId: "match-1"}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetStatus()).To(Equal("live"))
	})
})
