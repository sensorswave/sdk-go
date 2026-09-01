package conformance

import (
	"encoding/json"
	"testing"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// TestTrackingCoreConformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/tracking-core.json
// 不在本测试代码里硬编码任何 spec 字面量或 expected 值。
//
// case 含多种 operation（track_event / identify / profile_set / profile_set_once），
// 按 operation 调用对应 SDK API 后做 normalize，再与 golden 比较。
//
// 与 conformance runner 一致：`injected` 模式下从两边比较前删除 `$lib_version`，
// 因该字段由 SDK 运行时注入（version.go），不应锁在 golden 中。
func TestTrackingCoreConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "tracking-core")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				Operation       string         `json:"operation"`
				AnonID          string         `json:"anon_id"`
				LoginID         string         `json:"login_id"`
				Event           string         `json:"event"`
				Properties      map[string]any `json:"properties"`
				Time            int64          `json:"time"`
				TraceID         string         `json:"trace_id"`
				LibMetadataMode string         `json:"lib_metadata_mode"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			evt := buildEventFromCase(t, input)
			evt = evt.WithTime(input.Time).WithTraceID(input.TraceID)
			require.NoError(t, evt.Normalize(), "Normalize")

			actualJSON, err := json.Marshal(evt)
			require.NoError(t, err, "marshal actual")

			var actual map[string]any
			require.NoError(t, json.Unmarshal(actualJSON, &actual), "unmarshal actual")

			var expected map[string]any
			require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected), "unmarshal expected")

			stripLibVersionForInjected(input.LibMetadataMode, actual, expected)

			require.Equal(t, expected, actual, "event JSON should match golden")
		})
	}
}

func buildEventFromCase(t *testing.T, input struct {
	Operation       string         `json:"operation"`
	AnonID          string         `json:"anon_id"`
	LoginID         string         `json:"login_id"`
	Event           string         `json:"event"`
	Properties      map[string]any `json:"properties"`
	Time            int64          `json:"time"`
	TraceID         string         `json:"trace_id"`
	LibMetadataMode string         `json:"lib_metadata_mode"`
}) Event {
	t.Helper()
	switch input.Operation {
	case "track_event":
		evt := NewEvent(input.AnonID, input.LoginID, input.Event)
		if input.Properties != nil {
			props := NewProperties()
			for k, v := range input.Properties {
				props.Set(k, v)
			}
			evt = evt.WithProperties(props)
		}
		return evt

	case "identify":
		return NewEvent(input.AnonID, input.LoginID, "$Identify")

	case "profile_set":
		return profileEvent(input.AnonID, input.LoginID, input.Properties, false)

	case "profile_set_once":
		return profileEvent(input.AnonID, input.LoginID, input.Properties, true)

	default:
		t.Fatalf("unknown operation: %s", input.Operation)
		return Event{}
	}
}

func profileEvent(anonID, loginID string, props map[string]any, once bool) Event {
	evt := NewEvent(anonID, loginID, "$UserSet")
	up := NewUserPropertyOpts()
	for k, v := range props {
		if once {
			up = up.SetOnce(k, v)
		} else {
			up = up.Set(k, v)
		}
	}
	return evt.WithUserPropertyOpts(up)
}
