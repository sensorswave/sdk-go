package conformance

import (
	"encoding/json"
	"testing"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// TestComplexPropertyInputConventionsConformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/complex-property-input-conventions.json
// （由 backend-sdk-harness 通过 scripts/sync_ab_testdata.py --conformance-data 同步）。
// 不在本测试代码里硬编码任何 spec 字面量或 expected 值。
//
// case 覆盖 5 种 operation（track_event / profile_set / profile_set_once /
// profile_append / profile_union）下复杂属性 pass-through 行为：嵌套对象、
// 对象数组、混合标量/对象的列表、空数组等。
//
// 与 conformance runner 一致：`injected` 模式下从两边比较前删除 `$lib_version`，
// 因该字段由 SDK 运行时注入（version.go），不应锁在 golden 中。
//
// 详见 docs/specs/testing-strategy.md 第 4.1 / 5 节。
func TestComplexPropertyInputConventionsConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "complex-property-input-conventions")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				Operation       string           `json:"operation"`
				AnonID          string           `json:"anon_id"`
				LoginID         string           `json:"login_id"`
				Event           string           `json:"event"`
				Properties      map[string]any   `json:"properties"`
				ListProperties  map[string][]any `json:"list_properties"`
				Time            int64            `json:"time"`
				TraceID         string           `json:"trace_id"`
				LibMetadataMode string           `json:"lib_metadata_mode"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			evt := buildComplexPropertyEvent(t, input)
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

func buildComplexPropertyEvent(t *testing.T, input struct {
	Operation       string           `json:"operation"`
	AnonID          string           `json:"anon_id"`
	LoginID         string           `json:"login_id"`
	Event           string           `json:"event"`
	Properties      map[string]any   `json:"properties"`
	ListProperties  map[string][]any `json:"list_properties"`
	Time            int64            `json:"time"`
	TraceID         string           `json:"trace_id"`
	LibMetadataMode string           `json:"lib_metadata_mode"`
}) Event {
	t.Helper()

	switch input.Operation {
	case "track_event":
		evt := NewEvent(input.AnonID, input.LoginID, input.Event)
		props := NewProperties()
		for k, v := range input.Properties {
			props.Set(k, v)
		}
		return evt.WithProperties(props)

	case "profile_set":
		return profileFromMap(input.AnonID, input.LoginID, input.Properties, UserSetTypeSet, false)

	case "profile_set_once":
		return profileFromMap(input.AnonID, input.LoginID, input.Properties, UserSetTypeSetOnce, true)

	case "profile_append":
		return profileFromListMap(input.AnonID, input.LoginID, input.ListProperties, UserSetTypeAppend, false)

	case "profile_union":
		return profileFromListMap(input.AnonID, input.LoginID, input.ListProperties, UserSetTypeUnion, true)

	default:
		t.Fatalf("unknown operation: %s", input.Operation)
		return Event{}
	}
}

func profileFromMap(anonID, loginID string, props map[string]any, setType string, once bool) Event {
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

func profileFromListMap(anonID, loginID string, listProps map[string][]any, setType string, union bool) Event {
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
