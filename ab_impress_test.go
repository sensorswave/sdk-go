package sensorswave

import (
	"encoding/json"
	"testing"
)

func newImpressTestClient() *client {
	return &client{
		cfg:     &Config{Logger: &noopLogger{}},
		msgchan: make(chan []byte, 1),
	}
}

func readImpressEvent(t *testing.T, c *client) Event {
	t.Helper()
	msg := <-c.msgchan
	var evt Event
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	return evt
}

func TestLogABImpression_Feature(t *testing.T) {
	c := newImpressTestClient()
	user := User{AnonID: "anon", LoginID: "login"}
	vid := "on"
	result := ABResult{ID: 12, Key: "feat_key", Typ: int(ABTypGate), VariantID: &vid}

	c.logABImpression(user, result)

	evt := readImpressEvent(t, c)
	if evt.Event != "$FeatureImpress" {
		t.Fatalf("expected event %s, got %s", "$FeatureImpress", evt.Event)
	}

	setMap, ok := evt.UserProperties["$set"].(map[string]any)
	if !ok {
		t.Fatalf("expected $set in user_properties")
	}
	if setMap["$feature_12"] != vid {
		t.Fatalf("expected user prop $feature_12=%s", vid)
	}

	if evt.Properties["$feature_key"] != "feat_key" {
		t.Fatalf("expected $feature_key to be feat_key")
	}
	if evt.Properties["$feature_variant"] != vid {
		t.Fatalf("expected $feature_variant to be %s", vid)
	}
	if _, ok := evt.Properties["$exp_key"]; ok {
		t.Fatalf("did not expect $exp_key on feature impress")
	}
	if _, ok := evt.Properties["$exp_variant"]; ok {
		t.Fatalf("did not expect $exp_variant on feature impress")
	}
}

func TestLogABImpression_Experiment(t *testing.T) {
	c := newImpressTestClient()
	user := User{AnonID: "anon", LoginID: "login"}
	vid := "B"
	result := ABResult{ID: 99, Key: "exp_key", Typ: int(ABTypExp), VariantID: &vid}

	c.logABImpression(user, result)

	evt := readImpressEvent(t, c)
	if evt.Event != "$ExpImpress" {
		t.Fatalf("expected event %s, got %s", "$ExpImpress", evt.Event)
	}

	setMap, ok := evt.UserProperties["$set"].(map[string]any)
	if !ok {
		t.Fatalf("expected $set in user_properties")
	}
	if setMap["$exp_99"] != vid {
		t.Fatalf("expected user prop $exp_99=%s", vid)
	}

	if evt.Properties["$exp_key"] != "exp_key" {
		t.Fatalf("expected $exp_key to be exp_key")
	}
	if evt.Properties["$exp_variant"] != vid {
		t.Fatalf("expected $exp_variant to be %s", vid)
	}
	if _, ok := evt.Properties["$feature_key"]; ok {
		t.Fatalf("did not expect $feature_key on exp impress")
	}
	if _, ok := evt.Properties["$feature_variant"]; ok {
		t.Fatalf("did not expect $feature_variant on exp impress")
	}
}

func TestLogABImpression_UnsetWhenNoVariant(t *testing.T) {
	c := newImpressTestClient()
	user := User{AnonID: "anon", LoginID: "login"}
	result := ABResult{ID: 7, Key: "feat_key", Typ: int(ABTypConfig)}

	c.logABImpression(user, result)

	evt := readImpressEvent(t, c)
	unsetMap, ok := evt.UserProperties["$unset"].(map[string]any)
	if !ok {
		t.Fatalf("expected $unset in user_properties")
	}
	if _, ok := unsetMap["$feature_7"]; !ok {
		t.Fatalf("expected $feature_7 to be unset")
	}
	if _, ok := evt.Properties["$feature_variant"]; ok {
		t.Fatalf("did not expect $feature_variant when variant is nil")
	}
}
