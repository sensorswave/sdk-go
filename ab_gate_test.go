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
// gate-021: Gate 过滤器的依赖关系（FilterGate 依赖其他 gate 结果）。
func TestABCoreEvalGateFilter(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "gate_filter.json"))

	t.Run("dependency-fails-rollout-0", func(t *testing.T) {
		core := newTestAbCoreWithStorage(t, store)
		spec := core.getABSpec("FilterGate")
		require.NotNil(t, spec)

		user := User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}
		result, err := core.evalAB(user, spec, 0)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
	})

	if spec, ok := store.ABSpecs["EasyFilterGate"]; ok {
		if rules, ok := spec.Rules["GATE"]; ok && len(rules) > 0 {
			rules[0].Rollout = 100
			spec.Rules["GATE"] = rules
			store.ABSpecs["EasyFilterGate"] = spec
		}
	}

	t.Run("dependency-passes", func(t *testing.T) {
		core := newTestAbCoreWithStorage(t, store)
		spec := core.getABSpec("FilterGate")
		require.NotNil(t, spec)

		user := User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}
		result, err := core.evalAB(user, spec, 0)
		require.NoError(t, err)
		require.True(t, result.CheckFeatureGate())
	})

	t.Run("dependency-fails-condition", func(t *testing.T) {
		core := newTestAbCoreWithStorage(t, store)
		spec := core.getABSpec("FilterGate")
		require.NotNil(t, spec)

		user := User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.9"}}
		result, err := core.evalAB(user, spec, 0)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
	})
}

// gate-022: 复杂条件组合（多条件混合 + 匿名 ID 测试）。
func TestABCoreEvalGateComplicate(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "complicate.json"))

	setRollout := func(s *storage, key string, ruleIndex int, rollout float64) {
		if spec, ok := s.ABSpecs[key]; ok {
			if rules, ok := spec.Rules["GATE"]; ok && len(rules) > ruleIndex {
				rules[ruleIndex].Rollout = rollout
				spec.Rules["GATE"] = rules
				s.ABSpecs[key] = spec
			}
		}
	}

	setRollout(store, "AnonIdTest", 2, 100.0)
	setRollout(store, "AnonIdTest", 3, 100.0)

	core := newTestAbCoreWithStorage(t, store)
	spec := core.getABSpec("AnonIdTest")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "rule1-pass",
			user: User{AnonID: "any", ABUserProperties: Properties{"$app_version": "10.1", "$ip_address": "127.0.0.1"}},
			want: true,
		},
		{
			name: "rule1-fail-version",
			user: User{AnonID: "any", ABUserProperties: Properties{"$app_version": "9.0", "$ip_address": "127.0.0.1", "$device_model": "Pixel"}},
			want: false,
		},
		{
			name: "rule2-pass",
			user: User{AnonID: "any", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Chrome"}},
			want: true,
		},
		{
			name: "rule3-pass",
			user: User{AnonID: "any", LoginID: "login-id-example-2", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox"}},
			want: true,
		},
		{
			name: "rule4-pass",
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 11}},
			want: true,
		},
		{
			name: "rule5-pass",
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 5}},
			want: true,
		},
		{
			name: "rule6-pass",
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 5, "$device_model": "Pixel", "$country": "US"}},
			want: true,
		},
		{
			name: "all-fail",
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 5, "$device_model": "Pixel"}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckFeatureGate())
		})
	}
}

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

