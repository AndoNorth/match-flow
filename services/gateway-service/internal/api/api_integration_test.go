//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/api"
	"github.com/AndoNorth/match-flow/services/gateway-service/internal/matchclient"
)

// This test deliberately runs the REAL match-service binary as an OS
// subprocess rather than wiring its handler in-process: match-service's
// gRPC server, store, and event pipeline all live under its own
// internal/ tree, which Go's internal-import rule makes unreachable
// from gateway-service's own internal/ tree (see the design spec's
// canonical.go duplication note for the same constraint biting
// elsewhere). Driving the whole thing over the network is also more
// faithful to production - the two services actually talk over the
// network, never via a shared Go import.
const matchServiceAddr = "http://localhost:19082"

var _ = Describe("Gateway REST routes against a real Match Service process", func() {
	It("round-trips a match end-to-end: Redis publish -> match-service -> gRPC -> Gateway REST", func() {
		ctx := context.Background()

		pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("matchflow"),
			tcpostgres.WithUsername("matchflow"),
			tcpostgres.WithPassword("matchflow"),
		)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = pgContainer.Terminate(context.Background()) })

		dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())

		redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = redisContainer.Terminate(context.Background()) })

		redisConnStr, err := redisContainer.ConnectionString(ctx)
		Expect(err).NotTo(HaveOccurred())

		binPath := buildMatchServiceBinary(ctx)

		//nolint:gosec // fixed argv, no user input; test-only subprocess
		cmd := exec.CommandContext(ctx, binPath)
		cmd.Env = append(os.Environ(),
			"POSTGRES_DSN="+dsn,
			"REDIS_URL="+redisConnStr,
			"PORT=19082",
			"MATCH_SERVICE_WORKERS=2",
		)
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter
		Expect(cmd.Start()).To(Succeed())
		DeferCleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})

		Eventually(func() int {
			resp, err := http.Get(matchServiceAddr + "/healthz")
			if err != nil {
				return 0
			}
			defer func() { _ = resp.Body.Close() }()
			return resp.StatusCode
		}, 20*time.Second, 200*time.Millisecond).Should(Equal(http.StatusOK))

		opts, err := redis.ParseURL(redisConnStr)
		Expect(err).NotTo(HaveOccurred())
		redisClient := redis.NewClient(opts)
		DeferCleanup(func() { _ = redisClient.Close() })

		event := map[string]any{
			"payload":     map[string]any{},
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"ingested_at": time.Now().UTC().Format(time.RFC3339),
			"match_id":    "match-1",
			"type":        "kickoff",
			"provider":    "integration-test",
			"sequence":    1,
		}
		payload, err := json.Marshal(event)
		Expect(err).NotTo(HaveOccurred())
		Expect(redisClient.Publish(ctx, "matchflow:events", payload).Err()).NotTo(HaveOccurred())

		client := matchclient.New(matchServiceAddr, http.DefaultClient)

		gatewayMux := http.NewServeMux()
		gatewayAPI := humaAPIForTest(gatewayMux)
		api.Register(gatewayAPI, client)
		gatewayServer := httptest.NewServer(gatewayMux)
		DeferCleanup(gatewayServer.Close)

		var body map[string]any
		Eventually(func() int {
			resp, err := http.Get(gatewayServer.URL + "/matches/match-1")
			if err != nil {
				return 0
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				return resp.StatusCode
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			return resp.StatusCode
		}, 10*time.Second, 200*time.Millisecond).Should(Equal(http.StatusOK))

		Expect(body["status"]).To(Equal("live"))
	})
})

// buildMatchServiceBinary compiles match-service's own binary by full
// import path (not a relative path) so the build works regardless of
// this test's working directory, and returns the path to the built
// executable.
func buildMatchServiceBinary(ctx context.Context) string {
	binPath := GinkgoT().TempDir() + "/match-service"
	//nolint:gosec // fixed argv, no user input; test-only build helper
	cmd := exec.CommandContext(
		ctx, "go", "build", "-o", binPath,
		"github.com/AndoNorth/match-flow/services/match-service/cmd/match-service",
	)
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(out))
	return binPath
}
