package conformance

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// TestPropertyDatetimeISO8601UTCConformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/property-datetime-iso8601-utc.json
// （由 backend-sdk-harness 通过 scripts/sync_ab_testdata.py --conformance-data 同步）。
// 不在本测试代码里硬编码任何 spec 字面量或 expected 值。
//
// fixture 字符串语义为 UTC 墙上时间；测试代码（与 conformance adapter 一致）把
// `native_time_properties` / `native_time_list_properties` 列出的 key 解析为
// time.Time（UTC），交给 Normalize 输出 ISO8601。其他字符串保持原样。
//
// 与 conformance/runners/go/property_datetime_iso8601_utc.py 一致：`injected` 模式
// 下从两边比较前删除 `$lib_version`，因该字段由 SDK 运行时注入（version.go），不应
// 锁在 golden 中。
//
// 详见 docs/specs/testing-strategy.md 第 4.1 / 5 节。
func TestPropertyDatetimeISO8601UTCConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "property-datetime-iso8601-utc")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				Operation                string           `json:"operation"`
				AnonID                   string           `json:"anon_id"`
				LoginID                  string           `json:"login_id"`
				Event                    string           `json:"event"`
				Properties               map[string]any   `json:"properties"`
				ListProperties           map[string][]any `json:"list_properties"`
				NativeTimeProperties     []string         `json:"native_time_properties"`
				NativeTimeListProperties []string         `json:"native_time_list_properties"`
				Time                     int64            `json:"time"`
				TraceID                  string           `json:"trace_id"`
				LibMetadataMode          string           `json:"lib_metadata_mode"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			evt := buildDatetimeEvent(t, input)
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

func buildDatetimeEvent(t *testing.T, input struct {
	Operation                string           `json:"operation"`
	AnonID                   string           `json:"anon_id"`
	LoginID                  string           `json:"login_id"`
	Event                    string           `json:"event"`
	Properties               map[string]any   `json:"properties"`
	ListProperties           map[string][]any `json:"list_properties"`
	NativeTimeProperties     []string         `json:"native_time_properties"`
	NativeTimeListProperties []string         `json:"native_time_list_properties"`
	Time                     int64            `json:"time"`
	TraceID                  string           `json:"trace_id"`
	LibMetadataMode          string           `json:"lib_metadata_mode"`
}) Event {
	t.Helper()

	props := nativeProps(t, input.Properties, input.NativeTimeProperties)
	listProps := nativeListProps(t, input.ListProperties, input.NativeTimeListProperties)

	switch input.Operation {
	case "track_event":
		evt := NewEvent(input.AnonID, input.LoginID, input.Event)
		p := NewProperties()
		for k, v := range props {
			p.Set(k, v)
		}
		return evt.WithProperties(p)

	case "profile_set":
		return profileFromNativeMap(input.AnonID, input.LoginID, props, UserSetTypeSet, false)

	case "profile_set_once":
		return profileFromNativeMap(input.AnonID, input.LoginID, props, UserSetTypeSetOnce, true)

	case "profile_append":
		return profileFromNativeListMap(input.AnonID, input.LoginID, listProps, UserSetTypeAppend, false)

	case "profile_union":
		return profileFromNativeListMap(input.AnonID, input.LoginID, listProps, UserSetTypeUnion, true)

	default:
		t.Fatalf("unknown operation: %s", input.Operation)
		return Event{}
	}
}

// nativeProps 把 fixture 中 native_time_properties 列出的 key 从字符串解析为 time.Time（UTC）。
func nativeProps(t *testing.T, raw map[string]any, nativeKeys []string) map[string]any {
	t.Helper()
	nativeSet := map[string]bool{}
	for _, k := range nativeKeys {
		nativeSet[k] = true
	}
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		if nativeSet[k] {
			s, ok := v.(string)
			require.True(t, ok, "native time property %q must be string", k)
			parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000", s, time.UTC)
			require.NoError(t, err, "parse native time %q", k)
			out[k] = parsed
			continue
		}
		out[k] = v
	}
	return out
}

func nativeListProps(t *testing.T, raw map[string][]any, nativeKeys []string) map[string][]any {
	t.Helper()
	nativeSet := map[string]bool{}
	for _, k := range nativeKeys {
		nativeSet[k] = true
	}
	out := make(map[string][]any, len(raw))
	for k, vals := range raw {
		if !nativeSet[k] {
			out[k] = vals
			continue
		}
		converted := make([]any, 0, len(vals))
		for _, v := range vals {
			s, ok := v.(string)
			require.True(t, ok, "native time list property %q element must be string", k)
			parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000", s, time.UTC)
			require.NoError(t, err, "parse native time list %q", k)
			converted = append(converted, parsed)
		}
		out[k] = converted
	}
	return out
}

func profileFromNativeMap(anonID, loginID string, props map[string]any, setType string, once bool) Event {
	evt := NewEvent(anonID, loginID, PseUserSet)
	up := NewUserPropertyOpts()
	for k, v := range props {
		if once {
			up = up.SetOnce(k, v)
		} else {
			up = up.Set(k, v)
		}
	}
	evt = evt.WithUserPropertyOpts(up)
	if evt.Properties == nil {
		evt.Properties = NewProperties()
	}
	evt.Properties.Set(PspUserSetType, setType)
	return evt
}

func profileFromNativeListMap(anonID, loginID string, listProps map[string][]any, setType string, union bool) Event {
	evt := NewEvent(anonID, loginID, PseUserSet)
	up := NewUserPropertyOpts()
	for k, vals := range listProps {
		if union {
			up = up.Union(k, vals)
		} else {
			up = up.Append(k, vals)
		}
	}
	evt = evt.WithUserPropertyOpts(up)
	if evt.Properties == nil {
		evt.Properties = NewProperties()
	}
	evt.Properties.Set(PspUserSetType, setType)
	return evt
}
