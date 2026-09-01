package conformance

import (
	"encoding/json"
	"testing"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// TestUserProfileOpsConformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/user-profile-ops.json
// 不在本测试代码里硬编码任何 spec 字面量或 expected 值。
//
// case 覆盖 5 种 profile 操作（profile_append / profile_union / profile_increment /
// profile_unset / profile_delete），按 operation 调对应 UserPropertyOpts API 构造
// $UserSet 事件，Normalize 后与 golden 比较。
//
// 与 conformance/runners/go/user_profile_ops.py 的 user_profile_comparator 对齐：
// `injected` 模式下从两边比较前删除 `$lib_version`，因该字段由 SDK 运行时注入
// （version.go），不应锁在 golden 中。
func TestUserProfileOpsConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "user-profile-ops")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				Operation       string           `json:"operation"`
				AnonID          string           `json:"anon_id"`
				LoginID         string           `json:"login_id"`
				Properties      map[string]any   `json:"properties"`
				ListProperties  map[string][]any `json:"list_properties"`
				PropertyKeys    []string         `json:"property_keys"`
				Time            int64            `json:"time"`
				TraceID         string           `json:"trace_id"`
				LibMetadataMode string           `json:"lib_metadata_mode"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			evt := buildProfileOpsEvent(t, input)
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

func buildProfileOpsEvent(t *testing.T, input struct {
	Operation       string           `json:"operation"`
	AnonID          string           `json:"anon_id"`
	LoginID         string           `json:"login_id"`
	Properties      map[string]any   `json:"properties"`
	ListProperties  map[string][]any `json:"list_properties"`
	PropertyKeys    []string         `json:"property_keys"`
	Time            int64            `json:"time"`
	TraceID         string           `json:"trace_id"`
	LibMetadataMode string           `json:"lib_metadata_mode"`
}) Event {
	t.Helper()
	evt := NewEvent(input.AnonID, input.LoginID, PseUserSet)
	up := NewUserPropertyOpts()

	switch input.Operation {
	case "profile_append":
		for k, vals := range input.ListProperties {
			up = up.Append(k, vals)
		}

	case "profile_union":
		for k, vals := range input.ListProperties {
			up = up.Union(k, vals)
		}

	case "profile_increment":
		for k, v := range input.Properties {
			up = up.Increment(k, v)
		}

	case "profile_unset":
		for _, k := range input.PropertyKeys {
			up = up.Unset(k)
		}

	case "profile_delete":
		up = up.Delete()

	default:
		t.Fatalf("unknown operation: %s", input.Operation)
	}

	return evt.WithUserPropertyOpts(up)
}
