# SensorsWave SDK

A lightweight Go SDK for event tracking and A/B testing with feature flag evaluation.

## Features

- **Event Tracking**: Track user events with custom properties
- **User Profiles**: Set, increment, append, and manage user profile properties
- **A/B Testing**: Evaluate feature flags, gates, experiments, and dynamic configs
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
    "time"
    "github.com/sensorswave/sdk-go"
)

func main() {
    // 1. Create configuration
    cfg := sensorswave.DefaultConfig("your-endpoint", "your-source-token")

    // 2. (Optional) Enable A/B testing
    abCfg := sensorswave.DefaultABConfig("your-source-token", "your-project-secret")
    cfg.WithABConfig(abCfg)

    // 3. Create client
    client, err := sensorswave.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 4. Track events and evaluate A/B tests
    // ... see API reference below
}
```

## API Reference

### Client Interface

```go
type Client interface {
    Close() error
    Identify(anonID, loginID string) error
    TrackEvent(anonID, loginID, event string, props Properties) error
    Track(event Event) error
    
    // User Profile Methods
    ProfileSet(anonID, loginID string, props Properties) error
    ProfileSetOnce(anonID, loginID string, props Properties) error
    ProfileIncrement(anonID, loginID string, props Properties) error
    ProfileAppend(anonID, loginID string, props Properties) error
    ProfileUnion(anonID, loginID string, props Properties) error
    ProfileUnset(anonID, loginID string, properties ...string) error
    ProfileDelete(anonID, loginID string) error
    
    // A/B Testing Methods
    ABEval(user ABUser, key string, withLog ...bool) (ABResult, error)
    ABEvalAll(user ABUser) ([]ABResult, error)
    ABStorageForFastBoot() ([]byte, error)
    GetABSpecs() ([]ABSpec, int64, error)
}
```

---

### Event Tracking

#### Identify User

Links an anonymous ID with a login ID (sign-up event).

```go
err := client.Identify("anon-123", "user-456")
```

#### Track Custom Event

```go
err := client.TrackEvent(
    "anon-123",
    "user-456",
    "purchase",
    sensorswave.Properties{
        "product_id": "SKU-001",
        "price":      99.99,
        "quantity":   2,
    },
)
```

#### Track with Full Event Structure

```go
event := sensorswave.NewEvent("anon-123", "user-456", "page_view").
    WithProperties(sensorswave.NewProperties().
        Set("page", "/home").
        Set("referrer", "google.com"))

err := client.Track(event)
```

---

### User Profile Management

#### Set Profile Properties

```go
err := client.ProfileSet("anon-123", "user-456", sensorswave.Properties{
    "name":  "John Doe",
    "email": "john@example.com",
    "level": 5,
})
```

#### Set Once (Only if Not Exists)

```go
err := client.ProfileSetOnce("anon-123", "user-456", sensorswave.Properties{
    "first_login_date": "2026-01-20",
})
```

#### Increment Numeric Properties

```go
err := client.ProfileIncrement("anon-123", "user-456", sensorswave.Properties{
    "login_count": 1,
    "points":      100,
})
```

#### Append to List Properties

```go
err := client.ProfileAppend("anon-123", "user-456", sensorswave.Properties{
    "tags": "premium",
})
```

#### Unset Properties

```go
err := client.ProfileUnset("anon-123", "user-456", "temp_field", "old_field")
```

#### Delete User Profile

```go
err := client.ProfileDelete("anon-123", "user-456")
```

---

### A/B Testing

#### Evaluate a Single Feature Flag

```go
user := sensorswave.ABUser{
    LoginID: "user-456",
    AnonID:  "anon-123",
    Props: sensorswave.Properties{
        sensorswave.PspAppVer: "11.0",
        "is_premium":          true,
    },
}

result, err := client.ABEval(user, "my_feature_flag")
if err != nil {
    log.Printf("AB eval error: %v", err)
    return
}
```

#### Check Gate (Boolean Toggle)

```go
result, _ := client.ABEval(user, "new_feature_gate")
if result.CheckGate() {
    // Feature is enabled for this user
    enableNewFeature()
} else {
    // Feature is disabled
    useOldBehavior()
}
```

#### Get Dynamic Config Values

```go
result, _ := client.ABEval(user, "button_color_config")

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

#### Evaluate Experiment

```go
result, _ := client.ABEval(user, "pricing_experiment")

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

#### Evaluate All Feature Flags

```go
results, err := client.ABEvalAll(user)
if err != nil {
    log.Fatal(err)
}

for _, r := range results {
    fmt.Printf("Flag: %s, Variant: %v\n", r.Key, r.VariantID)
}
```

#### Fast Boot with Cached Metadata

```go
// Export current state for caching
storage, _ := client.ABStorageForFastBoot()
saveToCache(storage)

// On next startup, use cached data for faster initialization
cachedStorage := loadFromCache()
abCfg.WithLocalStorageForFastBoot(cachedStorage)
```

---

## Configuration Options

### Main Config

| Method | Description | Default |
|--------|-------------|---------|
| `WithEndpoint(url)` | Event tracking server base URL | Empty (Required) |
| `WithTrackURIPath(path)` | Track endpoint path | `/in/track` |
| `WithTransport(transport)` | Custom HTTP transport | Default transport |
| `WithLogger(logger)` | Custom logger implementation | Console logger |
| `WithFlushInterval(duration)` | Event flush interval | 10 seconds |
| `WithHTTPConcurrency(n)` | Max concurrent HTTP requests | 10 |
| `WithHTTPTimeout(duration)` | HTTP request timeout | 3 seconds |
| `WithHTTPRetry(n)` | HTTP retry count | 2 |
| `WithABConfig(abConfig)` | Enable A/B testing | nil (disabled) |

### A/B Config

| Method | Description | Default |
|--------|-------------|---------|
| `WithMetaEndpoint(url)` | A/B metadata server base URL | Empty (defaults to Config.Endpoint) |
| `WithMetaURIPath(path)` | A/B metadata path | `/ab/all4eval` |
| `WithLoadMetaInterval(duration)` | Metadata polling interval | 10 seconds |
| `WithLocalStorageForFastBoot(data)` | Cached metadata for fast startup | nil |
| `WithStickyHandler(handler)` | Custom sticky session handler | nil |

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
