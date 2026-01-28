package sensorswave

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestABCoreEvalExperimentPublic(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("New_Experiment")
	require.NotNil(t, spec)

	// First, find users that actually get into the experiment
	// Test multiple users to find ones in different variants
	testLoginIDs := []string{
		"user0", "user1", "user2", "user3", "user4", "user5",
		"user-pass", "user-fail", "test-user", "alice", "bob",
		"charlie", "david", "eve", "frank", "grace",
	}

	var variant1User, variant2User string
	for _, uid := range testLoginIDs {
		result, err := core.evalAB(User{LoginID: uid}, spec, 0)
		require.NoError(t, err)
		if result.VariantID != nil {
			if *result.VariantID == "v1" && variant1User == "" {
				variant1User = uid
			}
			if *result.VariantID == "v2" && variant2User == "" {
				variant2User = uid
			}
		}
		if variant1User != "" && variant2User != "" {
			break
		}
	}

	require.NotEmpty(t, variant1User, "Could not find user for variant 1")
	require.NotEmpty(t, variant2User, "Could not find user for variant 2")

	t.Run("variant-1", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: variant1User}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)

		// Check payload values
		require.Equal(t, float64(0), result.GetNumber("test", -1))
		require.Equal(t, "str0", result.GetString("test_str", ""))
		require.Equal(t, "false", result.GetString("test_bool", ""))
		require.Equal(t, `{"color":"blue"}`, result.GetString("test_json", ""))
	})

	t.Run("variant-2", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: variant2User}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v2", *result.VariantID)

		// Check payload values
		require.Equal(t, float64(1), result.GetNumber("test", -1))
		require.Equal(t, "str1", result.GetString("test_str", ""))
		require.Equal(t, "true", result.GetString("test_bool", ""))
		require.Equal(t, `{"color":"red"}`, result.GetString("test_json", ""))
	})
}

func TestABCoreEvalExperimentTrafficRollout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("New_Experiment")
	require.NotNil(t, spec)

	// Test user in and out of traffic rollout (60%)
	// Find a user inside traffic and one outside
	testLoginIDs := []string{
		"user0", "user1", "user2", "user3", "user4", "user5",
		"user6", "user7", "user8", "user9", "user10",
		"alice", "bob", "charlie", "david", "eve",
		"frank", "grace", "henry", "iris", "jack",
	}

	var userInTraffic, userOutTraffic string
	for _, uid := range testLoginIDs {
		result, err := core.evalAB(User{LoginID: uid}, spec, 0)
		require.NoError(t, err)

		if result.VariantID != nil && userInTraffic == "" {
			userInTraffic = uid
		}
		if result.VariantID == nil && userOutTraffic == "" {
			userOutTraffic = uid
		}
		if userInTraffic != "" && userOutTraffic != "" {
			break
		}
	}

	require.NotEmpty(t, userInTraffic, "Could not find user in traffic")
	require.NotEmpty(t, userOutTraffic, "Could not find user out of traffic")

	t.Run("in-traffic", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: userInTraffic}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		// Should be in variant 1 or 2
		require.Contains(t, []string{"v1", "v2"}, *result.VariantID)
	})

	t.Run("out-traffic", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: userOutTraffic}, spec, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID)
	})
}

func TestABCoreEvalExperimentTarget(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "target.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TargetExperiment")
	require.NotNil(t, spec)

	testCases := []struct {
		name          string
		user          User
		shouldBeInExp bool
		wantVariantID *string
	}{
		{
			name:          "matching-version",
			user:          User{LoginID: "user1", ABUserProperties: Properties{"$app_version": "10.0"}},
			shouldBeInExp: true,
			wantVariantID: nil, // Will be determined by rollout
		},
		{
			name:          "non-matching-version",
			user:          User{LoginID: "user1", ABUserProperties: Properties{"$app_version": "9.0"}},
			shouldBeInExp: false,
			wantVariantID: nil,
		},
		{
			name:          "missing-version",
			user:          User{LoginID: "user1"},
			shouldBeInExp: false,
			wantVariantID: nil,
		},
		{
			name:          "different-version",
			user:          User{LoginID: "user1", ABUserProperties: Properties{"$app_version": "10.1"}},
			shouldBeInExp: false,
			wantVariantID: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(tc.user, spec, 0)
			require.NoError(t, err)

			if tc.shouldBeInExp {
				require.NotNil(t, result.VariantID)
				require.Contains(t, []string{"v1", "v2"}, *result.VariantID)
			} else {
				require.Nil(t, result.VariantID)
			}
		})
	}

	// Test that different users with version 10.0 get assigned to variants
	t.Run("variant-distribution", func(t *testing.T) {
		testLoginIDs := []string{
			"user0", "user1", "user2", "user3", "user4", "user5",
			"user6", "user7", "user8", "user9", "user10",
		}

		var variant1User, variant2User string
		for _, uid := range testLoginIDs {
			result, err := core.evalAB(User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.0"}}, spec, 0)
			require.NoError(t, err)
			if result.VariantID != nil {
				if *result.VariantID == "v1" && variant1User == "" {
					variant1User = uid
				}
				if *result.VariantID == "v2" && variant2User == "" {
					variant2User = uid
				}
			}
			if variant1User != "" && variant2User != "" {
				break
			}
		}

		require.NotEmpty(t, variant1User, "Could not find user for variant 1")
		require.NotEmpty(t, variant2User, "Could not find user for variant 2")

		// Verify variant 1 payload
		result1, err := core.evalAB(User{LoginID: variant1User, ABUserProperties: Properties{"$app_version": "10.0"}}, spec, 0)
		require.NoError(t, err)
		require.Equal(t, float64(0), result1.GetNumber("test", -1))

		// Verify variant 2 payload
		result2, err := core.evalAB(User{LoginID: variant2User, ABUserProperties: Properties{"$app_version": "10.0"}}, spec, 0)
		require.NoError(t, err)
		require.Equal(t, float64(1), result2.GetNumber("test", -1))
	})
}

func TestABCoreEvalExperimentRelease(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "release.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("TargetExperiment")
	require.NotNil(t, spec)

	// All users should be in the experiment and assigned to variant 2 due to override
	testCases := []struct {
		name    string
		loginID string
	}{
		{name: "user1", loginID: "user1"},
		{name: "user2", loginID: "user2"},
		{name: "alice", loginID: "alice"},
		{name: "bob", loginID: "bob"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.evalAB(User{LoginID: tc.loginID}, spec, 0)
			require.NoError(t, err)
			require.NotNil(t, result.VariantID)
			// All users should get variant 2 because of the GATE override
			require.Equal(t, "v2", *result.VariantID)
			require.Equal(t, float64(1), result.GetNumber("test", -1))
		})
	}
}

func TestABCoreEvalExperimentLayer(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "layer.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("exp3")
	require.NotNil(t, spec)

	// Test traffic allocation using BUCKET_SET
	// The TRAFFIC rule uses BUCKET_SET which determines if user enters the experiment
	t.Run("traffic-allocation", func(t *testing.T) {
		testLoginIDs := []string{
			"user0", "user1", "user2", "user3", "user4", "user5",
			"user6", "user7", "user8", "user9", "user10",
			"user11", "user12", "user13", "user14", "user15",
			"alice", "bob", "charlie", "david", "eve",
		}

		var userInTraffic, userOutTraffic string
		for _, uid := range testLoginIDs {
			result, err := core.evalAB(User{LoginID: uid}, spec, 0)
			require.NoError(t, err)

			if result.VariantID != nil && userInTraffic == "" {
				userInTraffic = uid
			}
			if result.VariantID == nil && userOutTraffic == "" {
				userOutTraffic = uid
			}
			if userInTraffic != "" && userOutTraffic != "" {
				break
			}
		}

		require.NotEmpty(t, userInTraffic, "Could not find user in traffic")
		require.NotEmpty(t, userOutTraffic, "Could not find user out of traffic")

		// Verify user in traffic gets a variant
		result1, err := core.evalAB(User{LoginID: userInTraffic}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result1.VariantID)
		require.Contains(t, []string{"v1", "v2"}, *result1.VariantID)

		// Verify user out of traffic gets no variant
		result2, err := core.evalAB(User{LoginID: userOutTraffic}, spec, 0)
		require.NoError(t, err)
		require.Nil(t, result2.VariantID)
	})

	// Test variant distribution and payloads for users in traffic
	t.Run("variant-distribution-and-payloads", func(t *testing.T) {
		testLoginIDs := []string{
			"user0", "user1", "user2", "user3", "user4", "user5",
			"user6", "user7", "user8", "user9", "user10",
			"user11", "user12", "user13", "user14", "user15",
			"alice", "bob", "charlie", "david", "eve",
			"frank", "grace", "henry", "iris", "jack",
		}

		var variant1User, variant2User string
		for _, uid := range testLoginIDs {
			result, err := core.evalAB(User{LoginID: uid}, spec, 0)
			require.NoError(t, err)
			if result.VariantID != nil {
				if *result.VariantID == "v1" && variant1User == "" {
					variant1User = uid
				}
				if *result.VariantID == "v2" && variant2User == "" {
					variant2User = uid
				}
			}
			if variant1User != "" && variant2User != "" {
				break
			}
		}

		require.NotEmpty(t, variant1User, "Could not find user for variant 1")
		require.NotEmpty(t, variant2User, "Could not find user for variant 2")

		// Verify variant 1 payload
		result1, err := core.evalAB(User{LoginID: variant1User}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result1.VariantID)
		require.Equal(t, "v1", *result1.VariantID)
		require.Equal(t, float64(1), result1.GetNumber("test", -1))

		// Verify variant 2 payload
		result2, err := core.evalAB(User{LoginID: variant2User}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result2.VariantID)
		require.Equal(t, "v2", *result2.VariantID)
		require.Equal(t, float64(2), result2.GetNumber("test", -1))
	})

	// Test that variants are split approximately 50-50 within the traffic
	t.Run("variant-split-ratio", func(t *testing.T) {
		testLoginIDs := make([]string, 0, 100)
		for i := 0; i < 100; i++ {
			testLoginIDs = append(testLoginIDs, fmt.Sprintf("testuser%d", i))
		}

		variant1Count := 0
		variant2Count := 0
		for _, uid := range testLoginIDs {
			result, err := core.evalAB(User{LoginID: uid}, spec, 0)
			require.NoError(t, err)
			if result.VariantID != nil {
				switch *result.VariantID {
				case "v1":
					variant1Count++
				case "v2":
					variant2Count++
				}
			}
		}

		// We should have users in both variants
		require.Greater(t, variant1Count, 0, "Should have users in variant 1")
		require.Greater(t, variant2Count, 0, "Should have users in variant 2")
	})
}

func TestABCoreEvalExperimentHoldout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "holdout.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("BKduZnxYPD")
	require.NotNil(t, spec)

	totalUsers := 1000
	holdoutCount := 0
	variantCount := map[string]int{}

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("holdout-user-%d", i)
		result, err := core.evalAB(User{LoginID: uid}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		switch *result.VariantID {
		case "holdout":
			holdoutCount++
		case "v1", "v2":
			variantCount[*result.VariantID]++
		default:
			require.Failf(t, "unexpected variant", "user %s got variant %s", uid, *result.VariantID)
		}
	}

	require.Greater(t, variantCount["v1"], 0, "Should have users in variant 1")
	require.Greater(t, variantCount["v2"], 0, "Should have users in variant 2")

	holdoutRate := float64(holdoutCount) / float64(totalUsers)
	require.InDelta(t, 0.10, holdoutRate, 0.03, "Holdout rate should be around 10%%")
}

func TestABCoreEvalExperimentLayerWithHoldout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "layer_with_holdout.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("ARQjYcfVPI")
	require.NotNil(t, spec)

	totalUsers := 1000
	holdoutCount := 0
	variantCount := map[string]int{}

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("layer-holdout-user-%d", i)
		result, err := core.evalAB(User{LoginID: uid}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		switch *result.VariantID {
		case "holdout":
			holdoutCount++
		case "v1", "v2":
			variantCount[*result.VariantID]++
		default:
			require.Failf(t, "unexpected variant", "user %s got variant %s", uid, *result.VariantID)
		}
	}

	require.Greater(t, variantCount["v1"], 0, "Should have users in variant 1")
	require.Greater(t, variantCount["v2"], 0, "Should have users in variant 2")

	holdoutRate := float64(holdoutCount) / float64(totalUsers)
	require.InDelta(t, 0.10, holdoutRate, 0.03, "Holdout rate should be around 10%%")
}

func TestABCoreEvalExperimentSticky(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "sticky.json"))
	handler := &memoryStickyHandler{data: make(map[string]string)}
	core := newTestAbCoreWithStorageAndSticky(t, store, handler)

	spec := core.getABSpec("Sticky_Experiment")
	require.NotNil(t, spec)

	t.Run("use-sticky-cache", func(t *testing.T) {
		cacheVar := "v2"
		cacheBytes, err := json.Marshal(abResultCache{VariantID: &cacheVar})
		require.NoError(t, err)

		key := fmt.Sprintf("%d-%s", spec.ID, "sticky-user-cache")
		handler.data[key] = string(cacheBytes)

		result, err := core.evalAB(User{LoginID: "sticky-user-cache", ABUserProperties: Properties{"is_member": false}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v2", *result.VariantID)
		require.Equal(t, "red", result.GetString("color", ""))
	})

	t.Run("write-sticky-cache", func(t *testing.T) {
		loginID := "sticky-user-new"
		result, err := core.evalAB(User{LoginID: loginID, ABUserProperties: Properties{"is_member": true}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		key := fmt.Sprintf("%d-%s", spec.ID, loginID)
		cache, ok := handler.data[key]
		require.True(t, ok)

		var cacheResult abResultCache
		require.NoError(t, json.Unmarshal([]byte(cache), &cacheResult))
		require.NotNil(t, cacheResult.VariantID)
		require.Equal(t, *result.VariantID, *cacheResult.VariantID)
	})
}

func TestABCoreEvalExperimentGateTarget(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "gate_target_pass.json"))
	core := newTestAbCoreWithStorage(t, store)

	exp := core.getABSpec("ArlrvEnebz")
	require.NotNil(t, exp)

	t.Run("gate-fails-when-dependency-fails", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "user0", ABUserProperties: Properties{"$app_version": "9.9"}}, exp, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID)
	})

	t.Run("gate-passes-when-dependency-passes", func(t *testing.T) {
		// Rollout on dependency gate is 30%, so pick a user that actually passes it.
		var passingUser string
		for i := 0; i < 200; i++ {
			uid := fmt.Sprintf("user-pass-%d", i)
			if res, err := core.evalAB(User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.1"}}, exp, 0); err == nil && res.VariantID != nil {
				passingUser = uid
				break
			}
		}
		require.NotEmpty(t, passingUser, "could not find a user passing the dependency gate")

		result, err := core.evalAB(User{LoginID: passingUser, ABUserProperties: Properties{"$app_version": "10.1"}}, exp, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Contains(t, []string{"v1", "v2"}, *result.VariantID)
	})
}

func TestABCoreEvalExperimentGateTargetFail(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "exp", "gate_target_fail.json"))
	core := newTestAbCoreWithStorage(t, store)

	exp := core.getABSpec("Failed_Gate")
	require.NotNil(t, exp)

	t.Run("gate-fail-when-dependency-passes", func(t *testing.T) {
		dep := core.getABSpec("TestSpec")
		require.NotNil(t, dep)

		var passingUser string
		for i := 0; i < 200; i++ {
			uid := fmt.Sprintf("user-pass-%d", i)
			depRes, err := core.evalAB(User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.1"}}, dep, 0)
			require.NoError(t, err)
			if depRes.VariantID != nil && depRes.CheckFeatureGate() {
				passingUser = uid
				break
			}
		}
		require.NotEmpty(t, passingUser, "could not find user passing dependency gate")

		result, err := core.evalAB(User{LoginID: passingUser, ABUserProperties: Properties{"$app_version": "10.1"}}, exp, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID, "gate_fail should block experiment entry")
	})

	t.Run("gate-pass-when-dependency-fails", func(t *testing.T) {
		// Dependency gate fails, so GATE_FAIL should let this experiment proceed.
		result, err := core.evalAB(User{LoginID: "user0", ABUserProperties: Properties{"$app_version": "9.9"}}, exp, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Contains(t, []string{"v1", "v2"}, *result.VariantID)
	})
}
