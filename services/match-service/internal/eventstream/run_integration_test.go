// services/match-service/internal/eventstream/run_integration_test.go
//go:build integration

package eventstream_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/AndoNorth/match-flow/services/match-service/internal/eventstream"
	"github.com/AndoNorth/match-flow/services/match-service/internal/matchstate"
)

var _ = Describe("Run", func() {
	var (
		redisClient *redis.Client
		store       *matchstate.Store
		pool        *matchstate.Pool
		ctx         context.Context
		cancel      context.CancelFunc
		done        chan struct{}
	)

	BeforeEach(func() {
		bgCtx := context.Background()

		pgContainer, err := tcpostgres.Run(bgCtx, "postgres:16-alpine",
			tcpostgres.WithDatabase("matchflow"),
			tcpostgres.WithUsername("matchflow"),
			tcpostgres.WithPassword("matchflow"),
		)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = pgContainer.Terminate(context.Background()) })

		dsn, err := pgContainer.ConnectionString(bgCtx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())
		Expect(matchstate.Migrate(bgCtx, dsn)).To(Succeed())

		dbPool, err := pgxpool.New(bgCtx, dsn)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(dbPool.Close)
		store = matchstate.NewStore(dbPool, "football")

		redisContainer, err := tcredis.Run(bgCtx, "redis:7-alpine")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = redisContainer.Terminate(context.Background()) })

		connStr, err := redisContainer.ConnectionString(bgCtx)
		Expect(err).NotTo(HaveOccurred())
		opts, err := redis.ParseURL(connStr)
		Expect(err).NotTo(HaveOccurred())
		redisClient = redis.NewClient(opts)
		DeferCleanup(func() { _ = redisClient.Close() })

		pool = matchstate.NewPool(4, store, slog.Default())

		ctx, cancel = context.WithCancel(bgCtx)
		done = make(chan struct{})
		go func() {
			eventstream.Run(ctx, redisClient, pool, slog.Default())
			close(done)
		}()

		// Pub/Sub is fire-and-forget: a message published before the
		// subscription above is registered server-side is simply lost,
		// no error either side. Wait for the subscriber to actually be
		// registered before any test publishes, so a slow goroutine
		// schedule/connection dial (this sandbox's Docker networking is
		// slower than bare-metal) can't race the first publish.
		Eventually(func() (int64, error) {
			counts, err := redisClient.PubSubNumSub(context.Background(), eventstream.Channel).Result()
			if err != nil {
				return 0, err
			}
			return counts[eventstream.Channel], nil
		}, "5s", "10ms").Should(BeNumerically(">=", 1))
	})

	publish := func(matchID string, seq int, eventType string, payload map[string]any) {
		event := eventstream.CanonicalEvent{
			MatchID: matchID, Sequence: seq, Type: eventType, Payload: payload,
			Timestamp: time.Now(), IngestedAt: time.Now(), Provider: "test",
		}
		data, err := json.Marshal(event)
		Expect(err).NotTo(HaveOccurred())
		Expect(redisClient.Publish(context.Background(), eventstream.Channel, data).Err()).To(Succeed())
	}

	It("applies a full event sequence for one match, and ignores odds_update", func() {
		publish("match-1", 1, "kickoff", nil)
		publish("match-1", 2, "goal", map[string]any{"team": "home", "minute": 10})
		publish("match-1", 3, "card", map[string]any{"team": "away", "minute": 20})
		publish("match-1", 4, "half_time", nil)
		publish("match-1", 5, "goal", map[string]any{"team": "home", "minute": 50})
		publish("match-1", 6, "full_time", nil)
		publish("match-1", 7, "odds_update", map[string]any{"price": 2.5})

		Eventually(func() (matchstate.MatchRecord, error) {
			return store.GetMatch(context.Background(), "match-1")
		}, "5s", "50ms").Should(HaveField("Status", "full_time"))

		record, err := store.GetMatch(context.Background(), "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(record.HomeScore).To(Equal(2))
		Expect(record.AwayScore).To(Equal(0))

		events, err := store.ListEvents(context.Background(), "match-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(6)) // odds_update never stored
	})

	It("keeps two interleaved matches' state correct under concurrent workers", func() {
		publish("match-a", 1, "kickoff", nil)
		publish("match-b", 1, "kickoff", nil)
		publish("match-a", 2, "goal", map[string]any{"team": "home"})
		publish("match-b", 2, "goal", map[string]any{"team": "away"})
		publish("match-a", 3, "full_time", nil)
		publish("match-b", 3, "full_time", nil)

		Eventually(func() (matchstate.MatchRecord, error) {
			return store.GetMatch(context.Background(), "match-a")
		}, "5s", "50ms").Should(HaveField("Status", "full_time"))
		Eventually(func() (matchstate.MatchRecord, error) {
			return store.GetMatch(context.Background(), "match-b")
		}, "5s", "50ms").Should(HaveField("Status", "full_time"))

		a, err := store.GetMatch(context.Background(), "match-a")
		Expect(err).NotTo(HaveOccurred())
		Expect(a.HomeScore).To(Equal(1))
		Expect(a.AwayScore).To(Equal(0))

		b, err := store.GetMatch(context.Background(), "match-b")
		Expect(err).NotTo(HaveOccurred())
		Expect(b.HomeScore).To(Equal(0))
		Expect(b.AwayScore).To(Equal(1))
	})

	It("finishes applying buffered events before Run returns on cancellation", func() {
		for i := 1; i <= 5; i++ {
			publish("match-c", i, "goal", map[string]any{"team": "home"})
		}
		cancel()

		Eventually(done, "5s", "50ms").Should(BeClosed())

		record, err := store.GetMatch(context.Background(), "match-c")
		Expect(err).NotTo(HaveOccurred())
		Expect(record.HomeScore).To(Equal(5)) // every buffered event still applied
	})
})
