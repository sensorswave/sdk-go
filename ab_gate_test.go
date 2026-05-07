package sensorswave

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// memoryStickyHandler — 跨 ab_*_test.go 共享的内存 sticky 实现。
type memoryStickyHandler struct {
	data map[string]string
}

func (m *memoryStickyHandler) GetStickyResult(key string) (string, error) {
	return m.data[key], nil
}

func (m *memoryStickyHandler) SetStickyResult(key, result string) error {
	m.data[key] = result
	return nil
}

// failStickyHandler — sticky write 失败模拟，用于错误传播测试。
type failStickyHandler struct {
	data map[string]string
}

func (f *failStickyHandler) GetStickyResult(key string) (string, error) {
	return f.data[key], nil
}

func (f *failStickyHandler) SetStickyResult(key string, result string) error {
	return fmt.Errorf("sticky write failed")
}

// 以下 7 个方法对应 test-specs/ab-gate.yaml 中的 unit_test 类 test_id（C 类，
// 公开 API 触不到的内部行为或错误路径），不在派生测试覆盖范围。
// gate-001 ~ gate-018 / gate-020 / gate-023 ~ gate-025 / gate-029 ~ gate-037 /
// gate-039 ~ gate-042 等 conformance 类原 method 已迁移到
// TestABCoreEvaluationConformance（参 ab_core_evaluation_conformance_test.go）。

// gate-019: 不存在的 Gate Key 查询返回 nil。
// gate-026: 错误传播——注入非法规则触发 evalCond 错误。
func TestABCoreEvalRuleErrorPropagation(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "is_true.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Is_True_Gate")
	require.NotNil(t, spec)

	spec.Rules = map[RuleTypEnum][]Rule{
		RuleGate: {
			{
				Conditions: []Condition{{FieldClass: "COMMON", Field: "unknown", Opt: "IS_TRUE"}},
				Rollout:    100,
			},
		},
	}

	_, err := core.evalAB(User{LoginID: "u"}, spec, 0)
	require.Error(t, err)
}

// gate-027: Sticky Handler 失败时返回错误。
func TestABCoreStickyWriteErrorPropagation(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "sticky.json"))
	handler := &failStickyHandler{data: make(map[string]string)}
	core := newTestAbCoreWithStorageAndSticky(t, store, handler)

	spec := core.getABSpec("Sticky_Is_True_Gate")
	require.NotNil(t, spec)

	_, err := core.evalAB(User{LoginID: "user-fail", ABUserProperties: Properties{"is_premium": true}}, spec, 0)
	require.Error(t, err)
}

// gate-028: 边界情况——各种极端输入触发 evalCond 错误或边界。
func TestABCoreEvalCondEdgeCases(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "is_true.json"))
	core := newTestAbCoreWithStorage(t, store)

	t.Run("unknown-common-field", func(t *testing.T) {
		cond := Condition{FieldClass: "COMMON", Field: "unknown", Opt: "IS_TRUE"}
		_, err := core.evalCond(&User{LoginID: "u"}, &cond, "u", 0)
		require.Error(t, err)
	})

	t.Run("ffuser-anon-id", func(t *testing.T) {
		cond := Condition{FieldClass: "FFUSER", Field: "anon_id", Opt: "IS_NOT_NULL"}
		pass, err := core.evalCond(&User{AnonID: "anon"}, &cond, "anon", 0)
		require.NoError(t, err)
		require.True(t, pass)
	})

	t.Run("ffuser-missing", func(t *testing.T) {
		cond := Condition{FieldClass: "FFUSER", Field: "login_id", Opt: "IS_NULL"}
		pass, err := core.evalCond(&User{}, &cond, "", 0)
		require.NoError(t, err)
		require.True(t, pass)
	})

	t.Run("bucket-set-type-error", func(t *testing.T) {
		cond := Condition{FieldClass: "DEFAULT", Field: "salt", Opt: "BUCKET_SET", Value: 123}
		_, err := core.evalCond(&User{LoginID: "u"}, &cond, "u", 0)
		require.Error(t, err)
	})

	t.Run("unknown-operator", func(t *testing.T) {
		cond := Condition{FieldClass: "PROPS", Field: "x", Opt: "NOT_A_REAL_OP", Value: 1}
		_, err := core.evalCond(&User{LoginID: "u"}, &cond, "u", 0)
		require.Error(t, err)
	})
}

