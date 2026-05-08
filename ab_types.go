package sensorswave

import (
	"encoding/json"
	"fmt"
)

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
	VariantID         *string        `json:"vid,omitempty"`              // Variant ID: "pass/fail" for gate, variant id for config/exp, "holdout", or nil
	VariantParamValue map[string]any `json:"value,omitempty"`            // Variant parameter values (read-only)
	DisableImpress    bool           `json:"disable_impress,omitempty"`  // Disable Impress
	DecisionRuleID    *string        `json:"decision_rule_id,omitempty"` // Decision rule ID
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
func (r *ABResult) GetString(key, fallback string) string {
	if v, ok := r.VariantParamValue[key]; ok {
		if val, ok := v.(string); ok {
			return val
		}
	}
	return fallback
}

// GetNumber returns a numeric parameter value by key, or the fallback value if not found or not a number.
func (r *ABResult) GetNumber(key string, fallback float64) float64 {
	if v, ok := r.VariantParamValue[key]; ok {
		if val, ok := v.(float64); ok {
			return val
		}
	}
	return fallback
}

// GetBool returns a boolean parameter value by key, or the fallback value if not found or not a boolean.
func (r *ABResult) GetBool(key string, fallback bool) bool {
	if v, ok := r.VariantParamValue[key]; ok {
		if val, ok := v.(bool); ok {
			return val
		}
	}

	return fallback
}

// GetSlice returns a slice parameter value by key, or the fallback value if not found or not a slice.
func (r *ABResult) GetSlice(key string, fallback []interface{}) []interface{} {
	if v, ok := r.VariantParamValue[key]; ok {
		if val, ok := v.([]interface{}); ok {
			return val
		}
	}

	return fallback
}

// GetMap returns a map parameter value by key, or the fallback value if not found or not a map.
func (r *ABResult) GetMap(key string, fallback map[string]interface{}) map[string]interface{} {
	if v, ok := r.VariantParamValue[key]; ok {
		if val, ok := v.(map[string]interface{}); ok {
			return val
		}
	}

	return fallback
}

// abResultCache stores sticky session data.
type abResultCache struct {
	VariantID *string `json:"v,omitempty"` // Cached variant ID
}

// ABTypEnum identifies the kind of an AB item (gate / config / experiment / layer / holdout).
type ABTypEnum int

const (
	ABTypGate    ABTypEnum = 1
	ABTypConfig  ABTypEnum = 2
	ABTypExp     ABTypEnum = 3
	ABTypLayer   ABTypEnum = 4
	ABTypHoldout ABTypEnum = 5
)

func (s ABTypEnum) String() string {
	switch s {
	case ABTypGate:
		return "Gate"
	case ABTypConfig:
		return "Config"
	case ABTypExp:
		return "Experiment"
	case ABTypLayer:
		return "Layer"
	case ABTypHoldout:
		return "Holdout"
	default:
		return "Unknown"
	}
}

// ABSpec is the protocol-level definition of a single gate/config/experiment.
type ABSpec struct {
	ID              int                               `json:"id"`
	Key             string                            `json:"key"`     // gate/config/experiment key
	Name            string                            `json:"name"`    //
	Typ             int                               `json:"typ"`     //
	Traffic         string                            `json:"traffic"` // 1:client 2:server
	SubjectID       string                            `json:"subject_id"`
	Enabled         bool                              `json:"enabled"`
	Sticky          bool                              `json:"sticky"`
	Salt            string                            `json:"salt"`
	Version         int                               `json:"version"`          // Version number, increment on each update
	DisableImpress  bool                              `json:"disable_impress"`  // Enable Impress, Debug status is false
	Rules           map[RuleTypEnum][]Rule            `json:"rules"`            // Rule table map[RuleTyp][]rules
	VariantPayloads map[string]json.RawMessage        `json:"variant_payloads"` // Raw variant value
	VariantValues   map[string]map[string]interface{} `json:"-"`                // Parsed variant value
}

// RuleTypEnum is the rule-bucket key in ABSpec.Rules.
type RuleTypEnum string

const (
	RuleOverride RuleTypEnum = "OVERRIDE"
	RuleTraffic  RuleTypEnum = "TRAFFIC" // holdout+bucket
	RuleGate     RuleTypEnum = "GATE"
	RuleGroup    RuleTypEnum = "GROUP"
)

// Rule is one entry inside an ABSpec rule bucket.
type Rule struct {
	Name       string      `json:"name"` // display name
	ID         string      `json:"id"`   // uniq id
	Salt       string      `json:"salt,omitempty"`
	Rollout    float64     `json:"rollout"` // 0.0-100.0
	Conditions []Condition `json:"conditions,omitempty"`
	Override   *string     `json:"override,omitempty"` // override&return "true/false/variant_id"
}

// Condition is a predicate inside a Rule.Conditions list.
type Condition struct {
	FieldClass string `json:"field_class"` // "COMMON" "FFUSER" "PROPS" "TARGET" "DEFAULT"
	Field      string `json:"field"`
	Opt        string `json:"opt"`   // "ANY_OF" "NONE_OF" "ANY_OF_CASE_SENSITIVE" "NONE_OF_CASE_SENSITIVE" "IS_TRUE" "IS_FALSE"...
	Value      any    `json:"value"` // Target value
}

// UnmarshalJSON deserializes ABSpec and immediately derives VariantValues
// (parsed map) from VariantPayloads (raw bytes). VariantValues is tagged
// json:"-" and therefore never appears on the wire — every unmarshal path
// (server meta responses, GetStorageSnapshot output, etc.) rebuilds the cache
// here so callers don't each have to parse it themselves and miss it.
//
// The local type alias `abSpecRaw` short-circuits the UnmarshalJSON interface
// to avoid infinite recursion.
func (s *ABSpec) UnmarshalJSON(data []byte) error {
	type abSpecRaw ABSpec
	var raw abSpecRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = ABSpec(raw)
	if len(s.VariantPayloads) == 0 {
		return nil
	}
	s.VariantValues = make(map[string]map[string]any, len(s.VariantPayloads))
	for vid, payload := range s.VariantPayloads {
		if len(payload) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			return fmt.Errorf("ABSpec.UnmarshalJSON: variant_payloads[%s]: %w", vid, err)
		}
		s.VariantValues[vid] = m
	}
	return nil
}
