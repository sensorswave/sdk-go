package sensorswave

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestABCoreEvaluateDecisionRuleID(t *testing.T) {
	t.Run("matched rule reports decision rule", func(t *testing.T) {
		spec := newTraceTestGateSpec("trace_pass", 100, false, []Condition{
			{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
		})
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))

		result, err := core.Evaluate(User{LoginID: "user-pass"}, spec.Key)
		require.NoError(t, err)
		require.True(t, result.CheckFeatureGate())
		require.NotNil(t, result.DecisionRuleID)
		require.Equal(t, "rule-"+spec.Key, *result.DecisionRuleID)
	})

	t.Run("gate rollout rejection reports the matched gate rule as decision rule", func(t *testing.T) {
		spec := newTraceTestGateSpec("trace_rollout_fail", 50, false, []Condition{
			{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
		})
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))
		user := findTraceTestUser(t, "rule-"+spec.Key, 50, false)

		result, err := core.Evaluate(user, spec.Key)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
		require.NotNil(t, result.DecisionRuleID)
		require.Equal(t, "rule-"+spec.Key, *result.DecisionRuleID)
	})

	t.Run("zero rollout reports decision rule when gate conditions match", func(t *testing.T) {
		spec := newTraceTestGateSpec("trace_zero_rollout", 0, false, []Condition{
			{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
		})
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))

		result, err := core.Evaluate(User{LoginID: "zero-rollout-user"}, spec.Key)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
		require.NotNil(t, result.DecisionRuleID)
		require.Equal(t, "rule-"+spec.Key, *result.DecisionRuleID)
	})

	t.Run("zero rollout with unmatched conditions keeps empty decision rule", func(t *testing.T) {
		spec := newTraceTestGateSpec("trace_zero_rollout_unmatched", 0, false, []Condition{
			{FieldClass: "FFUSER", Field: "login_id", Opt: "EQ", Value: "vip-user"},
		})
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))

		result, err := core.Evaluate(User{LoginID: "plain-user"}, spec.Key)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
		require.Nil(t, result.DecisionRuleID)
	})

	t.Run("no matched gate rule reports empty decision rule", func(t *testing.T) {
		spec := newTraceTestGateSpec("trace_default", 100, false, []Condition{
			{FieldClass: "FFUSER", Field: "login_id", Opt: "EQ", Value: "vip-user"},
		})
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))

		result, err := core.Evaluate(User{LoginID: "plain-user"}, spec.Key)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
		require.Nil(t, result.DecisionRuleID)
	})

	t.Run("later matched gate rule still wins after earlier zero rollout rule", func(t *testing.T) {
		spec := ABSpec{
			ID:             42,
			Key:            "trace_gate_chain",
			Name:           "trace_gate_chain",
			Typ:            int(ABTypGate),
			Traffic:        "SERVER",
			SubjectID:      "login_id",
			Enabled:        true,
			Sticky:         false,
			Salt:           "trace-gate-chain-salt",
			Version:        1,
			DisableImpress: false,
			Rules: map[RuleTypEnum][]Rule{
				RuleGate: {
					{
						ID:      "rule-zero",
						Name:    "Rule Zero",
						Salt:    "rule-zero",
						Rollout: 0,
						Conditions: []Condition{
							{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
						},
					},
					{
						ID:      "rule-pass",
						Name:    "Rule Pass",
						Salt:    "rule-pass",
						Rollout: 100,
						Conditions: []Condition{
							{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
						},
					},
				},
			},
			VariantPayloads: map[string]json.RawMessage{},
		}
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))

		result, err := core.Evaluate(User{LoginID: "chain-user"}, spec.Key)
		require.NoError(t, err)
		require.True(t, result.CheckFeatureGate())
		require.NotNil(t, result.DecisionRuleID)
		require.Equal(t, "rule-pass", *result.DecisionRuleID)
	})

	t.Run("later matched gate rule still wins after earlier matched gate rollout rejection", func(t *testing.T) {
		spec := ABSpec{
			ID:             53,
			Key:            "trace_gate_rollout_then_pass",
			Name:           "trace_gate_rollout_then_pass",
			Typ:            int(ABTypGate),
			Traffic:        "SERVER",
			SubjectID:      "login_id",
			Enabled:        true,
			Sticky:         false,
			Salt:           "trace-gate-rollout-then-pass-salt",
			Version:        1,
			DisableImpress: false,
			Rules: map[RuleTypEnum][]Rule{
				RuleGate: {
					{
						ID:      "rule-fail",
						Name:    "Rule Fail",
						Salt:    "rule-fail",
						Rollout: 50,
						Conditions: []Condition{
							{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
						},
					},
					{
						ID:      "rule-pass",
						Name:    "Rule Pass",
						Salt:    "rule-pass",
						Rollout: 100,
						Conditions: []Condition{
							{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
						},
					},
				},
			},
			VariantPayloads: map[string]json.RawMessage{},
		}
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))
		user := findTraceTestUser(t, "rule-fail", 50, false)

		result, err := core.Evaluate(user, spec.Key)
		require.NoError(t, err)
		require.True(t, result.CheckFeatureGate())
		require.NotNil(t, result.DecisionRuleID)
		require.Equal(t, "rule-pass", *result.DecisionRuleID)
	})

	t.Run("traffic rejection reports decision rule because it is the final decision", func(t *testing.T) {
		spec := ABSpec{
			ID:             52,
			Key:            "trace_traffic_reject",
			Name:           "trace_traffic_reject",
			Typ:            int(ABTypGate),
			Traffic:        "SERVER",
			SubjectID:      "login_id",
			Enabled:        true,
			Sticky:         false,
			Salt:           "trace-traffic-reject-salt",
			Version:        1,
			DisableImpress: false,
			Rules: map[RuleTypEnum][]Rule{
				RuleTraffic: {
					{
						ID:      "traffic-rule",
						Name:    "Traffic Rule",
						Salt:    "traffic-rule",
						Rollout: 0,
					},
				},
			},
			VariantPayloads: map[string]json.RawMessage{},
		}
		core := newTestAbCoreWithStorage(t, newTraceTestStorage(spec))

		result, err := core.Evaluate(User{LoginID: "traffic-user"}, spec.Key)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
		require.NotNil(t, result.DecisionRuleID)
		require.Equal(t, "traffic-rule", *result.DecisionRuleID)
	})

	t.Run("sticky hit keeps empty decision rule because no rule was evaluated", func(t *testing.T) {
		spec := newTraceTestGateSpec("trace_sticky_cached", 100, true, []Condition{
			{FieldClass: "COMMON", Field: "public", Opt: "IS_TRUE", Value: nil},
		})
		handler := &memoryStickyHandler{data: make(map[string]string)}
		core := newTestAbCoreWithStorageAndSticky(t, newTraceTestStorage(spec), handler)

		failVar := VariantIDFail
		cacheBytes, err := json.Marshal(abResultCache{VariantID: &failVar})
		require.NoError(t, err)
		handler.data[fmt.Sprintf("%d-%s", spec.ID, "sticky-user")] = string(cacheBytes)

		result, err := core.Evaluate(User{LoginID: "sticky-user"}, spec.Key)
		require.NoError(t, err)
		require.False(t, result.CheckFeatureGate())
		require.Nil(t, result.DecisionRuleID)
	})
}

func newTraceTestGateSpec(key string, rollout float64, sticky bool, conditions []Condition) ABSpec {
	return ABSpec{
		ID:             len(key) + 1,
		Key:            key,
		Name:           key,
		Typ:            int(ABTypGate),
		Traffic:        "SERVER",
		SubjectID:      "login_id",
		Enabled:        true,
		Sticky:         sticky,
		Salt:           key + "-salt",
		Version:        1,
		DisableImpress: false,
		Rules: map[RuleTypEnum][]Rule{
			RuleGate: {
				{
					ID:         "rule-" + key,
					Name:       "Rule " + key,
					Salt:       "rule-" + key,
					Rollout:    rollout,
					Conditions: conditions,
				},
			},
		},
		VariantPayloads: map[string]json.RawMessage{},
	}
}

func newTraceTestStorage(specs ...ABSpec) *storage {
	store := &storage{ABSpecs: make(map[string]ABSpec, len(specs))}
	for _, spec := range specs {
		store.ABSpecs[spec.Key] = spec
	}
	return store
}

func findTraceTestUser(t *testing.T, salt string, rollout float64, shouldPass bool) User {
	t.Helper()

	for i := 0; i < 10000; i++ {
		loginID := fmt.Sprintf("trace-user-%d", i)
		pass := hashUint64(loginID, salt)%10000 < uint64(rollout*100)
		if pass == shouldPass {
			return User{LoginID: loginID}
		}
	}

	t.Fatalf("failed to find user for rollout=%v shouldPass=%v", rollout, shouldPass)
	return User{}
}
