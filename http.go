package sensorswave

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

func DefaultHTTPTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

// httpClient is a wrapper around http.Client.
type httpClient struct {
	client *http.Client
}

// requestOpts defines options for HTTP requests.
type requestOpts struct {
	URL           string
	Method        string
	Headers       map[string]string
	Body          []byte
	Retry         int           // default 0, without retry
	Timeout       time.Duration // per-attempt timeout; overall time ~= (Retry+1)*Timeout + backoff, bounded by parent ctx
	YieldInterval time.Duration // default 100ms, min:10ms
}

func newRequestOpts() *requestOpts {
	return &requestOpts{Headers: make(map[string]string), YieldInterval: time.Millisecond * 100}
}

func (o *requestOpts) WithURL(url string) *requestOpts {
	o.URL = url
	return o
}

func (o *requestOpts) WithMethod(method string) *requestOpts {
	m := strings.ToUpper(method) // POST GET PUT...
	switch m {
	case "POST":
		o.Method = m
	case "GET":
		o.Method = m
	case "PUT":
		o.Method = m
	default:
		o.Method = "POST"
	}
	return o
}

func (o *requestOpts) WithHeaders(headers map[string]string) *requestOpts {
	for k, v := range headers {
		o.Headers[k] = v
	}
	return o
}

func (o *requestOpts) WithBody(body []byte) *requestOpts {
	o.Body = body
	return o
}

func (o *requestOpts) WithRetry(retry int) *requestOpts {
	if retry >= 0 {
		o.Retry = retry
	}
	return o
}

func (o *requestOpts) WithTimeout(timeout time.Duration) *requestOpts {
	o.Timeout = timeout
	return o
}

func (o *requestOpts) WithYieldInterval(yieldInterval time.Duration) *requestOpts {
	if yieldInterval > 10*time.Millisecond {
		o.YieldInterval = yieldInterval
	}

	return o
}

// NewHTTPClient creates a new httpClient instance.
// if transport==nil, use net/http default transport
func NewHTTPClient(transport *http.Transport) *httpClient {
	if transport == nil {
		transport = DefaultHTTPTransport()
	}
	return &httpClient{
		client: &http.Client{Transport: transport},
	}
}

// Do sends the request with per-attempt timeout and retry backoff.
// If the caller needs a hard overall deadline, pass a ctx with timeout/deadline.
func (h *httpClient) Do(ctx context.Context, opts *requestOpts) (respBody []byte, httpCode int, err error) {
	for i := 0; i <= opts.Retry; i++ {
		if i > 0 {
			YieldTick(ctx, opts.YieldInterval, i) // yield for retry
		}

		respBody, httpCode, err = h.doWithTimeout(ctx, opts)
		if err != nil {
			// opts.Timeout is per-attempt; retry continues unless parent ctx ends.
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if httpCode == http.StatusOK {
			return
		}
		// continue
	}

	return
}

func (h *httpClient) doWithTimeout(ctx context.Context, opts *requestOpts) ([]byte, int, error) {
	if opts.Timeout > 0 {
		ctxTimeout, cancel := context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
		respBody, httpCode, err := h.do(ctxTimeout, opts)
		if err != nil && ctxTimeout.Err() != nil {
			return respBody, httpCode, ctxTimeout.Err()
		}
		return respBody, httpCode, err
	}
	return h.do(ctx, opts)
}

func (h *httpClient) do(ctx context.Context, opts *requestOpts) ([]byte, int, error) {
	var bodyReader io.Reader
	if opts.Body != nil {
		bodyReader = bytes.NewReader(opts.Body)
	}

	// 1. Create a request using the provided context
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, bodyReader)
	if err != nil {
		return nil, 0, err
	}

	// 2. Set request headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// 3. Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		// Error might be caused by context timeout or cancellation
		return nil, 0, err
	}
	defer resp.Body.Close() // Ensure response body is closed

	// 4. Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// Get is a shortcut for GET requests.
func (h *httpClient) Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error) {
	params := newRequestOpts().WithMethod("GET").WithURL(url).WithHeaders(headers)

	return h.Do(ctx, params)
}

// Post is a shortcut for POST requests.
func (h *httpClient) Post(ctx context.Context, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	params := newRequestOpts().WithMethod("POST").WithURL(url).WithHeaders(headers).WithBody(body)
	return h.Do(ctx, params)
}

func YieldTick(ctx context.Context, d time.Duration, idx int) (ctxDone bool) {
	gap := d * (1 << (idx % 8)) // Backoff algorithm up to 2^7 (128 times)
	tick := time.NewTicker(gap)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			return // Normal return when timed
		case <-ctx.Done():
			ctxDone = true
			return // Context terminated early
		}
	}
}
