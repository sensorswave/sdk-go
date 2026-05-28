package conformance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sensorswave "github.com/sensorswave/sdk-go"
	"github.com/stretchr/testify/require"
)

// TestABMetaSnapshotBootstrapConformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/ab-meta-snapshot-bootstrap.json
// + spec_file 间接引用 testdata/{config,gate}/ 下的 spec JSON。
// 派生测试在 SubTest 内通过 LoadABSpecs 公开 API 构造 ABCore，
// 调 GetStorageSnapshot 导出快照与 golden 比较。
//
// 执行逻辑照搬 conformance/adapters/go/ab_meta_snapshot_bootstrap_adapter.go，
// 与 conformance runner 保持同义。
func TestABMetaSnapshotBootstrapConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "ab-meta-snapshot-bootstrap")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				SpecFile  string   `json:"spec_file"`
				Operation string   `json:"operation"`
				Languages []string `json:"languages"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			if !abMetaCaseAppliesTo(input.Languages, "go") {
				t.Skip("case does not apply to go")
			}

			if input.Operation == "init_client_without_project_secret" {
				actual := runABMetaMissingProjectSecret()
				var expected map[string]any
				require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected), "unmarshal expected")
				require.Equal(t, expected, actual, "missing project_secret result should match golden")
				return
			}
			if input.Operation == "init_client_loads_meta_once" {
				actual := runABMetaInitLoadsMetaOnce(t, input.SpecFile, c.Raw)
				var expected map[string]any
				require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected), "unmarshal expected")
				require.Equal(t, expected, actual, "initial LoadMeta result should match golden")
				return
			}
			if input.Operation == "meta_request_headers" {
				actual := runABMetaRequestHeaders(t, input.SpecFile, c.Raw)
				var expected map[string]any
				require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected), "unmarshal expected")
				require.Equal(t, expected, actual, "meta request headers should match golden")
				return
			}

			core := loadABCoreFromSpecFile(t, input.SpecFile, nil)

			snapshot, err := core.GetStorageSnapshot()
			require.NoError(t, err, "GetStorageSnapshot")

			var actual map[string]any
			require.NoError(t, json.Unmarshal(snapshot, &actual), "parse snapshot")

			var expected map[string]any
			require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected), "unmarshal expected")

			require.Equal(t, expected, actual, "storage snapshot should match golden")
		})
	}
}

func runABMetaRequestHeaders(t *testing.T, specFile string, rawCase json.RawMessage) map[string]any {
	t.Helper()

	specPath := filepath.Join("..", "testdata", specFile)
	specBytes, err := os.ReadFile(specPath)
	require.NoError(t, err, "read spec file")

	type capturedMetaRequest struct {
		method  string
		path    string
		headers http.Header
	}
	var captured capturedMetaRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.headers = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(specBytes)
	}))
	defer server.Close()

	client, err := sensorswave.NewWithConfig(
		sensorswave.Endpoint(server.URL),
		sensorswave.SourceToken("test-token"),
		sensorswave.Config{
			Logger: &noopLogger{},
			AB: &sensorswave.ABConfig{
				ProjectSecret:    "test-secret",
				MetaLoadInterval: 30 * time.Second,
			},
		},
	)
	require.NoError(t, err, "create client")
	defer func() { require.NoError(t, client.Close()) }()

	sdkVersion := captured.headers.Get("X-SDK-Version")
	authorization := captured.headers.Get("Authorization")
	var input struct {
		AllowedHeaders          []string `json:"allowed_headers"`
		IgnoredTransportHeaders []string `json:"ignored_transport_headers"`
	}
	require.NoError(t, json.Unmarshal(rawCase, &input), "unmarshal header policy")
	ignoredHeaders := stringSet(input.IgnoredTransportHeaders)
	headerKeys := make([]string, 0, len(captured.headers))
	for key := range captured.headers {
		if ignoredHeaders[strings.ToLower(key)] {
			continue
		}
		headerKeys = append(headerKeys, key)
	}
	return map[string]any{
		"ok":                       true,
		"method":                   captured.method,
		"path":                     captured.path,
		"source_token":             captured.headers.Get("SourceToken"),
		"has_only_allowed_headers": hasOnlyAllowedMetaHeaders(headerKeys, input.AllowedHeaders),
		"has_authorization":        authorization != "",
		"has_sdk":                  captured.headers.Get("X-SDK") != "",
		"has_sdk_version":          sdkVersion != "",
		"sdk_version_non_empty":    sdkVersion != "",
		"sdk_version_has_v_prefix": strings.HasPrefix(sdkVersion, "v"),
		"authorization_signed_headers_include_sdk_version": strings.Contains(strings.ToLower(authorization), "x-sdk-version"),
	}
}

func hasOnlyAllowedMetaHeaders(keys []string, allowedHeaders []string) bool {
	allowed := stringSet(allowedHeaders)
	for _, key := range keys {
		if !allowed[strings.ToLower(key)] {
			return false
		}
	}
	return true
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[strings.ToLower(value)] = true
	}
	return out
}

func abMetaCaseAppliesTo(languages []string, current string) bool {
	if len(languages) == 0 {
		return true
	}
	for _, lang := range languages {
		if lang == current {
			return true
		}
	}
	return false
}

func runABMetaMissingProjectSecret() map[string]any {
	_, err := sensorswave.NewWithConfig(
		sensorswave.Endpoint("http://example.com"),
		sensorswave.SourceToken("test-token"),
		sensorswave.Config{
			Logger: &noopLogger{},
			AB:     &sensorswave.ABConfig{},
		},
	)
	if err == nil {
		return map[string]any{"ok": true}
	}
	return map[string]any{
		"ok":    false,
		"error": "project_secret_required",
	}
}

func runABMetaInitLoadsMetaOnce(t *testing.T, specFile string, rawCase json.RawMessage) map[string]any {
	t.Helper()

	specPath := filepath.Join("..", "testdata", specFile)
	specBytes, err := os.ReadFile(specPath)
	require.NoError(t, err, "read spec file")

	var loadCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&loadCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(specBytes)
	}))
	defer server.Close()

	client, err := sensorswave.NewWithConfig(
		sensorswave.Endpoint(server.URL),
		sensorswave.SourceToken("test-token"),
		sensorswave.Config{
			Logger: &noopLogger{},
			AB: &sensorswave.ABConfig{
				ProjectSecret:    "test-secret",
				MetaLoadInterval: 30 * time.Second,
			},
		},
	)
	require.NoError(t, err, "create client")
	defer func() { require.NoError(t, client.Close()) }()

	var input struct {
		Eval struct {
			Key  string `json:"key"`
			User struct {
				AnonID     string         `json:"anon_id"`
				LoginID    string         `json:"login_id"`
				Properties map[string]any `json:"properties"`
			} `json:"user"`
		} `json:"eval"`
	}
	require.NoError(t, json.Unmarshal(rawCase, &input), "unmarshal eval input")

	ready, err := client.CheckFeatureGate(sensorswave.User{
		AnonID:           input.Eval.User.AnonID,
		LoginID:          input.Eval.User.LoginID,
		ABUserProperties: input.Eval.User.Properties,
	}, input.Eval.Key)
	require.NoError(t, err, "check feature gate")

	return map[string]any{
		"ok":         true,
		"load_count": float64(atomic.LoadInt64(&loadCount)),
		"ready":      ready,
	}
}
