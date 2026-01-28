# SensorsWave SDK

[![Release](https://img.shields.io/github/v/release/sensorswave/sdk-go.svg)](https://github.com/sensorswave/sdk-go/releases)
[![Go Doc](https://godoc.org/github.com/sensorswave/sdk-go?status.svg)](https://godoc.org/github.com/sensorswave/sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/sensorswave/sdk-go)](https://goreportcard.com/report/github.com/sensorswave/sdk-go)
[![Test](https://github.com/sensorswave/sdk-go/actions/workflows/test.yml/badge.svg)](https://github.com/sensorswave/sdk-go/actions/workflows/test.yml)
[![Lint](https://github.com/sensorswave/sdk-go/actions/workflows/lint.yml/badge.svg)](https://github.com/sensorswave/sdk-go/actions/workflows/lint.yml)
[![License](https://img.shields.io/github/license/sensorswave/sdk-go.svg)](https://github.com/sensorswave/sdk-go/blob/main/LICENSE)

A lightweight Go SDK for event tracking and A/B testing.

## Features

- **Event Tracking**: Track user events with custom properties
- **User Profiles**: Set, increment, append, and manage user profile properties
- **A/B Testing**: Evaluate feature gates, experiments, and feature configs
- **Automatic Exposure Logging**: Automatically track A/B test impressions

## Installation

```bash
go get github.com/sensorswave/sdk-go
```

## Quick Start

### Basic Event Tracking

```go
package main

import (
    "log"
    "github.com/sensorswave/sdk-go"
)

func main() {
    // Create client with minimal configuration
    client, err := sensorswave.New(
        sensorswave.Endpoint("https://your-endpoint.com"),
        sensorswave.SourceToken("your-source-token"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Track events
    user := sensorswave.User{
        LoginID: "user-123",
        AnonID:  "device-456",
    }

    client.TrackEvent(user, "page_view", sensorswave.Properties{
        "page": "/home",
    })
}
```

### Enable A/B Testing (Optional)

To enable A/B testing, provide an `ABConfig`:

```go
cfg := sensorswave.Config{
    AB: &sensorswave.ABConfig{
        ProjectSecret: "your-project-secret",
    },
}

client, err := sensorswave.NewWithConfig(
    sensorswave.Endpoint("https://your-endpoint.com"),
    sensorswave.SourceToken("your-source-token"),
    cfg,
)

// Now you can use A/B testing methods
result, _ := client.GetExperiment(user, "my_experiment")

// Get parameters from the experiment result
btnColor := result.GetString("button_color", "blue")
showBanner := result.GetBool("show_banner", false)
discount := int(result.GetNumber("discount_percent", 0))

fmt.Printf("Experiment: %s, Button: %s, Banner: %v, Discount: %d%%\n",
    result.Key, btnColor, showBanner, discount)
```

## API Reference

### Client Interface

The SDK provides a `Client` interface with methods organized into the following categories:

```go
type Client interface {
    // ========== Lifecycle Management ==========
    
    // Close gracefully shuts down the client, flushing any pending events.
    // Always call this before your application exits.
    Close() error

    // ========== User Identity ==========
    
    // Identify links an anonymous ID with a login ID (signup event).
    // This creates a $SignUp event that connects the user's anonymous
    // session with their authenticated identity.
    Identify(user User) error

    // ========== Event Tracking ==========
    
    // TrackEvent tracks a custom event with properties.
    // This is the primary method for tracking user actions.
    TrackEvent(user User, event string, properties Properties) error
    
    // Track submits a fully populated Event structure directly.
    // Use this for advanced scenarios; prefer TrackEvent for normal usage.
    Track(event Event) error

    // ========== User Profile Operations ==========
    
    // ProfileSet sets user profile properties ($set).
    // Overwrites existing values.
    ProfileSet(user User, properties Properties) error
    
    // ProfileSetOnce sets user profile properties only if they don't exist ($set_once).
    // Useful for recording first-time values like registration date.
    ProfileSetOnce(user User, properties Properties) error
    
    // ProfileIncrement increments numeric user profile properties ($increment).
    // Use for counters like login_count or points.
    ProfileIncrement(user User, properties Properties) error
    
    // ProfileAppend appends values to list user profile properties ($append).
    // Allows duplicates in the list.
    ProfileAppend(user User, properties Properties) error
    
    // ProfileUnion adds unique values to list user profile properties ($union).
    // Ensures no duplicates in the list.
    ProfileUnion(user User, properties Properties) error
    
    // ProfileUnset removes user profile properties ($unset).
    // Deletes the specified properties from the user profile.
    ProfileUnset(user User, propertyKeys ...string) error
    
    // ProfileDelete deletes the entire user profile ($delete).
    // This is irreversible - use with caution.
    ProfileDelete(user User) error

    // ========== A/B Testing ==========

    // CheckFeatureGate evaluates a feature gate and returns whether it passes.
    // Returns (false, nil) if the key doesn't exist or is not a gate type.
    CheckFeatureGate(user User, key string) (bool, error)

    // GetFeatureConfig evaluates a feature config for a user.
    // Returns empty result if the key doesn't exist or is not a config type.
    GetFeatureConfig(user User, key string) (ABResult, error)

    // GetExperiment evaluates an experiment for a user.
    // Returns empty result if the key doesn't exist or is not an experiment type.
    GetExperiment(user User, key string) (ABResult, error)

    // GetABSpecs exports the current A/B testing metadata as JSON.
    // Use this to cache the A/B configuration for faster startup in future sessions.
    // Pass the returned bytes to ABConfig.LoadABSpecs on next initialization.
    GetABSpecs() ([]byte, error)
}
```

---

---

## User Type

> [!WARNING]
>
> ### 🔑 User Identity Requirements (MUST READ)
>
> **For ALL methods EXCEPT `Identify`:**
>
> - ✅ At least one of `AnonID` or `LoginID` must be non-empty
> - ⚡ **If both are provided, `LoginID` takes priority for user identification**
>
> **For the `Identify` method ONLY:**
>
> - ✅ **Both `AnonID` AND `LoginID` must be non-empty**
> - 🔗 This creates a `$identify` event linking anonymous and authenticated identities

### User Type Definition

The `User` type represents a user identity for both event tracking and A/B testing:

```go
type User struct {
    AnonID           string                 // Anonymous or device ID
    LoginID          string                 // Login user ID
    ABUserProperties map[string]interface{} // Properties for A/B test targeting
}
```

### Usage Examples

**Creating users with different ID combinations:**

```go
// ✅ Valid: LoginID only (for logged-in users)
user := sensorswave.User{LoginID: "user-123"}

// ✅ Valid: AnonID only (for anonymous users)
user := sensorswave.User{AnonID: "device-456"}

// ✅ Valid: Both IDs (LoginID takes priority for identification)
user := sensorswave.User{
    LoginID: "user-123",
    AnonID:  "device-456",
}

// ❌ INVALID: Neither ID provided - this will FAIL
user := sensorswave.User{}
```

**For the Identify method - both IDs are REQUIRED:**

```go
// ✅ Correct: Both IDs provided
err := client.Identify(sensorswave.User{
    AnonID:  "device-456", // ✅ Required
    LoginID: "user-123",   // ✅ Required
})

// ❌ INVALID: Only one ID - Identify will FAIL
err := client.Identify(sensorswave.User{
    LoginID: "user-123", // ❌ Missing AnonID
})
```

**Adding A/B targeting properties:**

```go
// Create user
user := sensorswave.User{
    LoginID: "user-123",
    AnonID:  "device-456",
}

// Add A/B targeting properties (immutable pattern)
user = user.WithABUserProperty(sensorswave.PspAppVer, "11.0")
user = user.WithABUserProperty("is_premium", true)

// Or add multiple properties at once
user = user.WithABUserProperties(sensorswave.Properties{
    sensorswave.PspAppVer: "11.0",
    "is_premium":          true,
})
```

---

## Event Tracking

### Identify User

Links an anonymous ID with a login ID (sign-up event).

```go
user := sensorswave.User{
    AnonID:  "anon-123",
    LoginID: "user-456",
}
if err := client.Identify(user); err != nil {
    fmt.Printf("Identify failed: %v\n", err)
    return
}
```

### Track Custom Event

```go
user := sensorswave.User{
    AnonID:  "anon-123",
    LoginID: "user-456",
}

err := client.TrackEvent(user, "purchase", sensorswave.Properties{
    "product_id": "SKU-001",
    "price":      99.99,
    "quantity":   2,
})
if err != nil {
    fmt.Printf("Track event failed: %v\n", err)
    return
}
```

### Track with Full Event Structure

```go
event := sensorswave.NewEvent("anon-123", "user-456", "page_view").
    WithProperties(sensorswave.NewProperties().
        Set("page", "/home").
        Set("referrer", "google.com"))

if err := client.Track(event); err != nil {
    fmt.Printf("Track failed: %v\n", err)
    return
}
```

---

## User Profile Management

### Set Profile Properties

```go
user := sensorswave.User{AnonID: "anon-123", LoginID: "user-456"}

err := client.ProfileSet(user, sensorswave.Properties{
    "name":  "John Doe",
    "email": "john@example.com",
    "level": 5,
})
if err != nil {
    fmt.Printf("ProfileSet failed: %v\n", err)
    return
}
```

### Set Once (Only if Not Exists)

```go
err := client.ProfileSetOnce(user, sensorswave.Properties{
    "first_login_date": "2026-01-20",
})
if err != nil {
    fmt.Printf("ProfileSetOnce failed: %v\n", err)
    return
}
```

### Increment Numeric Properties

```go
err := client.ProfileIncrement(user, sensorswave.Properties{
    "login_count": 1,
    "points":      100,
})
if err != nil {
    fmt.Printf("ProfileIncrement failed: %v\n", err)
    return
}
```

### Append to List Properties

```go
err := client.ProfileAppend(user, sensorswave.Properties{
    "tags": "premium",
})
if err != nil {
    fmt.Printf("ProfileAppend failed: %v\n", err)
    return
}
```

### Union List Properties

```go
err := client.ProfileUnion(user, sensorswave.Properties{
    "categories": "sports",
})
if err != nil {
    fmt.Printf("ProfileUnion failed: %v\n", err)
    return
}
```

### Unset Properties

```go
err := client.ProfileUnset(user, "temp_field", "old_field")
if err != nil {
    fmt.Printf("ProfileUnset failed: %v\n", err)
    return
}
```

### Delete User Profile

```go
err := client.ProfileDelete(user)
if err != nil {
    fmt.Printf("ProfileDelete failed: %v\n", err)
    return
}
```

---

## A/B Testing

### Get Feature Config Values

```go
result, err := client.GetFeatureConfig(user, "button_color_config")
if err != nil {
    fmt.Printf("Feature config eval error: %v\n", err)
    return
}

// Get string value with fallback
color := result.GetString("color", "blue")

// Get number value with fallback
size := result.GetNumber("size", 14.0)

// Get boolean value with fallback
enabled := result.GetBool("enabled", false)

// Get slice value with fallback
items := result.GetSlice("items", []interface{}{})

// Get map value with fallback
settings := result.GetMap("settings", map[string]interface{}{})
```

### Evaluate Experiment

```go
result, err := client.GetExperiment(user, "pricing_experiment")
if err != nil {
    fmt.Printf("Experiment eval error: %v\n", err)
    return
}

// Get experiment variant parameter
pricingStrategy := result.GetString("strategy", "original")

// Execute different logic based on experiment variant
switch pricingStrategy {
case "original":
    showOriginalPricing()
case "discount":
    showDiscountPricing(discount)
case "bundle":
    showBundlePricing(int(bundleSize))
default:
    showOriginalPricing()
}
```

---

## Complete API Method Reference

### Lifecycle Management

| Method | Signature | Description | Example |
|--------|-----------|-------------|---------|
| **Close** | `Close() error` | Gracefully shuts down the client and flushes pending events. Always call before application exit. | `defer client.Close()` |

### User Identity

| Method | Signature | Parameters | Returns | Description |
|---|---|---|---|---|
| **Identify** | `Identify(user User) error` | `user`: User with both AnonID and LoginID | `error` | Creates a `$SignUp` event linking anonymous and authenticated identities |

### Event Tracking

| Method | Signature | Parameters | Returns | Description |
|--------|-----------|------------|---------|-------------|
| **TrackEvent** | `TrackEvent(user User, event string, properties Properties) error` | `user`: User identity<br/>`event`: Event name<br/>`properties`: Event properties | `error` | Primary method for tracking user actions with custom properties |
| **Track** | `Track(event Event) error` | `event`: Fully populated Event structure | `error` | Low-level API for advanced scenarios. Use TrackEvent for normal usage |

### User Profile Operations

| Method | Signature | Description | Use Case |
|--------|-----------|-------------|----------|
| **ProfileSet** | `ProfileSet(user User, properties Properties) error` | Sets or overwrites profile properties | Update user name, email, settings |
| **ProfileSetOnce** | `ProfileSetOnce(user User, properties Properties) error` | Sets properties only if they don't exist | Record registration date, first source |
| **ProfileIncrement** | `ProfileIncrement(user User, properties Properties) error` | Increments numeric properties | Login count, points, score |
| **ProfileAppend** | `ProfileAppend(user User, properties Properties) error` | Appends to list properties (allows duplicates) | Add purchase history, activity log |
| **ProfileUnion** | `ProfileUnion(user User, properties Properties) error` | Adds unique values to list properties | Add interests, tags, categories |
| **ProfileUnset** | `ProfileUnset(user User, propertyKeys ...string) error` | Removes specified properties | Clear temporary or deprecated fields |
| **ProfileDelete** | `ProfileDelete(user User) error` | Deletes entire user profile (irreversible) | GDPR data deletion requests |

### A/B Testing

| Method | Signature | Parameters | Returns | Description |
|---|---|---|---|---|
| **CheckFeatureGate** | `CheckFeatureGate(user User, key string) (bool, error)` | `user`: User, `key`: Gate key | `bool, error` | Evaluates a feature gate. Returns (false, nil) if key not found or wrong type |
| **GetFeatureConfig** | `GetFeatureConfig(user User, key string) (ABResult, error)` | `user`: User, `key`: Config key | `ABResult, error` | Evaluates a feature config. Returns empty result if key not found or wrong type |
| **GetExperiment** | `GetExperiment(user User, key string) (ABResult, error)` | `user`: User, `key`: Experiment key | `ABResult, error` | Evaluates an experiment. Returns empty result if key not found or wrong type |
| **GetABSpecs** | `GetABSpecs() ([]byte, error)` | None | `[]byte, error` | Exports current A/B metadata as JSON for caching and faster startup |

---

## Configuration Options

### Client Config

| Field | Description | Default |
|-------|-------------|---------|
| `TrackURIPath` | Event tracking endpoint path | `/in/track` |
| `Transport` | Custom HTTP transport | Default transport |
| `Logger` | Custom logger implementation | Console logger |
| `FlushInterval` | Event flush interval | 10 seconds |
| `HTTPConcurrency` | Max concurrent HTTP requests | 1 |
| `HTTPTimeout` | HTTP request timeout | 3 seconds |
| `HTTPRetry` | HTTP retry count | 2 |
| `AB` | A/B testing configuration | nil (disabled) |

### ABConfig

| Field | Description | Default |
|---|---|---|
| `ProjectSecret` | Project secret for authentication | Required |
| `MetaEndpoint` | A/B metadata server URL | Uses main endpoint |
| `MetaURIPath` | A/B metadata path | `/ab/all4eval` |
| `MetaLoadInterval` | Metadata polling interval | 30 seconds (minimum) |
| `LoadABSpecs` | Cached A/B specs from `GetABSpecs()` for fast startup | nil |
| `StickyHandler` | Custom sticky session handler | nil |
| `MetaLoader` | Custom metadata loader | nil |

## Advanced: Caching A/B Specs

To improve startup performance, you can cache the A/B specifications and load them upon client initialization.

```go
// 1. Get specs from an initialized client
specs, err := client.GetABSpecs()
if err != nil {
    // handle error
}

// 2. Save specs to persistent storage (e.g. file, database, redis)
// saveToStorage(specs)

// 3. Load specs when creating a new client
savedSpecs := loadFromStorage()

cfg := sensorswave.Config{
    AB: &sensorswave.ABConfig{
        ProjectSecret: "your-project-secret",
        LoadABSpecs:   savedSpecs, // Inject cached specs
    },
}

// Client will be immediately ready for A/B evaluation using cached specs
client, err := sensorswave.NewWithConfig(..., cfg)
```

---

## Predefined Properties

The SDK provides predefined property constants for event tracking and user properties:

```go
const (
    // Device and System Properties
    PspAppVer        = "$app_version"     // Application version
    PspBrowser       = "$browser"         // Browser name
    PspBrowserVer    = "$browser_version" // Browser version
    PspModel         = "$model"           // Device model
    PspIP            = "$ip"              // IP address
    PspOS            = "$os"              // Operating system: ios/android/harmony
    PspOSVer         = "$os_version"      // OS version
    
    // Geographic Properties
    PspCountry       = "$country"         // Country
    PspProvince      = "$province"        // Province/State
    PspCity          = "$city"            // City
)
```

Usage in events:

```go
err := client.TrackEvent(user, "purchase", sensorswave.Properties{
    sensorswave.PspAppVer: "2.1.0",
    sensorswave.PspCountry: "US",
    "product_id": "SKU-001",
})
```

Usage in A/B testing:

```go
user = user.WithABUserProperty(sensorswave.PspAppVer, "2.1.0")
user = user.WithABUserProperty(sensorswave.PspCountry, "US")
```

---

## Running the Example

Track/Identify/ProfileSet example:

```bash
go run -tags=track_example ./example \
    --source-token=your_token \
    --endpoint=your_event_tracking_endpoint
```

AB testing example:

```bash
go run -tags=ab_example ./example \
    --source-token=your_token \
    --project-secret=your_secret \
    --endpoint=your_event_tracking_endpoint \
    --gate-key=my_feature_gate \
    --experiment-key=my_experiment \
    --feature-config-key=my_feature_config
```

---

## License

See LICENSE file for details.
