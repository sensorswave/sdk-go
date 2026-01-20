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
		result, err := core.evalAB(ABUser{LoginID: "1000"}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)
		require.Equal(t, "blue", result.GetString("color", ""))
	})

	t.Run("version-not-matching", func(t *testing.T) {
		result, err := core.evalAB(ABUser{LoginID: "user-low", Props: Properties{"$app_version": "10.0"}}, spec, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID)
	})

	t.Run("variant-distribution", func(t *testing.T) {
		testLoginIDs := []string{
			"user0", "user1", "user2", "user3", "user4", "user5",
			"user6", "user7", "user8", "user9", "user10",
			"alice", "bob", "charlie", "david", "eve",
			"frank", "grace", "henry", "iris", "jack",
		}

		var v1User, v2User, v3User string
		for _, uid := range testLoginIDs {
			result, err := core.evalAB(ABUser{LoginID: uid, Props: Properties{"$app_version": "10.1"}}, spec, 0)
			require.NoError(t, err)
			if result.VariantID == nil {
				continue
			}
			switch *result.VariantID {
			case "v1":
				if v1User == "" {
					v1User = uid
				}
			case "v2":
				if v2User == "" {
					v2User = uid
				}
			case "v3":
				if v3User == "" {
					v3User = uid
				}
			}
			if v1User != "" && v2User != "" && v3User != "" {
				break
			}
		}

		require.NotEmpty(t, v1User, "Could not find user for variant 1")
		require.NotEmpty(t, v2User, "Could not find user for variant 2")
		require.NotEmpty(t, v3User, "Could not find user for variant 3")

		r1, err := core.evalAB(ABUser{LoginID: v1User, Props: Properties{"$app_version": "10.1"}}, spec, 0)
		require.NoError(t, err)
		require.Equal(t, "blue", r1.GetString("color", ""))

		r2, err := core.evalAB(ABUser{LoginID: v2User, Props: Properties{"$app_version": "10.1"}}, spec, 0)
		require.NoError(t, err)
		require.Equal(t, "red", r2.GetString("color", ""))

		r3, err := core.evalAB(ABUser{LoginID: v3User, Props: Properties{"$app_version": "10.1"}}, spec, 0)
		require.NoError(t, err)
		require.Equal(t, "orange", r3.GetString("color", ""))
	})
}

func TestABCoreEvalConfigPublic(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	totalUsers := 1000
	counts := map[string]int{}
	samples := map[string]string{}

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("config-public-user-%d", i)
		result, err := core.evalAB(ABUser{LoginID: uid}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		vid := *result.VariantID
		counts[vid]++
		if _, ok := samples[vid]; !ok {
			samples[vid] = uid
		}
	}

	require.Contains(t, counts, "v1")
	require.Contains(t, counts, "v2")
	require.Contains(t, counts, "v3")

	// Rollout chain: ~10% -> variant1, ~30% -> variant2, remaining -> variant3
	v1Rate := float64(counts["v1"]) / float64(totalUsers)
	v2Rate := float64(counts["v2"]) / float64(totalUsers)
	v3Rate := float64(counts["v3"]) / float64(totalUsers)
	require.InDelta(t, 0.10, v1Rate, 0.05)
	require.InDelta(t, 0.30, v2Rate, 0.05)
	require.InDelta(t, 0.60, v3Rate, 0.05)

	r1, err := core.evalAB(ABUser{LoginID: samples["v1"]}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "blue", r1.GetString("color", ""))

	r2, err := core.evalAB(ABUser{LoginID: samples["v2"]}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "red", r2.GetString("color", ""))

	r3, err := core.evalAB(ABUser{LoginID: samples["v3"]}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "orange", r3.GetString("color", ""))
}

func TestABCoreEvalConfigTarget(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "target.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	t.Run("blocked-version", func(t *testing.T) {
		result, err := core.evalAB(ABUser{LoginID: "blocked", Props: Properties{"$app_version": "10.0"}}, spec, 0)
		require.NoError(t, err)
		require.Nil(t, result.VariantID)
	})

	totalUsers := 1000
	counts := map[string]int{}
	samples := map[string]string{}

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("config-target-user-%d", i)
		result, err := core.evalAB(ABUser{LoginID: uid, Props: Properties{"$app_version": "10.1"}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		vid := *result.VariantID
		counts[vid]++
		if _, ok := samples[vid]; !ok {
			samples[vid] = uid
		}
	}

	require.Contains(t, counts, "v1")
	require.Contains(t, counts, "v2")
	require.Contains(t, counts, "v3")

	// Rollout chain on same salt: ~10% -> variant1, ~30% -> variant2, rest -> variant3
	v1Rate := float64(counts["v1"]) / float64(totalUsers)
	v2Rate := float64(counts["v2"]) / float64(totalUsers)
	v3Rate := float64(counts["v3"]) / float64(totalUsers)
	require.InDelta(t, 0.10, v1Rate, 0.05)
	require.InDelta(t, 0.30, v2Rate, 0.05)
	require.InDelta(t, 0.60, v3Rate, 0.05)

	r1, err := core.evalAB(ABUser{LoginID: samples["v1"], Props: Properties{"$app_version": "10.1"}}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "blue", r1.GetString("color", ""))

	r2, err := core.evalAB(ABUser{LoginID: samples["v2"], Props: Properties{"$app_version": "10.1"}}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "red", r2.GetString("color", ""))

	r3, err := core.evalAB(ABUser{LoginID: samples["v3"], Props: Properties{"$app_version": "10.1"}}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "orange", r3.GetString("color", ""))
}

func TestABCoreEvalConfigHoldout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "holdout.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	totalUsers := 1000
	holdoutCount := 0
	variantCount := map[string]int{}
	samples := map[string]string{}

	for i := 0; i < totalUsers; i++ {
		uid := fmt.Sprintf("config-holdout-user-%d", i)
		result, err := core.evalAB(ABUser{LoginID: uid}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		vid := *result.VariantID
		if vid == "holdout" {
			holdoutCount++
			continue
		}
		variantCount[vid]++
		if _, ok := samples[vid]; !ok {
			samples[vid] = uid
		}
	}

	require.Equal(t, totalUsers-holdoutCount, variantCount["v1"]+variantCount["v2"]+variantCount["v3"])

	holdoutRate := float64(holdoutCount) / float64(totalUsers)
	require.InDelta(t, 0.10, holdoutRate, 0.03, "Holdout rate should be around 10%%")

	nonHoldout := float64(totalUsers - holdoutCount)
	require.InDelta(t, 0.10, float64(variantCount["v1"])/nonHoldout, 0.05)
	require.InDelta(t, 0.30, float64(variantCount["v2"])/nonHoldout, 0.05)
	require.InDelta(t, 0.60, float64(variantCount["v3"])/nonHoldout, 0.05)

	r1, err := core.evalAB(ABUser{LoginID: samples["v1"]}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "blue", r1.GetString("color", ""))

	r2, err := core.evalAB(ABUser{LoginID: samples["v2"]}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "red", r2.GetString("color", ""))

	r3, err := core.evalAB(ABUser{LoginID: samples["v3"]}, spec, 0)
	require.NoError(t, err)
	require.Equal(t, "orange", r3.GetString("color", ""))
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

		result, err := core.evalAB(ABUser{LoginID: "sticky-config-cache", Props: Properties{"is_member": false}}, spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)
		require.Equal(t, "v1", *result.VariantID)
		require.Equal(t, "blue", result.GetString("color", ""))
	})

	t.Run("write-sticky-cache", func(t *testing.T) {
		loginID := "sticky-config-new"
		result, err := core.evalAB(ABUser{LoginID: loginID, Props: Properties{"is_member": true}}, spec, 0)
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
