# SensorsWave SDK

A lightweight Go SDK for event tracking and A/B testing.

## Features

- **Event Tracking**: Track user events with custom properties
- **User Profiles**: Set, increment, append, and manage user profile properties
- **A/B Testing**: Evaluate gates, experiments, and dynamic configs
- **Automatic Exposure Logging**: Automatically track A/B test impressions
- **Fast Boot**: Cache and restore A/B metadata for faster startup
- **Sticky Sessions**: Persist traffic assignment for consistent user experiences

## Installation

```bash
go get github.com/sensorswave/sdk-go
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/sensorswave/sdk-go"
)

func main() {
    // 1. Create configuration
    cfg := sensorswave.Config{
        AB: &sensorswave.ABConfig{
            ProjectSecret: "your-project-secret",
        },
    }

    // 2. Create client
    client, err := sensorswave.NewWithConfig(
        "https://your-endpoint.com",
        "your-source-token",
        cfg,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 3. Track events and evaluate A/B tests
    user := sensorswave.User{
        LoginID: "user-123",
        AnonID:  "device-456",
    }

    client.TrackEvent(user, "page_view", sensorswave.Properties{
        "page": "/home",
    })
}
```

## API Reference

### Client Interface

```go
type Client interface {
    Close() error

    // User Identity
    Identify(user User) error

    // Event Tracking
    TrackEvent(user User, event string, properties Properties) error
    Track(event Event) error

    // User Profile Methods
    SetUserProperties(user User, properties Properties) error
    SetUserPropertiesOnce(user User, properties Properties) error
    IncrementUserProperties(user User, properties Properties) error
    AppendUserProperties(user User, properties Properties) error
    UnionUserProperties(user User, properties Properties) error
    UnsetUserProperties(user User, propertyKeys ...string) error
    DeleteUserProfile(user User) error

    // A/B Testing Methods
    ABEvaluate(user User, key string, withImpressionLog ...bool) (ABResult, error)
    ABEvaluateAll(user User) ([]ABResult, error)
    GetABSpecStorage() ([]byte, error)
}
```

---

## User Type

The `User` type represents a user identity for both event tracking and A/B testing:

```go
type User struct {
    AnonID           string                 // Anonymous or device ID
    LoginID          string                 // Login user ID
    ABUserProperties map[string]interface{} // Properties for A/B test targeting
}

// Create user
user := sensorswave.User{
    LoginID: "user-123",
    AnonID:  "device-456",
}

// Add A/B targeting properties (immutable pattern)
user = user.WithABProperty(sensorswave.PspAppVer, "11.0")
user = user.WithABProperty("is_premium", true)

// Or add multiple properties at once
user = user.WithABProperties(sensorswave.Properties{
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
err := client.Identify(user)
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
```

### Track with Full Event Structure

```go
event := sensorswave.NewEvent("anon-123", "user-456", "page_view").
    WithProperties(sensorswave.NewProperties().
        Set("page", "/home").
        Set("referrer", "google.com"))

err := client.Track(event)
```

---

## User Profile Management

### Set Profile Properties

```go
user := sensorswave.User{AnonID: "anon-123", LoginID: "user-456"}

err := client.SetUserProperties(user, sensorswave.Properties{
    "name":  "John Doe",
    "email": "john@example.com",
    "level": 5,
})
```

### Set Once (Only if Not Exists)

```go
err := client.SetUserPropertiesOnce(user, sensorswave.Properties{
    "first_login_date": "2026-01-20",
})
```

### Increment Numeric Properties

```go
err := client.IncrementUserProperties(user, sensorswave.Properties{
    "login_count": 1,
    "points":      100,
})
```

### Append to List Properties

```go
err := client.AppendUserProperties(user, sensorswave.Properties{
    "tags": "premium",
})
```

### Union List Properties

```go
err := client.UnionUserProperties(user, sensorswave.Properties{
    "categories": "sports",
})
```

### Unset Properties

```go
err := client.UnsetUserProperties(user, "temp_field", "old_field")
```

### Delete User Profile

```go
err := client.DeleteUserProfile(user)
```

---

## A/B Testing

### Evaluate a Single Experiment or Gate

```go
user := sensorswave.User{
    LoginID: "user-456",
    AnonID:  "anon-123",
}
user = user.WithABProperties(sensorswave.Properties{
    sensorswave.PspAppVer: "11.0",
    "is_premium":          true,
})

result, err := client.ABEvaluate(user, "my_experiment")
if err != nil {
    log.Printf("AB eval error: %v", err)
    return
}
```

### Check Gate (Boolean Toggle)

```go
result, _ := client.ABEvaluate(user, "new_feature_gate")
if result.CheckGate() {
    // Feature is enabled for this user
    enableNewFeature()
} else {
    // Feature is disabled
    useOldBehavior()
}
```

### Get Dynamic Config Values

```go
result, _ := client.ABEvaluate(user, "button_color_config")

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
result, _ := client.ABEvaluate(user, "pricing_experiment")

if result.VariantID != nil {
    switch *result.VariantID {
    case "control":
        showOriginalPricing()
    case "variant_a":
        showDiscountPricing()
    case "variant_b":
        showBundlePricing()
    }
}
```

### Evaluate All Experiments and Gates

```go
results, err := client.ABEvaluateAll(user)
if err != nil {
    log.Fatal(err)
}

for _, r := range results {
    fmt.Printf("Key: %s, Variant: %v\n", r.Key, r.VariantID)
}
```

### Disable Automatic Impression Logging

By default, A/B evaluation automatically logs an impression event. You can disable this:

```go
// Disable impression logging for this evaluation
result, err := client.ABEvaluate(user, "my_experiment", false)
```

### Fast Boot with Cached Metadata

```go
// Export current state for caching
storage, _ := client.GetABSpecStorage()
saveToCache(storage)

// On next startup, use cached data for faster initialization
cfg := sensorswave.Config{
    AB: &sensorswave.ABConfig{
        ProjectSecret:           "your-secret",
        LocalStorageForFastBoot: loadFromCache(),
    },
}
```

---

## Configuration Options

### Main Config

| Field | Description | Default |
|-------|-------------|---------|
| `TrackURIPath` | Event tracking endpoint path | `/in/track` |
| `Transport` | Custom HTTP transport | Default transport |
| `Logger` | Custom logger implementation | Console logger |
| `FlushInterval` | Event flush interval | 10 seconds |
| `HTTPConcurrency` | Max concurrent HTTP requests | 10 |
| `HTTPTimeout` | HTTP request timeout | 3 seconds |
| `HTTPRetry` | HTTP retry count | 2 |
| `AB` | A/B testing configuration | nil (disabled) |

### ABConfig

| Field | Description | Default |
|-------|-------------|---------|
| `ProjectSecret` | Project secret for authentication | Required |
| `MetaEndpoint` | A/B metadata server URL | Uses main endpoint |
| `MetaURIPath` | A/B metadata path | `/ab/all4eval` |
| `MetaLoadInterval` | Metadata polling interval | 30 seconds (minimum) |
| `LocalStorageForFastBoot` | Cached metadata for fast startup | nil |
| `StickyHandler` | Custom sticky session handler | nil |
| `MetaLoader` | Custom metadata loader | nil |

---

## Predefined Properties

The SDK provides predefined property constants for A/B testing targeting:

```go
const (
    PspAppVer           = "$app_version"      // App version
    PspOS               = "$os"               // Operating system
    PspOSVer            = "$os_version"       // OS version
    PspModel            = "$model"            // Device model
    PspManufacturer     = "$manufacturer"     // Device manufacturer
    PspScreenWidth      = "$screen_width"     // Screen width
    PspScreenHeight     = "$screen_height"    // Screen height
    PspNetworkType      = "$network_type"     // Network type
    PspCarrier          = "$carrier"          // Mobile carrier
    PspIsFirstDay       = "$is_first_day"     // Is first day
    PspIsFirstTime      = "$is_first_time"    // Is first time
    PspIP               = "$ip"               // IP address
    PspCountry          = "$country"          // Country
    PspProvince         = "$province"         // Province/State
    PspCity             = "$city"             // City
)
```

Usage:

```go
user = user.WithABProperty(sensorswave.PspAppVer, "2.1.0")
user = user.WithABProperty(sensorswave.PspCountry, "US")
```

---

## Running the Example

```bash
go run ./example \
    --source-token=your_token \
    --project-secret=your_secret \
    --endpoint=http://localhost:8106 \
    --meta-endpoint=http://localhost:8110 \
    --gate-key=my_gate \
    --experiment-key=my_experiment \
    --dynamic-key=my_config
```

---

## License

See LICENSE file for details.
