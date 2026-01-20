package sensorswave

import "encoding/json"

// ABTypEnum type
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

// ABSpec AB protocol data structure definition
type ABSpec struct {
	ID              int                               `json:"id"`
	Key             string                            `json:"key"`     // featureKey
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

// RuleTypEnum rule type
type RuleTypEnum string

const (
	RuleOverride RuleTypEnum = "OVERRIDE"
	RuleTraffic  RuleTypEnum = "TRAFFIC" // holdout+bucket
	RuleGate     RuleTypEnum = "GATE"
	RuleGroup    RuleTypEnum = "GROUP"
)

type Rule struct {
	Name       string      `json:"name"` // display name
	ID         string      `json:"id"`   // uniq id
	Salt       string      `json:"salt,omitempty"`
	Rollout    float64     `json:"rollout"` // 0.0-100.0
	Conditions []Condition `json:"conditions,omitempty"`
	Override   *string     `json:"override,omitempty"` // override&return "true/false/variant_id"
}

type Condition struct {
	FieldClass string `json:"field_class"` // "COMMON" "FFUSER" "PROPS" "TARGET" "DEFAULT"
	Field      string `json:"field"`
	Opt        string `json:"opt"`   // "ANY_OF" "NONE_OF" "ANY_OF_CASE_SENSITIVE" "NONE_OF_CASE_SENSITIVE" "IS_TRUE" "IS_FALSE"...
	Value      any    `json:"value"` // Target value
}
