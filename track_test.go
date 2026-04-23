package sensorswave

import (
	"encoding/json"
	"testing"
	"time"
)

func TestIdentifyEventName(t *testing.T) {
	client, err := New(Endpoint("http://test.example.com"), SourceToken("test-token"))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Create test user
	user := User{
		AnonID:  "test-anon-id",
		LoginID: "test-login-id",
	}

	// Track identify event
	event := NewEvent(user.AnonID, user.LoginID, PseIdentify)
	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	// Verify event name is $Identify
	if event.Event != "$Identify" {
		t.Errorf("expected event name to be '$Identify', got '%s'", event.Event)
	}
}

func TestTrackEventDefaultProperties(t *testing.T) {
	client, err := New(Endpoint("http://test.example.com"), SourceToken("test-token"))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Create test user
	user := User{
		AnonID:  "test-anon-id",
		LoginID: "test-login-id",
	}

	// Track custom event
	event := NewEvent(user.AnonID, user.LoginID, "TestEvent").
		WithProperties(NewProperties().Set("custom_prop", "value"))

	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	// Verify $lib property exists
	lib, exists := event.Properties[PspLib]
	if !exists {
		t.Error("expected $lib property to exist")
	}
	if lib != sdkType {
		t.Errorf("expected $lib to be '%s', got '%v'", sdkType, lib)
	}

	// Verify $lib_version property exists
	libVersion, exists := event.Properties[PspLibVersion]
	if !exists {
		t.Error("expected $lib_version property to exist")
	}
	if libVersion != version {
		t.Errorf("expected $lib_version to be '%s', got '%v'", version, libVersion)
	}

	// Verify custom property still exists
	customProp, exists := event.Properties["custom_prop"]
	if !exists {
		t.Error("expected custom_prop to exist")
	}
	if customProp != "value" {
		t.Errorf("expected custom_prop to be 'value', got '%v'", customProp)
	}
}

func TestEventLibPropertiesNotOverwritten(t *testing.T) {
	// Create event with pre-existing $lib and $lib_version
	event := NewEvent("anon-123", "user-456", "CustomEvent").
		WithProperties(NewProperties().
			Set(PspLib, "custom-lib").
			Set(PspLibVersion, "custom-version"))

	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	// Verify that existing values are NOT overwritten
	if event.Properties[PspLib] != "custom-lib" {
		t.Errorf("expected $lib to remain 'custom-lib', got '%v'", event.Properties[PspLib])
	}
	if event.Properties[PspLibVersion] != "custom-version" {
		t.Errorf("expected $lib_version to remain 'custom-version', got '%v'", event.Properties[PspLibVersion])
	}
}

func TestProfileSetHasDefaultProperties(t *testing.T) {
	client, err := New(Endpoint("http://test.example.com"), SourceToken("test-token"))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Create test user
	user := User{
		AnonID:  "test-anon-id",
		LoginID: "test-login-id",
	}

	// Create profile set event
	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSet))

	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	// Verify $lib and $lib_version exist
	if _, exists := event.Properties[PspLib]; !exists {
		t.Error("expected $lib property to exist in profile set event")
	}
	if _, exists := event.Properties[PspLibVersion]; !exists {
		t.Error("expected $lib_version property to exist in profile set event")
	}
}

func TestEventJSONSerialization(t *testing.T) {
	// Create and normalize event
	event := NewEvent("anon-123", "user-456", "TestEvent").
		WithProperties(NewProperties().Set("test_key", "test_value"))

	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	// Serialize to JSON
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	// Deserialize back
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	// Verify properties
	if decoded.Properties[PspLib] != sdkType {
		t.Errorf("expected $lib in decoded event to be '%s', got '%v'", sdkType, decoded.Properties[PspLib])
	}
	if decoded.Properties[PspLibVersion] != version {
		t.Errorf("expected $lib_version in decoded event to be '%s', got '%v'", version, decoded.Properties[PspLibVersion])
	}
}

func TestEventJSONSerializationNativePropertyTimeUsesISO8601UTCFormat(t *testing.T) {
	eventTime := int64(1776932130123)
	propertyTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)

	event := NewEvent("anon-123", "user-456", "TimeProbe").
		WithTime(eventTime).
		WithProperties(NewProperties().
			Set("native_time", propertyTime).
			Set("literal_time", "2026-04-23 08:15:30.123"))

	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	if decoded["time"] != float64(eventTime) {
		t.Fatalf("expected top-level time to remain %d, got %v", eventTime, decoded["time"])
	}

	properties, ok := decoded["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map, got %T", decoded["properties"])
	}

	if properties["native_time"] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("expected native_time ISO8601 UTC string, got %v", properties["native_time"])
	}
	if properties["literal_time"] != "2026-04-23 08:15:30.123" {
		t.Fatalf("expected literal_time preserved, got %v", properties["literal_time"])
	}
}

func TestProfilePropertyTimeSerializationUsesISO8601UTCFormat(t *testing.T) {
	propertyTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)

	profileSet := NewEvent("", "user-456", PseUserSet).
		WithTime(1776932130123).
		WithUserPropertyOpts(NewUserPropertyOpts().
			Set("registered_at", propertyTime).
			Set("literal_time", "2026-04-23 08:15:30.123")).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSet))

	if err := profileSet.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	setJSON, err := json.Marshal(profileSet)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	var setDecoded map[string]any
	if err := json.Unmarshal(setJSON, &setDecoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	userProperties, ok := setDecoded["user_properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected user_properties map, got %T", setDecoded["user_properties"])
	}
	setGroup := userProperties["$set"].(map[string]any)
	if setGroup["registered_at"] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("expected $set.registered_at ISO8601 UTC string, got %v", setGroup["registered_at"])
	}
	if setGroup["literal_time"] != "2026-04-23 08:15:30.123" {
		t.Fatalf("expected $set.literal_time preserved, got %v", setGroup["literal_time"])
	}

	profileSetOnce := NewEvent("", "user-456", PseUserSet).
		WithTime(1776932130123).
		WithUserPropertyOpts(NewUserPropertyOpts().SetOnce("first_seen_at", propertyTime)).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSetOnce))

	if err := profileSetOnce.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	setOnceJSON, err := json.Marshal(profileSetOnce)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	var setOnceDecoded map[string]any
	if err := json.Unmarshal(setOnceJSON, &setOnceDecoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	setOnceGroup := setOnceDecoded["user_properties"].(map[string]any)["$set_once"].(map[string]any)
	if setOnceGroup["first_seen_at"] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("expected $set_once.first_seen_at ISO8601 UTC string, got %v", setOnceGroup["first_seen_at"])
	}
}

func TestProfileListTimeSerializationUsesISO8601UTCFormat(t *testing.T) {
	propertyTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)

	appendEvent := NewEvent("", "user-456", PseUserSet).
		WithTime(1776932130123).
		WithUserPropertyOpts(NewUserPropertyOpts().Append("milestones", []any{propertyTime, propertyTime})).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeAppend))

	if err := appendEvent.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	appendJSON, err := json.Marshal(appendEvent)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	var appendDecoded map[string]any
	if err := json.Unmarshal(appendJSON, &appendDecoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	appendValues := appendDecoded["user_properties"].(map[string]any)["$append"].(map[string]any)["milestones"].([]any)
	if appendValues[0] != "2026-04-23T08:15:30.123Z" || appendValues[1] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("expected append milestones ISO8601 UTC strings, got %v", appendValues)
	}

	unionEvent := NewEvent("", "user-456", PseUserSet).
		WithTime(1776932130123).
		WithUserPropertyOpts(NewUserPropertyOpts().Union("milestones", []any{propertyTime, propertyTime})).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnion))

	if err := unionEvent.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	unionJSON, err := json.Marshal(unionEvent)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}

	var unionDecoded map[string]any
	if err := json.Unmarshal(unionJSON, &unionDecoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}

	unionValues := unionDecoded["user_properties"].(map[string]any)["$union"].(map[string]any)["milestones"].([]any)
	if len(unionValues) != 1 || unionValues[0] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("expected union milestones deduped ISO8601 UTC string, got %v", unionValues)
	}
}

func TestNativePropertyTimeAlwaysNormalizedToUTCRegardlessOfZone(t *testing.T) {
	// Same absolute instant expressed in three different zones.
	// All three must produce the same ISO8601 UTC string.
	shanghai := time.FixedZone("CST", 8*3600)
	newYork := time.FixedZone("EDT", -4*3600)
	utcTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)
	shanghaiTime := time.Date(2026, 4, 23, 16, 15, 30, 123000000, shanghai)
	newYorkTime := time.Date(2026, 4, 23, 4, 15, 30, 123000000, newYork)

	for name, pt := range map[string]time.Time{"utc": utcTime, "shanghai": shanghaiTime, "newYork": newYorkTime} {
		event := NewEvent("anon-123", "user-456", "ZoneProbe").
			WithTime(1776932130123).
			WithProperties(NewProperties().Set("t", pt))
		if err := event.Normalize(); err != nil {
			t.Fatalf("%s: normalize error: %v", name, err)
		}
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("%s: json marshal error: %v", name, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s: json unmarshal error: %v", name, err)
		}
		got := decoded["properties"].(map[string]any)["t"]
		if got != "2026-04-23T08:15:30.123Z" {
			t.Fatalf("%s: expected UTC-normalized ISO8601 string, got %v", name, got)
		}
	}
}

func TestPropertySettersPreserveNativeTypesUntilNormalize(t *testing.T) {
	propertyTime := time.Date(2026, 4, 23, 8, 15, 30, 123000000, time.UTC)

	props := NewProperties().Set("t", propertyTime)
	if _, ok := props["t"].(time.Time); !ok {
		t.Fatalf("Properties.Set must store raw time.Time until Normalize; got %T", props["t"])
	}

	up := NewUserPropertyOpts().
		Set("registered_at", propertyTime).
		SetOnce("first_seen_at", propertyTime).
		Append("milestones", []any{propertyTime, propertyTime}).
		Union("tags", []any{propertyTime, propertyTime})

	if _, ok := up["$set"].(map[string]any)["registered_at"].(time.Time); !ok {
		t.Fatalf("UserPropertyOpts.Set must store raw time.Time; got %T", up["$set"].(map[string]any)["registered_at"])
	}
	if _, ok := up["$set_once"].(map[string]any)["first_seen_at"].(time.Time); !ok {
		t.Fatalf("UserPropertyOpts.SetOnce must store raw time.Time; got %T", up["$set_once"].(map[string]any)["first_seen_at"])
	}
	appendList := up["$append"].(map[string]any)["milestones"].([]any)
	if len(appendList) != 2 {
		t.Fatalf("Append must not dedupe nor transform; got %d elements", len(appendList))
	}
	if _, ok := appendList[0].(time.Time); !ok {
		t.Fatalf("Append must store raw time.Time; got %T", appendList[0])
	}
	unionList := up["$union"].(map[string]any)["tags"].([]any)
	if len(unionList) != 2 {
		t.Fatalf("Union setter must be append-only (dedupe happens in Normalize); got %d elements", len(unionList))
	}

	// After Normalize, values are strings and $union is deduped.
	event := NewEvent("", "user-456", PseUserSet).
		WithTime(1776932130123).
		WithUserPropertyOpts(up).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnion))
	if err := event.Normalize(); err != nil {
		t.Fatalf("normalize error: %v", err)
	}

	normalizedUnion := event.UserProperties["$union"].(map[string]any)["tags"].([]any)
	if len(normalizedUnion) != 1 {
		t.Fatalf("Normalize must dedupe $union to 1 element; got %d", len(normalizedUnion))
	}
	if normalizedUnion[0] != "2026-04-23T08:15:30.123Z" {
		t.Fatalf("Normalize must format to ISO8601 UTC; got %v", normalizedUnion[0])
	}
	normalizedAppend := event.UserProperties["$append"].(map[string]any)["milestones"].([]any)
	if len(normalizedAppend) != 2 {
		t.Fatalf("Normalize must NOT dedupe $append; got %d elements", len(normalizedAppend))
	}
}
