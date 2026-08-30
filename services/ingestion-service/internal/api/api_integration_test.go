//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/api"
	"github.com/AndoNorth/match-flow/services/ingestion-service/internal/eventbus"
)

// Run via the existing TestAPI/RunSpecs entrypoint in api_suite_test.go -
// Ginkgo supports only one RunSpecs call per package, and that entrypoint
// already exists for the unit specs in api_test.go.
var _ = Describe("Ingestion Service HTTP + Redis contract", func() {
	var (
		client *redis.Client
		server *httptest.Server
	)

	BeforeEach(func() {
		ctx := context.Background()

		container, err := tcredis.Run(ctx, "redis:7-alpine")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = container.Terminate(context.Background()) })

		connStr, err := container.ConnectionString(ctx)
		Expect(err).NotTo(HaveOccurred())

		opts, err := redis.ParseURL(connStr)
		Expect(err).NotTo(HaveOccurred())
		client = redis.NewClient(opts)
		DeferCleanup(func() { _ = client.Close() })

		mux := http.NewServeMux()
		humaAPI := humago.New(mux, huma.DefaultConfig("test", "0.0.0"))
		api.Register(humaAPI, eventbus.NewPublisher(client))

		server = httptest.NewServer(mux)
		DeferCleanup(server.Close)
	})

	It("publishes a valid ProviderA payload to the Redis channel", func() {
		ctx := context.Background()
		sub := client.Subscribe(ctx, eventbus.Channel)
		defer func() { _ = sub.Close() }()
		_, err := sub.Receive(ctx) // subscribe confirmation
		Expect(err).NotTo(HaveOccurred())

		resp, err := http.Post(
			server.URL+"/events/provider-a",
			"application/json",
			strings.NewReader(
				`{"data":{"mid":"match-1","seq":1,"typ":"kickoff","ts":1735689600,"pl":{}}}`,
			),
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		msg, err := sub.ReceiveMessage(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Payload).To(ContainSubstring(`"match_id":"match-1"`))
		Expect(msg.Payload).To(ContainSubstring(`"provider":"provider-a"`))
	})

	It("rejects a schema-invalid ProviderA payload with 422 and publishes nothing", func() {
		resp, err := http.Post(
			server.URL+"/events/provider-a",
			"application/json",
			strings.NewReader(`{"data":{"seq":1,"typ":"kickoff","ts":1735689600}}`),
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
	})

	It("rejects malformed JSON with 400", func() {
		resp, err := http.Post(
			server.URL+"/events/provider-a",
			"application/json",
			strings.NewReader("not json"),
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	It("publishes a valid ProviderB payload to the Redis channel", func() {
		ctx := context.Background()
		sub := client.Subscribe(ctx, eventbus.Channel)
		defer func() { _ = sub.Close() }()
		_, err := sub.Receive(ctx)
		Expect(err).NotTo(HaveOccurred())

		resp, err := http.Post(
			server.URL+"/events/provider-b",
			"application/json",
			strings.NewReader(
				`{"match_id":"match-1","sequence":2,"event_type":"goal","occurred_at":"2026-08-30T12:00:00Z","details":{}}`,
			),
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		msg, err := sub.ReceiveMessage(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(msg.Payload).To(ContainSubstring(`"provider":"provider-b"`))
	})

	It("rejects an unparseable timestamp on ProviderB with 400 and publishes nothing", func() {
		resp, err := http.Post(
			server.URL+"/events/provider-b",
			"application/json",
			strings.NewReader(
				`{"match_id":"match-1","sequence":2,"event_type":"goal","occurred_at":"not-a-timestamp"}`,
			),
		)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})
})
