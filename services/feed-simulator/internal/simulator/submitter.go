package simulator

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

const submitTimeout = 5 * time.Second

// Submitter POSTs an already-encoded provider payload to route on
// Ingestion Service. Injected into Runner so tests can substitute a
// fake or an httptest.Server-backed instance instead of a live process.
type Submitter interface {
	Submit(ctx context.Context, route string, payload []byte) error
}

// HTTPSubmitter is the production Submitter, POSTing to baseURL+route.
type HTTPSubmitter struct {
	client  *http.Client
	baseURL string
}

// NewHTTPSubmitter builds an HTTPSubmitter targeting baseURL (e.g.
// "http://localhost:8081", no trailing slash).
func NewHTTPSubmitter(baseURL string) *HTTPSubmitter {
	return &HTTPSubmitter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: submitTimeout},
	}
}

// Submit POSTs payload to s.baseURL+route.
func (s *HTTPSubmitter) Submit(ctx context.Context, route string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+route, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("submit event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("submit event: unexpected status %d", resp.StatusCode)
	}
	return nil
}
