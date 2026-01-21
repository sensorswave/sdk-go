package sensorswave

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubTransport struct {
	body    []byte
	status  int
	mu      sync.Mutex
	calls   int
	lastReq *http.Request
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastReq = req
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func TestAbCoreLoadRemoteMeta(t *testing.T) {
	resp := httpResponseABLoadRemoteMeta{
		Code: 0,
		Data: ABDataResp{
			Update:     true,
			UpdateTime: 123,
			ABSpecs: []ABSpec{
				{
					ID:        9,
					Key:       "remote_ff",
					Name:      "Remote FF",
					Typ:       int(ABTypExp),
					SubjectID: "LOGIN_ID",
					Enabled:   true,
					Sticky:    false,
					Rules:     map[RuleTypEnum][]Rule{},
					VariantPayloads: map[string]json.RawMessage{
						"1": json.RawMessage(`{"color":"blue"}`),
					},
				},
			},
		},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	transport := &stubTransport{body: body, status: http.StatusOK}
	client := &httpClient{client: &http.Client{Transport: transport}}

	cfg := DefaultConfig("http://example.com", "project-token")
	ffcfg := &ABConfig{
		projectSecret:    "secret",
		sourceToken:      "project-token",
		metaEndpoint:     "http://example.com/api",
		loadMetaInterval: 50 * time.Millisecond,
	}
	cfg.WithABConfig(ffcfg)
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, client)
	core.loadRemoteMeta()

	transport.mu.Lock()
	calls := transport.calls
	req := transport.lastReq
	transport.mu.Unlock()

	require.Equal(t, 1, calls, "expected a single meta request")
	require.NotNil(t, req)
	require.Equal(t, "GET", req.Method)
	// require.Equal(t, "Bearer "+ffcfg.accountAPIToken, req.Header.Get("Authorization"))
	require.Contains(t, req.Header.Get("Authorization"), SignatureAlgorithm)
	require.Equal(t, cfg.sourceToken, req.Header.Get(HeaderSourceToken))

	storage := core.storage()
	require.NotNil(t, storage)
	require.Equal(t, int64(123), storage.UpdateTime)

	spec := core.getABSpec("remote_ff")
	require.NotNil(t, spec)
	require.NotNil(t, spec.VariantValues)
	require.Equal(t, "blue", spec.VariantValues["1"]["color"])
}

func TestNewABCoreUsesCustomMetaURIPath(t *testing.T) {
	cfg := DefaultConfig("http://example.com", "project-token")
	abCfg := DefaultABConfig("project-token", "secret").
		WithMetaEndpoint("http://example.com").
		WithMetaURIPath("/custom/path")
	cfg.WithABConfig(abCfg)
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, nil)
	require.NotNil(t, core)

	loader, ok := cfg.ab.metaLoader.(*HTTPSignatureMetaLoader)
	require.True(t, ok)
	require.Equal(t, "http://example.com", loader.Endpoint)
	require.Equal(t, "/custom/path", loader.URIPath)
}

func TestNewABCoreUsesConfigEndpointWhenMetaEndpointEmpty(t *testing.T) {
	cfg := DefaultConfig("http://example.com", "project-token")
	abCfg := DefaultABConfig("project-token", "secret").
		WithMetaURIPath("/custom/path")
	cfg.WithABConfig(abCfg)
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, nil)
	require.NotNil(t, core)

	loader, ok := cfg.ab.metaLoader.(*HTTPSignatureMetaLoader)
	require.True(t, ok)
	require.Equal(t, "http://example.com", loader.Endpoint)
	require.Equal(t, "/custom/path", loader.URIPath)
}

func TestAbCoreLoadRemoteMetaLoop(t *testing.T) {
	resp := httpResponseABLoadRemoteMeta{
		Code: 0,
		Data: ABDataResp{
			Update:     true,
			UpdateTime: 1,
			ABSpecs: []ABSpec{
				{
					ID:              1,
					Key:             "loop_ff",
					Name:            "Loop FF",
					Typ:             int(ABTypGate),
					SubjectID:       "LOGIN_ID",
					Enabled:         true,
					Rules:           map[RuleTypEnum][]Rule{},
					VariantPayloads: map[string]json.RawMessage{},
				},
			},
		},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	transport := &stubTransport{body: body, status: http.StatusOK}
	client := &httpClient{client: &http.Client{Transport: transport}}

	cfg := DefaultConfig("http://example.com", "project-token")
	ffcfg := &ABConfig{
		projectSecret:    "secret",
		sourceToken:      "project-token",
		metaEndpoint:     "http://example.com/api",
		loadMetaInterval: 10 * time.Millisecond,
	}
	cfg.WithABConfig(ffcfg)
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, client)
	// pre-fill storage so start() doesn't trigger an immediate sync
	core.setStorage(&storage{ABSpecs: map[string]ABSpec{}})

	core.start()
	time.Sleep(35 * time.Millisecond)
	core.stop()

	transport.mu.Lock()
	calls := transport.calls
	transport.mu.Unlock()

	require.GreaterOrEqual(t, calls, 1, "expected loadRemoteMetaLoop to poll at least once")
}

func TestHTTPSignatureMetaLoaderUsesURIPathWhenEndpointHasNoPath(t *testing.T) {
	resp := httpResponseABLoadRemoteMeta{
		Code: 0,
		Data: ABDataResp{
			Update:     true,
			UpdateTime: 1,
			ABSpecs:    []ABSpec{},
		},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	transport := &stubTransport{body: body, status: http.StatusOK}
	client := &httpClient{client: &http.Client{Transport: transport}}

	loader := &HTTPSignatureMetaLoader{
		Endpoint:      "http://example.com",
		URIPath:       "/ab/all4eval",
		SourceToken:   "token",
		ProjectSecret: "secret",
		HTTPClient:    client,
	}

	_, err = loader.LoadMeta()
	require.NoError(t, err)

	transport.mu.Lock()
	req := transport.lastReq
	transport.mu.Unlock()

	require.NotNil(t, req)
	require.Equal(t, "/ab/all4eval", req.URL.Path)
}

func TestAbCoreLoadRemoteMetaHTTPError(t *testing.T) {
	transport := &stubTransport{body: []byte(`{"msg":"fail"}`), status: http.StatusInternalServerError}
	client := &httpClient{client: &http.Client{Transport: transport}}

	cfg := DefaultConfig("http://example.com", "project-token").WithABConfig(&ABConfig{
		projectSecret: "secret",
		sourceToken:   "project-token",
		metaEndpoint:  "http://example.com",
	})
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, client)
	core.loadRemoteMeta()

	require.Nil(t, core.storage(), "storage should remain nil on http error")
}

func TestAbCoreLoadRemoteMetaNoUpdate(t *testing.T) {
	resp := httpResponseABLoadRemoteMeta{
		Code: 0,
		Data: ABDataResp{
			Update:     false,
			UpdateTime: 555,
			ABSpecs: []ABSpec{
				{ID: 1, Key: "spec", VariantPayloads: map[string]json.RawMessage{}},
			},
		},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	transport := &stubTransport{body: body, status: http.StatusOK}
	client := &httpClient{client: &http.Client{Transport: transport}}

	cfg := DefaultConfig("http://example.com", "project-token").WithABConfig(&ABConfig{
		projectSecret:    "secret",
		sourceToken:      "project-token",
		metaEndpoint:     "http://example.com",
		loadMetaInterval: time.Second,
	})
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, client)
	orig := &storage{UpdateTime: 555, ABSpecs: map[string]ABSpec{}}
	core.setStorage(orig)

	core.loadRemoteMeta()

	require.Equal(t, orig, core.storage(), "storage should not be replaced when server reports no update")
}

func TestAbCoreLoadRemoteMetaInvalidPayload(t *testing.T) {
	// invalid top-level json should trigger unmarshal error path
	body := []byte(`{"code":0,"data":{"update":true,"update_time":9,"ab_specs":[{"id":2,"key":"bad_ff","variant_payloads":{"1":{invalid}}}]}`)

	transport := &stubTransport{body: body, status: http.StatusOK}
	client := &httpClient{client: &http.Client{Transport: transport}}

	cfg := DefaultConfig("http://example.com", "project-token").WithABConfig(&ABConfig{
		projectSecret: "secret",
		sourceToken:   "project-token",
		metaEndpoint:  "http://example.com",
	})
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, client)
	orig := &storage{UpdateTime: 1, ABSpecs: map[string]ABSpec{}}
	core.setStorage(orig)

	core.loadRemoteMeta()

	require.Equal(t, orig, core.storage(), "storage should remain unchanged when variant payload unmarshal fails")
}

func TestAbCoreLoadRemoteMetaNoFeatures(t *testing.T) {
	resp := httpResponseABLoadRemoteMeta{
		Code: 0,
		Data: ABDataResp{
			Update:     true,
			UpdateTime: 11,
			ABSpecs:    []ABSpec{},
			ABEnv:      ABEnv{},
		},
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	transport := &stubTransport{body: body, status: http.StatusOK}
	client := &httpClient{client: &http.Client{Transport: transport}}

	cfg := DefaultConfig("http://example.com", "project-token").WithABConfig(&ABConfig{
		projectSecret: "secret",
		sourceToken:   "project-token",
		metaEndpoint:  "http://example.com",
	})
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, client)
	core.loadRemoteMeta()

	require.Nil(t, core.storage(), "storage should stay nil when server returns no feature flags")
}
