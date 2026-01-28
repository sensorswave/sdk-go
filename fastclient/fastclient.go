package fastclient

import (
	"context"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type Config struct {
	MaxConnsPerHost     int
	MaxIdleConnDuration time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	MaxTimeout          time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		MaxConnsPerHost:     100,
		MaxIdleConnDuration: 10 * time.Second,
		ReadTimeout:         1500 * time.Millisecond,
		WriteTimeout:        1500 * time.Millisecond,
		MaxTimeout:          3000 * time.Millisecond,
	}
}

// HTTPClient High-performance HTTP client
type HTTPClient struct {
	client   *fasthttp.Client
	reqPool  sync.Pool // Request object pool
	respPool sync.Pool // Response object pool
	timeout  time.Duration
}

// NewHTTPClient creates a client instance
// timeout=0 means no timeout limit
func NewHTTPClient(cfg *Config) *HTTPClient {
	return &HTTPClient{
		client: &fasthttp.Client{
			MaxConnsPerHost:     cfg.MaxConnsPerHost,
			MaxIdleConnDuration: cfg.MaxIdleConnDuration, // Recyclable idle connection timeout
			ReadTimeout:         cfg.ReadTimeout,
			WriteTimeout:        cfg.WriteTimeout,
		},
		reqPool: sync.Pool{
			New: func() interface{} { return fasthttp.AcquireRequest() },
		},
		respPool: sync.Pool{
			New: func() interface{} { return fasthttp.AcquireResponse() },
		},
		timeout: cfg.MaxTimeout,
	}
}

// DoRequest executes a general request
func (c *HTTPClient) DoRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	// Acquire request/response from object pool
	req := c.reqPool.Get().(*fasthttp.Request)
	defer c.reqPool.Put(req)
	resp := c.respPool.Get().(*fasthttp.Response)
	defer c.respPool.Put(resp)

	// Initialize request
	req.Reset()
	resp.Reset()

	req.Header.SetHost("localhost")
	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.SetBody(body)
	}

	// Listen for context cancellation signal
	done := make(chan struct{})
	defer close(done)
	var err error
	go func() {
		select {
		case <-ctx.Done(): // Context canceled
			fasthttp.ReleaseRequest(req)
			err = ctx.Err()
		case <-done: // Normal exit
		}
	}()

	// Execute request
	if err = c.client.DoRedirects(req, resp, 10); err != nil {
		return nil, 0, err
	}

	// Copy response body to avoid memory pollution
	bodyCopy := make([]byte, len(resp.Body()))
	copy(bodyCopy, resp.Body())
	return bodyCopy, resp.StatusCode(), nil
}

// Get is a shortcut for GET requests.
func (c *HTTPClient) Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error) {
	return c.DoRequest(ctx, fasthttp.MethodGet, url, headers, nil)
}

// Post is a shortcut for POST requests.
func (c *HTTPClient) Post(ctx context.Context, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	return c.DoRequest(ctx, fasthttp.MethodPost, url, headers, body)
}

// PostForm sends a POST request with form data.
func (c *HTTPClient) PostForm(ctx context.Context, url string, headers map[string]string, params map[string]string) ([]byte, int, error) {
	args := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(args)

	args.Reset()
	for k, v := range params {
		args.Set(k, v)
	}
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	return c.DoRequest(ctx, fasthttp.MethodPost, url,
		headers,
		args.QueryString())
}

// PostJSON JSON format POST
func (c *HTTPClient) PostJSON(ctx context.Context, url string, headers map[string]string, jsonbody []byte) ([]byte, int, error) {
	headers["Content-Type"] = "application/json"
	return c.DoRequest(ctx, fasthttp.MethodPost, url,
		headers,
		jsonbody)
}

// Put Put shortcut method
func (c *HTTPClient) Put(ctx context.Context, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	return c.DoRequest(ctx, fasthttp.MethodPut, url, headers, body)
}
