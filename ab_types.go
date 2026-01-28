package sensorswave

import "encoding/json"

// ABUser identifies a user for A/B testing evaluation.
type ABUser struct {
	AnonID     string     `json:"anon_id,omitempty"`  // AnonID is the anonymous or device ID
	LoginID    string     `json:"login_id,omitempty"` // LoginID is the unique login identifier
	Properties Properties `json:"props,omitempty"`    // Props are additional user properties used for targeting and splitting
}

// ABResult represents an AB test evaluation result.
type ABResult struct {
	ID                int            `json:"id,omitempty"`
	Key               string         `json:"key,omitempty"`
	Typ               int            `json:"typ,omitempty"`
	VariantID         *string        `json:"vid,omitempty"`             // Variant ID: "pass/fail" for gate, variant id for config/exp, "holdout", or nil
	VariantParamValue map[string]any `json:"value,omitempty"`           // Variant parameter values (read-only)
	DisableImpress    bool           `json:"disable_impress,omitempty"` // Disable Impress
}

// CheckFeatureGate returns true if the AB result indicates a "pass" for a gate.
func (r *ABResult) CheckFeatureGate() bool {
	// Early exit for various reasons, default is Fail
	if r.VariantID == nil {
		return false
	}
	return *r.VariantID == VariantIDPass
}

// JSONPayload returns the variant parameter values as a JSON raw message.
func (r *ABResult) JSONPayload() (p json.RawMessage) {
	p, _ = json.Marshal(r.VariantParamValue)
	return
}

// GetString returns a string parameter value by key, or the fallback value if not found or not a string.
func (r *ABResult) GetString(key string, fallback string) string {
	if v, ok := r.VariantParamValue[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		}
	}
	return fallback
}

// GetNumber returns a numeric parameter value by key, or the fallback value if not found or not a number.
func (r *ABResult) GetNumber(key string, fallback float64) float64 {
	if v, ok := r.VariantParamValue[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		}
	}
	return fallback
}

// GetBool returns a boolean parameter value by key, or the fallback value if not found or not a boolean.
func (r *ABResult) GetBool(key string, fallback bool) bool {
	if v, ok := r.VariantParamValue[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		}
	}

	return fallback
}

// GetSlice returns a slice parameter value by key, or the fallback value if not found or not a slice.
func (r *ABResult) GetSlice(key string, fallback []interface{}) []interface{} {
	if v, ok := r.VariantParamValue[key]; ok {
		switch val := v.(type) {
		case []interface{}:
			return val
		}
	}

	return fallback
}

// GetMap returns a map parameter value by key, or the fallback value if not found or not a map.
func (r *ABResult) GetMap(key string, fallback map[string]interface{}) map[string]interface{} {
	if v, ok := r.VariantParamValue[key]; ok {
		switch val := v.(type) {
		case map[string]interface{}:
			return val
		}
	}

	return fallback
}

// abResultCache stores sticky session data.
type abResultCache struct {
	VariantID *string `json:"v,omitempty"` // Cached variant ID
}
