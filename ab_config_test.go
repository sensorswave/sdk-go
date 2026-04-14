package sensorswave

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestABCoreEvalConfigOverride(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "override.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	t.Run("override-user", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "login-id-example-1"}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)
		require.Equal(t, "blue", result.GetString("color", ""))
	})

	t.Run("version-not-matching", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "user-low", ABUserProperties: Properties{"$app_version": "10.0"}}, spec, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID)
	})

	t.Run("first-match-wins-distribution", func(t *testing.T) {
		// First-match-wins: all version>10.0 users match rule 1, only ~10% pass rollout → get v1.
		totalUsers := 200
		v1Count := 0
		nilCount := 0

		for i := 0; i < totalUsers; i++ {
			uid := fmt.Sprintf("override-dist-user-%d", i)
			result, err := core.evalAB(User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.1"}}, spec, 0)
			require.NoError(t, err)

			if result.VariantID != nil {
				require.Equal(t, "v1", *result.VariantID)
				require.Equal(t, "blue", result.GetString("color", ""))
				v1Count++
			} else {
				nilCount++
			}
		}

		v1Rate := float64(v1Count) / float64(totalUsers)
		require.InDelta(t, 0.10, v1Rate, 0.07, "Only first gate rule's rollout (10%%) should pass")
	})
}

func TestABCoreEvalConfigPublic(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	// First-match-wins: all users match rule 1 (IS_TRUE), only ~10% pass rollout → get v1.
	// The remaining ~90% match but fail rollout → gate returns false → no variant.
	totalUsers := 1000
	v1Count := 0
	nilCount := 0

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("config-public-user-%d", i)
		result, err := core.evalAB(User{LoginID: uid}, spec, 0)
		require.NoError(t, err)

		if result.VariantID != nil {
			require.Equal(t, "v1", *result.VariantID)
			require.Equal(t, "blue", result.GetString("color", ""))
			v1Count++
		} else {
			nilCount++
		}
	}

	v1Rate := float64(v1Count) / float64(totalUsers)
	require.InDelta(t, 0.10, v1Rate, 0.05, "Only first gate rule's rollout (10%%) should pass")
	require.Equal(t, totalUsers, v1Count+nilCount)
}

func TestABCoreEvalConfigTarget(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "target.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	t.Run("blocked-version", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "blocked", ABUserProperties: Properties{"$app_version": "10.0"}}, spec, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID)
	})

	t.Run("first-match-wins", func(t *testing.T) {
		// First-match-wins: all version>10.0 users match rule 1, only ~10% pass rollout → get v1.
		totalUsers := 1000
		v1Count := 0
		nilCount := 0

		for i := 0; i < totalUsers; i++ {
			uid := fmt.Sprintf("config-target-user-%d", i)
			result, err := core.evalAB(User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.1"}}, spec, 0)
			require.NoError(t, err)

			if result.VariantID != nil {
				require.Equal(t, "v1", *result.VariantID)
				require.Equal(t, "blue", result.GetString("color", ""))
				v1Count++
			} else {
				nilCount++
			}
		}

		v1Rate := float64(v1Count) / float64(totalUsers)
		require.InDelta(t, 0.10, v1Rate, 0.05, "Only first gate rule's rollout (10%%) should pass")
		require.Equal(t, totalUsers, v1Count+nilCount)
	})
}

func TestABCoreEvalConfigHoldout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "holdout.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	// Traffic rule: rollout 90 → ~10% get holdout.
	// First-match-wins gate: all non-holdout users match rule 1 (IS_TRUE), only ~10% pass rollout → v1.
	totalUsers := 1000
	holdoutCount := 0
	v1Count := 0
	nilCount := 0

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("config-holdout-user-%d", i)
		result, err := core.evalAB(User{LoginID: uid}, spec, 0)
		require.NoError(t, err)

		if result.VariantID == nil {
			nilCount++
			continue
		}
		vid := *result.VariantID
		if vid == "holdout" {
			holdoutCount++
		} else {
			require.Equal(t, "v1", vid)
			require.Equal(t, "blue", result.GetString("color", ""))
			v1Count++
		}
	}

	holdoutRate := float64(holdoutCount) / float64(totalUsers)
	require.InDelta(t, 0.10, holdoutRate, 0.03, "Holdout rate should be around 10%%")

	nonHoldout := totalUsers - holdoutCount
	v1Rate := float64(v1Count) / float64(nonHoldout)
	require.InDelta(t, 0.10, v1Rate, 0.05, "Only first gate rule's rollout (10%%) should pass among non-holdout users")
	require.Equal(t, totalUsers, holdoutCount+v1Count+nilCount)
}

func TestABCoreEvalConfigSticky(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "sticky.json"))
	handler := &memoryStickyHandler{data: make(map[string]string)}
	core := newTestAbCoreWithStorageAndSticky(t, store, handler)

	spec := core.getABSpec("Sticky_Config")
	require.NotNil(t, spec)

	t.Run("use-sticky-cache", func(t *testing.T) {
		cacheVar := "v1"
		cacheBytes, err := json.Marshal(abResultCache{VariantID: &cacheVar})
		require.NoError(t, err)

		key := fmt.Sprintf("%d-%s", spec.ID, "sticky-config-cache")
		handler.data[key] = string(cacheBytes)

		result, err := core.evalAB(User{LoginID: "sticky-config-cache", ABUserProperties: Properties{"is_member": false}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)
		require.Equal(t, "blue", result.GetString("color", ""))
	})

	t.Run("write-sticky-cache", func(t *testing.T) {
		loginID := "sticky-config-new"
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

func TestABCoreEvalConfigFirstMatchWins(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "first_match_wins.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("config_first_match")
	require.NotNil(t, spec)

	t.Run("vip-user-gets-v1", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "vip-user-1"}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)
		require.Equal(t, "vip", result.GetString("tier", ""))
	})

	t.Run("vip-user-matches-first-rule-not-second", func(t *testing.T) {
		// VIP user who is also a member should still get v1 (first match wins)
		result, err := core.evalAB(User{LoginID: "vip-user-2", ABUserProperties: Properties{"is_member": true}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)
		require.Equal(t, "vip", result.GetString("tier", ""))
	})

	t.Run("member-user-gets-v2", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "regular-member", ABUserProperties: Properties{"is_member": true}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v2", *result.VariantID)
		require.Equal(t, "member", result.GetString("tier", ""))
	})

	t.Run("public-user-gets-v3", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "anonymous-user"}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v3", *result.VariantID)
		require.Equal(t, "public", result.GetString("tier", ""))
	})

	t.Run("non-member-non-vip-still-gets-v3-via-public", func(t *testing.T) {
		result, err := core.evalAB(User{LoginID: "plain-user", ABUserProperties: Properties{"is_member": false}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v3", *result.VariantID)
		require.Equal(t, "public", result.GetString("tier", ""))
	})
}
