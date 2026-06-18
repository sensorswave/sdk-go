package sensorswave

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

type sequenceRoundTripper struct {
	mu       sync.Mutex
	statuses []int
	calls    int
}

func (rt *sequenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.calls++
	status := http.StatusOK
	if len(rt.statuses) > 0 {
		status = rt.statuses[0]
		rt.statuses = rt.statuses[1:]
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(http.NoBody),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func (rt *sequenceRoundTripper) Calls() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.calls
}

func TestHTTPClientDoesNotRetryUnauthorized(t *testing.T) {
	transport := &sequenceRoundTripper{statuses: []int{http.StatusUnauthorized, http.StatusOK}}
	client := &httpClient{client: &http.Client{Transport: transport}}
	opts := newRequestOpts().
		WithMethod("POST").
		WithURL("https://collector.example.com/in/track").
		WithBody([]byte("{}")).
		WithRetry(2).
		WithYieldInterval(11 * time.Millisecond)

	_, code, err := client.Do(context.Background(), opts)

	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", code, http.StatusUnauthorized)
	}
	if transport.Calls() != 1 {
		t.Fatalf("request count = %d, want 1", transport.Calls())
	}
}
