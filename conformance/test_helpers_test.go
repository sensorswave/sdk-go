// Package conformance hosts derived A-class SDK tests that consume harness fixtures+goldens.
//
// Files here import sdk-go via a dot import so they read like the in-package tests they
// replaced; the trade-off is acceptable inside a single test sub-package whose only job is
// exercising the public API across all 8 capabilities. Cross-capability helpers
// (loadConformance, stripLibVersion, noopLogger) live here; capability-specific helpers
// live in that capability's *_test.go.
package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// noopLogger 满足 sensorswave.Logger 接口（与根包 ab_test_helpers_test.go 同形态；conformance
// 子包不允许反向依赖根包的 unexported 符号，故本地复制）。仅 ab-core / client-sdk-core 派生测试用到。
type noopLogger struct{}

func (n *noopLogger) Debugf(string, ...any) {}
func (n *noopLogger) Infof(string, ...any)  {}
func (n *noopLogger) Warnf(string, ...any)  {}
func (n *noopLogger) Errorf(string, ...any) {}

// ConformanceCase 是 fixture 里单个 case 的视图。
// ID 与 TestID 为常用字段被预解析；其余字段保留在 Raw 里，由调用方按 capability 自行 unmarshal。
type ConformanceCase struct {
	ID     string          `json:"id"`
	TestID string          `json:"test_id,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

// loadConformance 加载指定 capability 的 fixture 与 golden，
// 返回 cases 顺序切片（保留 fixture 内顺序）与按 case ID 索引的 expected JSON。
//
// 路径硬编码为 testdata/conformance/{fixtures,golden}/<capability>.json。
func loadConformance(t *testing.T, capability string) ([]ConformanceCase, map[string]json.RawMessage) {
	t.Helper()

	fixturesPath := filepath.Join("..", "testdata", "conformance", "fixtures", capability+".json")
	goldenPath := filepath.Join("..", "testdata", "conformance", "golden", capability+".json")

	fixtureRaw, err := os.ReadFile(fixturesPath)
	require.NoError(t, err, "read fixtures: %s", fixturesPath)
	goldenRaw, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "read golden: %s", goldenPath)

	var fixtureWrap struct {
		CapabilityID string            `json:"capability_id"`
		Cases        []json.RawMessage `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(fixtureRaw, &fixtureWrap), "unmarshal fixtures: %s", fixturesPath)
	require.Equal(t, capability, fixtureWrap.CapabilityID,
		"fixture capability_id mismatch")

	cases := make([]ConformanceCase, 0, len(fixtureWrap.Cases))
	for i, raw := range fixtureWrap.Cases {
		var meta struct {
			ID     string `json:"id"`
			TestID string `json:"test_id,omitempty"`
		}
		require.NoError(t, json.Unmarshal(raw, &meta), "unmarshal case[%d] meta", i)
		require.NotEmpty(t, meta.ID, "case[%d] missing id", i)
		cases = append(cases, ConformanceCase{
			ID:     meta.ID,
			TestID: meta.TestID,
			Raw:    raw,
		})
	}

	var goldenWrap struct {
		CapabilityID string `json:"capability_id"`
		Cases        []struct {
			ID       string          `json:"id"`
			Expected json.RawMessage `json:"expected"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(goldenRaw, &goldenWrap), "unmarshal golden: %s", goldenPath)
	require.Equal(t, capability, goldenWrap.CapabilityID,
		"golden capability_id mismatch")

	expectedByID := make(map[string]json.RawMessage, len(goldenWrap.Cases))
	for _, c := range goldenWrap.Cases {
		require.NotEmpty(t, c.ID, "golden case missing id")
		expectedByID[c.ID] = c.Expected
	}

	// 双向对齐：每个 fixture case 必须有对应 golden expected
	for _, c := range cases {
		_, ok := expectedByID[c.ID]
		require.True(t, ok, "fixture case %q has no matching golden expected", c.ID)
	}

	return cases, expectedByID
}

// stripLibVersion 删除 properties.$lib_version（运行时变量，由 SDK 注入；与 conformance
// runner 的 lib_version_strip_comparator 对齐）。
func stripLibVersion(m map[string]any) {
	props, ok := m["properties"].(map[string]any)
	if !ok {
		return
	}
	delete(props, "$lib_version")
}

// stripLibVersionForInjected 在 lib_metadata_mode=="injected" 时从两边都删除 $lib_version；
// 否则不动两个 map。
func stripLibVersionForInjected(libMode string, actual, expected map[string]any) {
	if libMode != "injected" {
		return
	}
	stripLibVersion(actual)
	stripLibVersion(expected)
}
