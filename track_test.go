package sensorswave

import (
	"encoding/json"
	"testing"
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
