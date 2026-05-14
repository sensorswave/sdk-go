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
		counts := sampleConfigVariants(t, core, spec, "override-dist-user", func(uid string) User {
			return User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.1"}}
		})
		requireConfigVariantSplit(t, counts.VariantTotal())
	})
}

func TestABCoreEvalConfigPublic(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "public.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	counts := sampleConfigVariants(t, core, spec, "config-public-user", func(uid string) User {
		return User{LoginID: uid}
	})
	requireConfigVariantSplit(t, counts.VariantTotal())
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
		counts := sampleConfigVariants(t, core, spec, "config-target-user", func(uid string) User {
			return User{LoginID: uid, ABUserProperties: Properties{"$app_version": "10.1"}}
		})
		requireConfigVariantSplit(t, counts.VariantTotal())
	})
}

func TestABCoreEvalConfigHoldout(t *testing.T) {
	store := mustLoadABStorageFromJSON(t, filepath.Join("testdata", "config", "holdout.json"))
	core := newTestAbCoreWithStorage(t, store)

	spec := core.getABSpec("bMHsfOAUKx")
	require.NotNil(t, spec)

	counts := sampleConfigVariants(t, core, spec, "config-holdout-user", func(uid string) User {
		return User{LoginID: uid}
	})
	variantCounts := counts.VariantTotal()
	require.Equal(t, counts.Total, counts.Holdout+variantCounts.Total)

	holdoutRate := float64(counts.Holdout) / float64(counts.Total)
	require.InDelta(t, 0.10, holdoutRate, 0.03)
	requireConfigVariantSplit(t, variantCounts)
}

type configVariantCounts struct {
	Total   int
	Holdout int
	V1      int
	V2      int
	V3      int
}

func (counts configVariantCounts) VariantTotal() configVariantCounts {
	counts.Total = counts.V1 + counts.V2 + counts.V3
	counts.Holdout = 0
	return counts
}

func sampleConfigVariants(t *testing.T, core *ABCore, spec *ABSpec, userPrefix string, makeUser func(string) User) configVariantCounts {
	t.Helper()

	counts := configVariantCounts{Total: 1000}
	for i := 0; i < counts.Total; i++ {
		uid := fmt.Sprintf("%s-%d", userPrefix, i)
		result, err := core.evalAB(makeUser(uid), spec, 0)
		require.NoError(t, err)
		require.NotNil(t, result.VariantID)

		switch *result.VariantID {
		case "holdout":
			counts.Holdout++
		case "v1":
			require.Equal(t, "blue", result.GetString("color", ""))
			counts.V1++
		case "v2":
			require.Equal(t, "red", result.GetString("color", ""))
			counts.V2++
		case "v3":
			require.Equal(t, "orange", result.GetString("color", ""))
			counts.V3++
		default:
			require.FailNowf(t, "unexpected variant", "variant_id=%s", *result.VariantID)
		}
	}
	return counts
}

func requireConfigVariantSplit(t *testing.T, counts configVariantCounts) {
	t.Helper()

	require.Equal(t, counts.Total, counts.V1+counts.V2+counts.V3)
	require.InDelta(t, 0.10, float64(counts.V1)/float64(counts.Total), 0.05)
	require.InDelta(t, 0.30, float64(counts.V2)/float64(counts.Total), 0.05)
	require.InDelta(t, 0.60, float64(counts.V3)/float64(counts.Total), 0.05)
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
