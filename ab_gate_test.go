package sensorswave

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestABCoreEvalGatePublicRollout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name    string
		loginID string
		want    bool
	}{
		{name: "rollout-pass", loginID: "user-pass", want: true},
		{name: "rollout-fail", loginID: "user-fail", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(User{LoginID: tc.loginID}, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalAllGatePublicRollout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "public.json"))

	testCases := []struct {
		name    string
		loginID string
		want    bool
	}{
		{name: "rollout-pass", loginID: "user-pass", want: true},
		{name: "rollout-fail", loginID: "user-fail", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			core := newTestAbCoreWithStorage(t, store)

			results, err := core.EvaluateAll(User{LoginID: tc.loginID})
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.NotNil(t, results[0].VariantID)
			require.Equal(t, tc.want, results[0].CheckGate())
			require.Equal(t, "TestSpec", results[0].Key)
		})
	}
}

func TestABCoreEvalGateAnyOfSensitiveProps(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "anyof_sensitive.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "missing-prop",
			user: User{LoginID: "user-pass"},
			want: false,
		},
		{
			name: "wrong-prop",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Safari"}},
			want: false,
		},
		{
			name: "case-mismatch",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "chrome"}},
			want: false,
		},
		{
			name: "correct-prop",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Chrome"}},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateNoneOfSensitiveProps(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "noneof_sensitive.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "missing-prop",
			user: User{LoginID: "user-pass"},
			want: true,
		},
		{
			name: "blocked-chrome",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Chrome"}},
			want: false,
		},
		{
			name: "blocked-safari",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Safari"}},
			want: false,
		},
		{
			name: "allowed-case-mismatch",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "chrome"}},
			want: true,
		},
		{
			name: "allowed-prop",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Edge"}},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateNoneOfInsensitiveProps(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "noneof_insentive.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "missing-prop",
			user: User{LoginID: "user-pass"},
			want: true,
		},
		{
			name: "blocked-chrome",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Chrome"}},
			want: false,
		},
		{
			name: "blocked-safari",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Safari"}},
			want: false,
		},
		{
			name: "blocked-case-mismatch",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "chrome"}},
			want: false,
		},
		{
			name: "allowed-prop",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Edge"}},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateIsNull(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "isnull.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "missing-prop",
			user: User{LoginID: "user-pass"},
			want: true,
		},
		{
			name: "explicit-empty-string",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": ""}},
			want: false,
		},
		{
			name: "prop-present",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Chrome"}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			if result.VariantID == nil {
				require.False(t, tc.want)
				return
			}
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateIsNotNull(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "isnotnull.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "missing-prop",
			user: User{LoginID: "user-pass"},
			want: false,
		},
		{
			name: "non-empty",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": "Chrome"}},
			want: true,
		},
		{
			// TODO Relation between empty string and NULL
			name: "empty-string",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"$browser_name": ""}},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateVersionGT(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "greater_version.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.9"}}, want: false},
		{name: "equal-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0"}}, want: false},
		{name: "greater-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}, want: true},
		{name: "greater-patch", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0.1"}}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateVersionGTE(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "greater_equal_version.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.9"}}, want: false},
		{name: "equal-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0"}}, want: true},
		{name: "greater-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateVersionLT(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "less_version.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.9"}}, want: true},
		{name: "equal-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0"}}, want: false},
		{name: "greater-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateVersionLTE(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "less_equal_version.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.5"}}, want: true},
		{name: "equal-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0"}}, want: true},
		{name: "greater-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateVersionEQ(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "equal_version.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "equal-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0"}}, want: true},
		{name: "equal-with-patch", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0.0"}}, want: true},
		{name: "not-equal", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.9"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.NotNil(t, result.VariantID)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateVersionNEQ(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "not_equal.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "equal-version", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.0"}}, want: false},
		{name: "different-major", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "11.0"}}, want: true},
		{name: "different-minor", user: User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.NotNil(t, result.VariantID)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateCustomField(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "custom_field.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-props", user: User{LoginID: "user-pass"}, want: false},
		{name: "pass-age-only", user: User{LoginID: "user-pass", ABUserProperties: Properties{"age": 11}}, want: true},
		{name: "pass-time-only", user: User{LoginID: "user-pass", ABUserProperties: Properties{"time": "2025-11-18T06:00:40Z"}}, want: true},
		{name: "fail-age-rule", user: User{LoginID: "user-pass", ABUserProperties: Properties{"age": 10}}, want: false},
		{name: "fail-time-rule", user: User{LoginID: "user-pass", ABUserProperties: Properties{"time": time.UnixMilli(1763445638999)}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.NotNil(t, result.VariantID)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateOverrideID(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "override_id.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "first-override", user: User{LoginID: "login-id-example-2"}, want: true},
		{name: "second-override", user: User{LoginID: "login-id-example-3"}, want: false},
		{name: "no-override", user: User{LoginID: "unknown"}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateOverrideCondition(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "override_condition.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "first-override", user: User{LoginID: "login-id-example-2"}, want: true},
		{name: "second-override", user: User{LoginID: "login-id-example-3"}, want: false},
		{name: "prop-override", user: User{LoginID: "other", ABUserProperties: Properties{"$country": "China"}}, want: true},
		{name: "no-override", user: User{LoginID: "other", ABUserProperties: Properties{"$country": "Japan"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateAnonIDSubject(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "anon_id.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("AnonIdTest")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-anon", user: User{LoginID: "user-pass"}, want: false},
		{name: "anon-present", user: User{AnonID: "anon-pass"}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateDisabled(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "disable.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	testCases := []struct {
		name    string
		loginID string
		want    bool
	}{
		{name: "any-user", loginID: "user-pass", want: false},
		{name: "another-user", loginID: "user-fail", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(User{LoginID: tc.loginID}, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateMissingKey(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Non_Existent_Key")
	require.Nil(t, spec)
}

func TestABCoreEvalGateEmptyRules(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "missing_gate_rules.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TestSpec")
	require.NotNil(t, spec)

	// Assuming if enabled is true but no rules, it defaults to false (safe default)
	result, err := core.evalAB(User{LoginID: "user-any"}, spec, 0)
	require.NoError(t, err)
	require.False(t, result.CheckGate())
}

func TestABCoreEvalGateFilter(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "gate_filter.json"))

	t.Run("dependency-fails-rollout-0", func(t *testing.T) {
		core := newTestAbCoreWithStorage(t, store)
		spec := core.getABSpec("FilterGate")
		require.NotNil(t, spec)

		user := User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}
		result, err := core.evalAB(user, spec, 0)
		require.NoError(t, err)
		require.False(t, result.CheckGate())
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

		// Correct version -> EasyFilterGate passes -> FilterGate passes
		user := User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "10.1"}}
		result, err := core.evalAB(user, spec, 0)
		require.NoError(t, err)
		require.True(t, result.CheckGate())
	})

	t.Run("dependency-fails-condition", func(t *testing.T) {
		core := newTestAbCoreWithStorage(t, store)
		spec := core.getABSpec("FilterGate")
		require.NotNil(t, spec)

		// Incorrect version -> EasyFilterGate fails -> FilterGate fails
		user := User{LoginID: "user-pass", ABUserProperties: Properties{PspAppVer: "9.9"}}
		result, err := core.evalAB(user, spec, 0)
		require.NoError(t, err)
		require.False(t, result.CheckGate())
	})
}

func TestABCoreEvalGateComplicate(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "complicate.json"))

	// Helper to modify rollout for deterministic testing
	setRollout := func(s *storage, key string, ruleIndex int, rollout float64) {
		if spec, ok := s.ABSpecs[key]; ok {
			if rules, ok := spec.Rules["GATE"]; ok && len(rules) > ruleIndex {
				rules[ruleIndex].Rollout = rollout
				spec.Rules["GATE"] = rules
				s.ABSpecs[key] = spec
			}
		}
	}

	// Set rollout to 100% for Rule 3 (index 2) and Rule 4 (index 3) to test conditions only
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
			// Fail Rule 1 (version).
			// Must also fail Rule 5 (Device Model IS NULL) by providing a device model.
			// Must also fail Rule 6 (Country IS NOT NULL) by NOT providing a country.
			user: User{AnonID: "any", ABUserProperties: Properties{"$app_version": "9.0", "$ip_address": "127.0.0.1", "$device_model": "Pixel"}},
			want: false,
		},
		{
			name: "rule2-pass",
			// Fail Rule 1 (version), Pass Rule 2 (browser)
			user: User{AnonID: "any", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Chrome"}},
			want: true,
		},
		{
			name: "rule3-pass",
			// Fail R1, R2. Pass Rule 3 (LoginID)
			user: User{AnonID: "any", LoginID: "login-id-example-2", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox"}},
			want: true,
		},
		{
			name: "rule4-pass",
			// Fail R1, R2, R3. Pass Rule 4 (Age > 10)
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 11}},
			want: true,
		},
		{
			name: "rule5-pass",
			// Fail R1-R4. Pass Rule 5 (Device Model is Null)
			// age=5 fails R4. device_model missing means it is null.
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 5}},
			want: true,
		},
		{
			name: "rule6-pass",
			// Fail R1-R5. Pass Rule 6 (Country Not Null)
			// device_model present fails R5. country present passes R6.
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 5, "$device_model": "Pixel", "$country": "US"}},
			want: true,
		},
		{
			name: "all-fail",
			// Fail all rules.
			// device_model present fails R5. country missing fails R6.
			user: User{AnonID: "any", LoginID: "other", ABUserProperties: Properties{"$app_version": "9.0", "$browser_name": "Firefox", "age": 5, "$device_model": "Pixel"}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateHoldout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "holdout.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("holdout_gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "version-match-holdout", // Matches "10.0", so NONE_OF fails -> Gate False
			user: User{LoginID: "user1", ABUserProperties: Properties{"$app_version": "10.0"}},
			want: false,
		},
		{
			name: "version-mismatch", // Does not match "10.0", so NONE_OF passes -> Gate True
			user: User{LoginID: "user2", ABUserProperties: Properties{"$app_version": "10.1"}},
			want: true,
		},
		{
			name: "missing-prop", // Missing prop, NONE_OF passes -> Gate True
			user: User{LoginID: "user3"},
			want: true,
		},
		{
			name: "holdout-fail",
			user: User{LoginID: "user0", ABUserProperties: Properties{"$app_version": "10.1"}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateRelease(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "release.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("ReleaseGate")
	require.NotNil(t, spec)

	// COMMON.public IS_TRUE is a system field that always evaluates to true
	// With rollout 100%, all users should pass the gate
	testCases := []struct {
		name    string
		loginID string
		want    bool
	}{
		{name: "user-pass", loginID: "user-pass", want: true},
		{name: "user-other", loginID: "user-other", want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(User{LoginID: tc.loginID}, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

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

type failStickyHandler struct {
	data map[string]string
}

func (f *failStickyHandler) GetStickyResult(key string) (string, error) {
	return f.data[key], nil
}

func (f *failStickyHandler) SetStickyResult(key string, result string) error {
	return fmt.Errorf("sticky write failed")
}

func TestABCoreEvalGateSticky(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "sticky.json"))
	handler := &memoryStickyHandler{data: make(map[string]string)}
	core := newTestAbCoreWithStorageAndSticky(t, store, handler)

	spec := core.getABSpec("Sticky_Is_True_Gate")
	require.NotNil(t, spec)

	t.Run("use-sticky-cache", func(t *testing.T) {
		trueVar := VariantIDPass
		cacheBytes, err := json.Marshal(abResultCache{VariantID: &trueVar})
		require.NoError(t, err)

		key := fmt.Sprintf("%d-%s", spec.ID, "user-cache")
		handler.data[key] = string(cacheBytes)

		result, err := core.evalAB(User{LoginID: "user-cache", ABUserProperties: Properties{"is_premium": false}}, spec, 0)
		require.NoError(t, err)
		require.True(t, result.CheckGate())
	})

	t.Run("write-sticky-cache", func(t *testing.T) {
		loginID := "user-new"
		result, err := core.evalAB(User{LoginID: loginID, ABUserProperties: Properties{"is_premium": true}}, spec, 0)
		require.NoError(t, err)
		require.True(t, result.CheckGate())

		key := fmt.Sprintf("%d-%s", spec.ID, loginID)
		cache, ok := handler.data[key]
		require.True(t, ok)

		var cacheResult abResultCache
		require.NoError(t, json.Unmarshal([]byte(cache), &cacheResult))
		require.NotNil(t, cacheResult.VariantID)
		require.Equal(t, VariantIDPass, *cacheResult.VariantID)
	})
}

func TestABCoreEvalRuleErrorPropagation(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "is_true.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Is_True_Gate")
	require.NotNil(t, spec)

	// 注入非法规则触发 evalCond 错误
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

func TestABCoreStickyWriteErrorPropagation(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "sticky.json"))
	handler := &failStickyHandler{data: make(map[string]string)}
	core := newTestAbCoreWithStorageAndSticky(t, store, handler)

	spec := core.getABSpec("Sticky_Is_True_Gate")
	require.NotNil(t, spec)

	_, err := core.evalAB(User{LoginID: "user-fail", ABUserProperties: Properties{"is_premium": true}}, spec, 0)
	require.Error(t, err)
}

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

func TestABCoreEvalGateGTE(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "gte_number.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("GTE_Number_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_age": 17}}, want: false},
		{name: "equal", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_age": 18}}, want: true},
		{name: "greater-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_age": 25}}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateLT(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "lt_number.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("LT_Number_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_age": 30}}, want: true},
		{name: "equal", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_age": 65}}, want: false},
		{name: "greater-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_age": 70}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateLTE(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "lte_number.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("LTE_Number_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "less-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_score": 85}}, want: true},
		{name: "equal", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_score": 100}}, want: true},
		{name: "greater-than", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_score": 105}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateIsTrue(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "is_true.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Is_True_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "true-value", user: User{LoginID: "user-pass", ABUserProperties: Properties{"is_premium": true}}, want: true},
		{name: "false-value", user: User{LoginID: "user-pass", ABUserProperties: Properties{"is_premium": false}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateIsFalse(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "is_false.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Is_False_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "false-value", user: User{LoginID: "user-pass", ABUserProperties: Properties{"is_banned": false}}, want: true},
		{name: "true-value", user: User{LoginID: "user-pass", ABUserProperties: Properties{"is_banned": true}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateEQ(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "eq.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("EQ_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "equal-value", user: User{LoginID: "user-pass", ABUserProperties: Properties{"country": "US"}}, want: true},
		{name: "not-equal", user: User{LoginID: "user-pass", ABUserProperties: Properties{"country": "CN"}}, want: false},
		{name: "case-mismatch", user: User{LoginID: "user-pass", ABUserProperties: Properties{"country": "us"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateNEQ(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "neq_number.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("NEQ_Number_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: true},                                                    // nil != 0 is true
		{name: "equal-value", user: User{LoginID: "user-pass", ABUserProperties: Properties{"level": float64(0)}}, want: false}, // Use float64 to match JSON deserialization
		{name: "not-equal-positive", user: User{LoginID: "user-pass", ABUserProperties: Properties{"level": 5}}, want: true},
		{name: "not-equal-negative", user: User{LoginID: "user-pass", ABUserProperties: Properties{"level": -1}}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateBefore(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "before.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Before_Time_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: true}, // zero time is before any actual time
		{name: "before-time", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_created_at": "2023-06-01T00:00:00Z"}}, want: true},
		{name: "equal-time", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_created_at": "2024-01-01T00:00:00Z"}}, want: false},
		{name: "after-time", user: User{LoginID: "user-pass", ABUserProperties: Properties{"user_created_at": "2024-06-01T00:00:00Z"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateAfter(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "after.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("After_Time_Gate")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{name: "missing-prop", user: User{LoginID: "user-pass"}, want: false},
		{name: "after-time", user: User{LoginID: "user-pass", ABUserProperties: Properties{"registration_date": "2023-06-01T00:00:00Z"}}, want: true},
		{name: "equal-time", user: User{LoginID: "user-pass", ABUserProperties: Properties{"registration_date": "2023-01-01T00:00:00Z"}}, want: false},
		{name: "before-time", user: User{LoginID: "user-pass", ABUserProperties: Properties{"registration_date": "2022-06-01T00:00:00Z"}}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalGateFail(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "gate_fail.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("Gate_Fail_Dependent")
	require.NotNil(t, spec)

	testCases := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "base-gate-fails-rollout-0",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"country": "CN"}},
			want: true, // Base gate fails (rollout=0), so GATE_FAIL passes
		},
		{
			name: "base-gate-fails-condition",
			user: User{LoginID: "user-pass", ABUserProperties: Properties{"country": "US"}},
			want: true, // Base gate fails (condition not match), so GATE_FAIL passes
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)
			require.Equal(t, tc.want, result.CheckGate())
		})
	}
}

func TestABCoreEvalAllMultipleGatesPartialHit(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "gate", "multi_gates.json"))
	core := newTestAbCoreWithStorage(t, store)

	testCases := []struct {
		name         string
		user         User
		wantGateA    bool // App version >= 10.0
		wantGateB    bool // Country in [CN, US, JP]
		wantGateC    bool // is_premium = true
		wantHitCount int  // Total gates hit (pass)
	}{
		{
			name:         "hit-all-gates",
			user:         User{LoginID: "user1", ABUserProperties: Properties{"$app_version": "10.0", "$country": "CN", "is_premium": true}},
			wantGateA:    true,
			wantGateB:    true,
			wantGateC:    true,
			wantHitCount: 3,
		},
		{
			name:         "hit-gate-a-only",
			user:         User{LoginID: "user2", ABUserProperties: Properties{"$app_version": "10.0", "$country": "KR", "is_premium": false}},
			wantGateA:    true,
			wantGateB:    false,
			wantGateC:    false,
			wantHitCount: 1,
		},
		{
			name:         "hit-gate-b-only",
			user:         User{LoginID: "user3", ABUserProperties: Properties{"$app_version": "9.0", "$country": "US", "is_premium": false}},
			wantGateA:    false,
			wantGateB:    true,
			wantGateC:    false,
			wantHitCount: 1,
		},
		{
			name:         "hit-gate-c-only",
			user:         User{LoginID: "user4", ABUserProperties: Properties{"$app_version": "9.0", "$country": "KR", "is_premium": true}},
			wantGateA:    false,
			wantGateB:    false,
			wantGateC:    true,
			wantHitCount: 1,
		},
		{
			name:         "hit-gate-a-and-b",
			user:         User{LoginID: "user5", ABUserProperties: Properties{"$app_version": "10.5", "$country": "JP"}},
			wantGateA:    true,
			wantGateB:    true,
			wantGateC:    false,
			wantHitCount: 2,
		},
		{
			name:         "hit-no-gates",
			user:         User{LoginID: "user6", ABUserProperties: Properties{"$app_version": "9.0", "$country": "KR", "is_premium": false}},
			wantGateA:    false,
			wantGateB:    false,
			wantGateC:    false,
			wantHitCount: 0,
		},
		{
			name:         "missing-all-props",
			user:         User{LoginID: "user7"},
			wantGateA:    false,
			wantGateB:    false,
			wantGateC:    false,
			wantHitCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := core.EvaluateAll(tc.user)
			require.NoError(t, err)

			// All gates should be returned in results, regardless of pass/fail
			require.Len(t, results, 3, "All 3 gates should be in results")

			// Create a map for easier assertions
			resultMap := make(map[string]ABResult)
			for _, r := range results {
				resultMap[r.Key] = r
			}

			// Verify Gate A
			gateA, ok := resultMap["Gate_A"]
			require.True(t, ok, "Gate_A should be in results")
			require.NotNil(t, gateA.VariantID, "Gate_A VariantID should not be nil")
			require.Equal(t, tc.wantGateA, gateA.CheckGate(), "Gate_A pass check mismatch")
			if tc.wantGateA {
				require.Equal(t, VariantIDPass, *gateA.VariantID)
			} else {
				require.Equal(t, VariantIDFail, *gateA.VariantID)
			}

			// Verify Gate B
			gateB, ok := resultMap["Gate_B"]
			require.True(t, ok, "Gate_B should be in results")
			require.NotNil(t, gateB.VariantID, "Gate_B VariantID should not be nil")
			require.Equal(t, tc.wantGateB, gateB.CheckGate(), "Gate_B pass check mismatch")
			if tc.wantGateB {
				require.Equal(t, VariantIDPass, *gateB.VariantID)
			} else {
				require.Equal(t, VariantIDFail, *gateB.VariantID)
			}

			// Verify Gate C
			gateC, ok := resultMap["Gate_C"]
			require.True(t, ok, "Gate_C should be in results")
			require.NotNil(t, gateC.VariantID, "Gate_C VariantID should not be nil")
			require.Equal(t, tc.wantGateC, gateC.CheckGate(), "Gate_C pass check mismatch")
			if tc.wantGateC {
				require.Equal(t, VariantIDPass, *gateC.VariantID)
			} else {
				require.Equal(t, VariantIDFail, *gateC.VariantID)
			}

			// Count how many gates passed
			hitCount := 0
			for _, r := range results {
				if r.CheckGate() {
					hitCount++
				}
			}
			require.Equal(t, tc.wantHitCount, hitCount, "Hit count mismatch")
		})
	}
}
