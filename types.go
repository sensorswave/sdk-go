package sensorswave

import (
	"encoding/json"
	"time"
)

// Event represents an analytics event.
//
// Example JSON representation:
//
//	{
//		"anon_id": "0f485d4d12345e5f",
//		"login_id": "130xxxx1234",
//		"time": 1434557935000,
//		"event": "$page_view",
//		"trace_id": "aaabbbbcccddddd",
//		"properties": {
//			"$manufacturer": "Apple",
//			"$model": "iPhone5,2",
//			"$os": "iOS",
//			"$os_version": "7.0",
//			"$app_version": "1.3",
//			"$wifi": true,
//			"$ip": "180.79.35.65",
//			"$province": "Hunan",
//			"$city": "Changsha",
//			"$screen_width": 320,
//			"$screen_height": 568
//		},
//		"user_properties": {
//			"$set": {
//				"$model": "iPhone5,2",
//				"$os": "iOS"
//			},
//			"$set_once": {
//				"register_time": "2025-06-09 10:11:20"
//			}
//		},
//	}
//
// Event represents a single tracking event or user profile update.
type Event struct {
	AnonID         string           `json:"anon_id,omitempty"`         // Anonymous User ID
	LoginID        string           `json:"login_id,omitempty"`        // Login User ID
	Time           int64            `json:"time"`                      // Event timestamp in milliseconds
	TraceID        string           `json:"trace_id"`                  // Event trace ID
	Event          string           `json:"event"`                     // Event name
	Properties     Properties       `json:"properties,omitempty"`      // Event properties
	UserProperties UserPropertyOpts `json:"user_properties,omitempty"` // User properties
}

func NewEvent(anonID string, loginID string, event string) Event {
	return Event{
		AnonID:  anonID,
		LoginID: loginID,
		Time:    time.Now().UnixMilli(),
		TraceID: NewUUID(),
		Event:   event,
	}
}

func (e Event) WithTraceID(traceID string) Event {
	e.TraceID = traceID
	return e
}

func (e Event) WithTime(ms int64) Event {
	e.Time = ms
	return e
}

func (e Event) WithProperties(p Properties) Event {
	e.Properties = p
	return e
}

func (e Event) WithUserPropertyOpts(upo UserPropertyOpts) Event {
	e.UserProperties = upo
	return e
}

func (e *Event) Bytes() []byte {
	b, _ := json.Marshal(e)
	return b
}

func (e *Event) Normalize() error {
	if len(e.AnonID) == 0 && len(e.LoginID) == 0 {
		return ErrEmptyUserIDs
	}

	// check event name
	if e.Event == "" {
		return ErrEventNameEmpty
	}

	// check trace id
	if len(e.TraceID) == 0 {
		e.TraceID = NewUUID()
	}

	// check time
	if e.Time == 0 {
		e.Time = time.Now().UnixMilli()
	}

	return nil
}

// Properties contains event properties.
//
// Example JSON snippet:
//
//	"properties": {
//		"$manufacturer": "Apple",
//		"$model": "iPhone5,2",
//		"$os": "iOS",
//		"$os_version": "7.0",
//		"$app_version": "1.3",
//		"$wifi": true,
//		"$ip": "180.79.35.65",
//		"$province": "Hunan",
//		"$city": "Changsha",
//		"$screen_width": 320,
//		"$screen_height": 568
//	}

// Properties

// UserPropertyOpts defines options for user profile properties.
//
// Example JSON snippet:
//
//	"user_properties": {
//		"$set": {
//			"$model": "iPhone5,2",
//			"$os": "iOS"
//		},
//		"$set_once": {
//			"register_time": "2025-06-09 10:11:20"
//		},
//		"$delete": true
//	}
type UserPropertyOpts map[string]any

func NewUserPropertyOpts() UserPropertyOpts {
	return make(UserPropertyOpts, 10)
}

func (up UserPropertyOpts) Set(key string, val any) UserPropertyOpts {
	if up["$set"] == nil {
		up["$set"] = make(map[string]any)
	}
	up["$set"].(map[string]any)[key] = val
	return up
}

func (up UserPropertyOpts) SetOnce(key string, val any) UserPropertyOpts {
	if up["$set_once"] == nil {
		up["$set_once"] = make(map[string]any)
	}
	up["$set_once"].(map[string]any)[key] = val
	return up
}

func (up UserPropertyOpts) Increment(key string, val any) UserPropertyOpts {
	if up["$increment"] == nil {
		up["$increment"] = make(map[string]any)
	}
	switch v := val.(type) {
	case int, int8, int16, int32, int64, float64, float32:
		up["$increment"].(map[string]any)[key] = v
	}
	return up
}

func (up UserPropertyOpts) Append(key string, val any) UserPropertyOpts {
	if up["$append"] == nil {
		up["$append"] = make(map[string]any)
	}

	// Ensure key exists and is a slice
	if _, ok := up["$append"].(map[string]any)[key]; !ok {
		up["$append"].(map[string]any)[key] = []any{} // Initialize to an empty slice
	}

	existingValues, ok := up["$append"].(map[string]any)[key].([]any)
	if !ok { // Return if not a slice type
		return up
	}
	// Handle different input types
	switch v := val.(type) {
	case []any:
		existingValues = append(existingValues, v...)
	default:
		existingValues = append(existingValues, v)
	}

	up["$append"].(map[string]any)[key] = existingValues
	return up
}

func (up UserPropertyOpts) Union(key string, val any) UserPropertyOpts {
	if up["$union"] == nil {
		up["$union"] = make(map[string]any)
	}

	// Ensure key exists and is a slice
	if _, ok := up["$union"].(map[string]any)[key]; !ok {
		up["$union"].(map[string]any)[key] = []any{} // Initialize to an empty slice
	}

	existingValues, ok := up["$union"].(map[string]any)[key].([]any)
	if !ok { // Return if not a slice type
		return up
	}

	// Handle different input types and perform deduplication
	switch v := val.(type) {
	case []any:
		for _, item := range v {
			if !containsItem(existingValues, item) {
				existingValues = append(existingValues, item)
			}
		}
	default:
		if !containsItem(existingValues, v) {
			existingValues = append(existingValues, v)
		}
	}

	up["$union"].(map[string]any)[key] = existingValues
	return up
}

// containsItem checks if a slice contains a specific item.
func containsItem(slice []any, item any) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func (up UserPropertyOpts) Unset(key string) UserPropertyOpts {
	if up["$unset"] == nil {
		up["$unset"] = make(map[string]any)
	}
	up["$unset"].(map[string]any)[key] = nil
	return up
}

func (up UserPropertyOpts) Delete() UserPropertyOpts {
	up["$delete"] = true
	return up
}

type EventMsg []Event

// Properties is a map for event and user properties.
// Properties is a map of key-value pairs representing event or user profile attributes.
type Properties map[string]any

func NewProperties() Properties {
	return make(Properties, 10)
}

func (p Properties) Set(name string, value any) Properties {
	p[name] = value
	return p
}

func (p Properties) Merge(properties Properties) Properties {
	if properties == nil {
		return p
	}

	for k, v := range properties {
		p[k] = v
	}

	return p
}

// SetDeviceInfo sets the $device_info property.
func (p Properties) SetDeviceInfo(info string) Properties {
	return p.Set(PspDeviceInfo, info)
}

func (p Properties) DeviceInfo() string {
	info, _ := p[PspDeviceInfo].(string)
	return info
}

// SetIP sets the $ip property.
func (p Properties) SetIP(ip string) Properties {
	return p.Set(PspIP, ip)
}

func (p Properties) IP() string {
	ip, _ := p[PspIP].(string)
	return ip
}

// SetUA sets the $ua property.
func (p Properties) SetUA(ua string) Properties {
	return p.Set(PspUA, ua)
}

func (p Properties) UA() string {
	ua, _ := p[PspUA].(string)
	return ua
}

// SetOS sets the $os property.
func (p Properties) SetOS(os string) Properties {
	return p.Set(PspOS, os)
}

func (p Properties) OS() string {
	os, _ := p[PspOS].(string)
	return os
}

// SetOSVersion sets the $os_version property.
func (p Properties) SetOSVersion(osver string) Properties {
	return p.Set(PspOSVer, osver)
}

func (p Properties) OSVersion() string {
	osver, _ := p[PspOSVer].(string)
	return osver
}

// SetModel sets the $model property.
func (p Properties) SetModel(model string) Properties {
	return p.Set(PspModel, model)
}

func (p Properties) Model() string {
	model, _ := p[PspModel].(string)
	return model
}

// SetAppVersion sets the $app_version property.
func (p Properties) SetAppVersion(appver string) Properties {
	return p.Set(PspAppVer, appver)
}

func (p Properties) AppVersion() string {
	appver, _ := p[PspAppVer].(string)
	return appver
}

// SetSDKType sets the $sdk_type property.
func (p Properties) SetSDKType(sdktype string) Properties {
	return p.Set(PspSDKType, sdktype)
}

func (p Properties) SDKType() string {
	sdktype, _ := p[PspSDKType].(string)
	return sdktype
}

// SetCountry sets the $country property.
func (p Properties) SetCountry(country string) Properties {
	return p.Set(PspCountry, country)
}

func (p Properties) Country() string {
	country, _ := p[PspCountry].(string)
	return country
}

// SetProvince sets the $province property.
func (p Properties) SetProvince(province string) Properties {
	return p.Set(PspProvince, province)
}

func (p Properties) Province() string {
	province, _ := p[PspProvince].(string)
	return province
}

// SetCity sets the $city property.
func (p Properties) SetCity(city string) Properties {
	return p.Set(PspCity, city)
}

func (p Properties) City() string {
	city, _ := p[PspCity].(string)
	return city
}

// User represents a unified user identity for both A/B testing and event tracking.
// Use struct literal to create: sensorswave.User{LoginID: "user-123"}
type User struct {
	AnonID           string     `json:"anon_id,omitempty"`  // Anonymous or device ID
	LoginID          string     `json:"login_id,omitempty"` // Login user ID
	ABUserProperties Properties `json:"props,omitempty"`    // Properties for A/B test targeting
}

// WithABProperty adds a single A/B testing property for targeting.
// Returns a new User with the property added (does not modify the original).
func (u User) WithABProperty(key string, value any) User {
	if u.ABUserProperties == nil {
		u.ABUserProperties = make(Properties)
	} else {
		// Create a copy to avoid mutating the original map.
		newProps := make(Properties, len(u.ABUserProperties)+1)
		for k, v := range u.ABUserProperties {
			newProps[k] = v
		}
		u.ABUserProperties = newProps
	}
	u.ABUserProperties[key] = value
	return u
}

// WithABProperties adds multiple A/B testing properties for targeting.
// Returns a new User with the properties added (does not modify the original).
func (u User) WithABProperties(properties Properties) User {
	if properties == nil {
		return u
	}
	if u.ABUserProperties == nil {
		u.ABUserProperties = make(Properties, len(properties))
	} else {
		// Create a copy to avoid mutating the original map.
		newProps := make(Properties, len(u.ABUserProperties)+len(properties))
		for k, v := range u.ABUserProperties {
			newProps[k] = v
		}
		u.ABUserProperties = newProps
	}
	for k, v := range properties {
		u.ABUserProperties[k] = v
	}
	return u
}

// toABUser converts User to ABUser for internal A/B testing evaluation.
func (u User) toABUser() ABUser {
	return ABUser{
		AnonID:     u.AnonID,
		LoginID:    u.LoginID,
		Properties: u.ABUserProperties,
	}
}
