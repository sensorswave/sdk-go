package conformance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// clientSDKCoreCapturedRequest — 派生测试用，捕获 mock server 接收到的请求。
type clientSDKCoreCapturedRequest struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	SourceToken string   `json:"source_token"`
	EventNames  []string `json:"event_names"`
	BatchSize   int      `json:"batch_size"`
}

// TestClientSDKCoreConformance — A 类完全派生测试。
//
// client-sdk-core 在 conformance 中用 in-process httptest echo server 提供一个目标
// HTTP 端点（不模拟错误 / 不控制时序）；输入 events → 输出 mock server 捕获的请求形态
// 是确定性映射，仍属 A 类。派生测试照搬 conformance/adapters/go/client_sdk_core_adapter.go
// 的执行逻辑，与 conformance runner 各持一份是派生模式 DRY 局限的另一实例
// （参 docs/specs/testing-derivation-pilot.md 第 11.3 节）。
func TestClientSDKCoreConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "client-sdk-core")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				EndpointSuffix string `json:"endpoint_suffix"`
				TrackURIPath   string `json:"track_uri_path"`
				Events         []struct {
					AnonID  string `json:"anon_id"`
					LoginID string `json:"login_id"`
					Event   string `json:"event"`
				} `json:"events"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			var mu sync.Mutex
			requests := make([]clientSDKCoreCapturedRequest, 0, 4)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				_ = r.Body.Close()

				var payloads []map[string]any
				require.NoError(t, json.Unmarshal(body, &payloads))

				eventNames := make([]string, 0, len(payloads))
				for _, payload := range payloads {
					if eventName, ok := payload["event"].(string); ok {
						eventNames = append(eventNames, eventName)
					}
				}

				mu.Lock()
				requests = append(requests, clientSDKCoreCapturedRequest{
					Method:      r.Method,
					Path:        r.URL.Path,
					SourceToken: r.Header.Get("SourceToken"),
					EventNames:  eventNames,
					BatchSize:   len(payloads),
				})
				mu.Unlock()

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			cfg := Config{
				TrackURIPath:  input.TrackURIPath,
				Logger:        &noopLogger{},
				FlushInterval: time.Hour,
				HTTPTimeout:   time.Second,
			}
			client, err := NewWithConfig(
				Endpoint(server.URL+input.EndpointSuffix),
				SourceToken("test-token"),
				cfg,
			)
			require.NoError(t, err, "NewWithConfig")

			for _, evt := range input.Events {
				err = client.TrackEvent(
					User{AnonID: evt.AnonID, LoginID: evt.LoginID},
					evt.Event,
					NewProperties(),
				)
				require.NoError(t, err, "TrackEvent %s", evt.Event)
			}

			mu.Lock()
			preClose := len(requests)
			mu.Unlock()

			require.NoError(t, client.Close(), "Close")

			mu.Lock()
			actualRequests := append([]clientSDKCoreCapturedRequest(nil), requests...)
			mu.Unlock()

			actual := map[string]any{
				"pre_close_request_count": float64(preClose),
				"request_count":           float64(len(actualRequests)),
				"requests":                requestsToMaps(actualRequests),
			}
			expected := map[string]any{}
			require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected))

			require.Equal(t, expected, actual, "client capture should match golden")
		})
	}
}

func requestsToMaps(rs []clientSDKCoreCapturedRequest) []any {
	out := make([]any, 0, len(rs))
	for _, r := range rs {
		eventNames := make([]any, 0, len(r.EventNames))
		for _, n := range r.EventNames {
			eventNames = append(eventNames, n)
		}
		out = append(out, map[string]any{
			"method":       r.Method,
			"path":         r.Path,
			"source_token": r.SourceToken,
			"event_names":  eventNames,
			"batch_size":   float64(r.BatchSize),
		})
	}
	return out
}
