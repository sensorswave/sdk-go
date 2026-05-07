package conformance

import (
	"encoding/json"
	"testing"

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
				SpecFile string `json:"spec_file"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

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
