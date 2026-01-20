package sensorswave

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type ABSpecPayload struct {
	Data struct {
		Update     bool     `json:"update"`
		UpdateTime int64    `json:"update_time"`
		UpdatedAt  int64    `json:"updated_at"`
		ABEnv      ABEnv    `json:"ab_env"`
		ABSpecs    []ABSpec `json:"ab_specs"`
	} `json:"data"`
}

func mustLoadABStorageFromJSON(t *testing.T, relPath string) *storage {
	t.Helper()

	bytes, err := os.ReadFile(filepath.Clean(relPath))
	require.NoError(t, err)

	var payload ABSpecPayload
	require.NoError(t, json.Unmarshal(bytes, &payload))

	store := &storage{
		ABEnv:   payload.Data.ABEnv,
		ABSpecs: make(map[string]ABSpec, len(payload.Data.ABSpecs)),
	}
	if store.UpdateTime == 0 {
		store.UpdateTime = payload.Data.UpdatedAt
	}

	for _, spec := range payload.Data.ABSpecs {
		if len(spec.VariantPayloads) > 0 {
			if spec.VariantValues == nil {
				spec.VariantValues = make(map[string]map[string]any, len(spec.VariantPayloads))
			}
			for vid, payload := range spec.VariantPayloads {
				if len(payload) == 0 {
					continue
				}
				value := make(map[string]any)
				require.NoError(t, json.Unmarshal(payload, &value))
				spec.VariantValues[vid] = value
			}
		}
		spec.VariantPayloads = nil
		store.ABSpecs[spec.Key] = spec
	}

	return store
}

type noopLogger struct{}

func (n *noopLogger) Debugf(string, ...any) {}
func (n *noopLogger) Infof(string, ...any)  {}
func (n *noopLogger) Warnf(string, ...any)  {}
func (n *noopLogger) Errorf(string, ...any) {}

func newTestAbCoreWithStorage(t *testing.T, store *storage) *abCore {
	t.Helper()

	cfg := DefaultConfig("http://example.com", "test-token")
	cfg.WithABConfig(&ABConfig{})
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, nil)
	core.setStorage(store)

	return core
}

func newTestAbCoreWithStorageAndSticky(t *testing.T, store *storage, stickyHandler IABStickyHandler) *abCore {
	t.Helper()

	cfg := DefaultConfig("http://example.com", "test-token")
	cfg.WithABConfig(&ABConfig{stickyHandler: stickyHandler})
	cfg.logger = &noopLogger{}

	core := NewABCore(cfg, nil)
	core.setStorage(store)

	return core
}
