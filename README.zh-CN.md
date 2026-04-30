# SensorsWave SDK

[![Release](https://img.shields.io/github/v/release/sensorswave/sdk-go.svg)](https://github.com/sensorswave/sdk-go/releases)
[![Go Doc](https://godoc.org/github.com/sensorswave/sdk-go?status.svg)](https://godoc.org/github.com/sensorswave/sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/sensorswave/sdk-go)](https://goreportcard.com/report/github.com/sensorswave/sdk-go)
[![Test](https://github.com/sensorswave/sdk-go/actions/workflows/test.yml/badge.svg)](https://github.com/sensorswave/sdk-go/actions/workflows/test.yml)
[![Lint](https://github.com/sensorswave/sdk-go/actions/workflows/lint.yml/badge.svg)](https://github.com/sensorswave/sdk-go/actions/workflows/lint.yml)
[![License](https://img.shields.io/github/license/sensorswave/sdk-go.svg)](https://github.com/sensorswave/sdk-go/blob/main/LICENSE)

[English](README.md) | **中文**

一款轻量级 Go SDK，用于事件埋点和 A/B 测试。

## 功能特性

- **事件埋点**：追踪用户事件，支持自定义属性
- **用户属性**：设置、累加、追加和管理用户的各类属性
- **A/B 测试**：评估功能开关、实验和功能配置
- **自动曝光记录**：自动追踪 A/B 测试曝光事件

## 安装

```bash
go get github.com/sensorswave/sdk-go
```

## 快速开始

### 基础事件埋点

```go
package main

import (
    "log"
    "github.com/sensorswave/sdk-go"
)

func main() {
    // 使用最简配置创建客户端
    client, err := sensorswave.New(
        sensorswave.Endpoint("https://your-endpoint.com"),
        sensorswave.SourceToken("your-source-token"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 追踪事件
    user := sensorswave.User{
        LoginID: "user-123",
        AnonID:  "device-456",
    }

    client.TrackEvent(user, "PageView", sensorswave.Properties{
        "page_name": "/home",
    })
}
```

### 启用 A/B 测试（可选）

启用 A/B 测试需要提供 `ABConfig`：

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

// 现在可以使用 A/B 测试方法
result, _ := client.GetExperiment(user, "my_experiment")

// 从实验结果中获取参数
btnColor := result.GetString("button_color", "blue")
showBanner := result.GetBool("show_banner", false)
discount := int(result.GetNumber("discount_percent", 0))

fmt.Printf("Experiment: %s, Button: %s, Banner: %v, Discount: %d%%\n",
    result.Key, btnColor, showBanner, discount)
```

## API 参考

### Client 接口

SDK 提供 `Client` 接口，方法按以下分类组织：

```go
type Client interface {
    // ========== 生命周期管理 ==========
    
    // Close 优雅关闭客户端，刷新所有待发送的事件。
    // 务必在应用退出前调用。
    Close() error

    // ========== 用户身份 ==========
    
    // Identify 将匿名 ID 与登录 ID 关联（注册事件）。
    // 创建一个 $identify 事件，关联用户的匿名会话与
    // 登录身份。
    Identify(user User) error

    // ========== 事件埋点 ==========
    
    // TrackEvent 追踪一个带属性的自定义事件。
    // 这是追踪用户行为的主要方法。
    TrackEvent(user User, event string, properties Properties) error
    
    // Track 直接提交一个完整构造的 Event 结构。
    // 用于高级场景；常规用法请使用 TrackEvent。
    Track(event Event) error

    // ========== 用户属性操作 ==========
    
    // ProfileSet 设置用户属性（$set）。
    // 覆盖已有值。
    ProfileSet(user User, properties Properties) error
    
    // ProfileSetOnce 仅在属性不存在时设置（$set_once）。
    // 适用于记录首次值，例如注册日期。
    ProfileSetOnce(user User, properties Properties) error
    
    // ProfileIncrement 累加数值型用户属性（$increment）。
    // 用于计数器，如 login_count 或 points。
    ProfileIncrement(user User, properties Properties) error
    
    // ProfileAppend 向列表型用户属性追加值（$append）。
    // 列表中允许重复。
    ProfileAppend(user User, properties ListProperties) error
    
    // ProfileUnion 向列表型用户属性添加唯一值（$union）。
    // 确保列表中无重复。
    ProfileUnion(user User, properties ListProperties) error
    
    // ProfileUnset 删除用户属性（$unset）。
    // 从该用户的属性中删除指定 key。
    ProfileUnset(user User, propertyKeys ...string) error
    
    // ProfileDelete 清空用户全部属性（$delete）。
    // 该操作不可逆 — 请谨慎使用。
    ProfileDelete(user User) error

    // ========== A/B 测试 ==========

    // CheckFeatureGate 评估一个功能开关并返回是否通过。
    // 当 key 不存在或不是 gate 类型时返回 (false, nil)。
    CheckFeatureGate(user User, key string) (bool, error)

    // GetFeatureConfig 为用户评估一个功能配置。
    // 当 key 不存在或不是 config 类型时返回空结果。
    GetFeatureConfig(user User, key string) (ABResult, error)

    // GetExperiment 为用户评估一个实验。
    // 当 key 不存在或不是 experiment 类型时返回空结果。
    GetExperiment(user User, key string) (ABResult, error)

    // GetABSpecs 将当前 A/B 测试元数据导出为 JSON。
    // 用于缓存 A/B 配置以便后续启动更快。
    // 在下次初始化时把返回的字节传给 ABConfig.LoadABSpecs。
    GetABSpecs() ([]byte, error)
}
```

---

---

## 用户类型

> [!WARNING]
>
> ### 🔑 用户身份要求（必读）
>
> **除 `Identify` 外的所有方法：**
>
> - ✅ `AnonID` 或 `LoginID` 至少有一个非空
> - ⚡ **如果同时提供，`LoginID` 优先用于用户标识**
>
> **仅 `Identify` 方法：**
>
> - ✅ **`AnonID` 和 `LoginID` 必须同时非空**
> - 🔗 该方法创建一个 `$identify` 事件，关联匿名身份与登录身份

### 用户类型定义

`User` 类型表示用户身份，同时用于事件埋点和 A/B 测试：

```go
type User struct {
    AnonID           string                 // 匿名 ID 或设备 ID
    LoginID          string                 // 登录用户 ID
    ABUserProperties map[string]interface{} // 用于 A/B 定向的属性
}
```

### 使用示例

**使用不同 ID 组合创建用户：**

```go
// ✅ 有效：仅 LoginID（已登录用户）
user := sensorswave.User{LoginID: "user-123"}

// ✅ 有效：仅 AnonID（匿名用户）
user := sensorswave.User{AnonID: "device-456"}

// ✅ 有效：同时提供（LoginID 优先用于标识）
user := sensorswave.User{
    LoginID: "user-123",
    AnonID:  "device-456",
}

// ❌ 无效：未提供任何 ID — 将会失败
user := sensorswave.User{}
```

**Identify 方法 — 必须同时提供两个 ID：**

```go
// ✅ 正确：同时提供两个 ID
err := client.Identify(sensorswave.User{
    AnonID:  "device-456", // ✅ 必填
    LoginID: "user-123",   // ✅ 必填
})

// ❌ 无效：只提供一个 ID — Identify 将失败
err := client.Identify(sensorswave.User{
    LoginID: "user-123", // ❌ 缺少 AnonID
})
```

**添加 A/B 定向属性：**

```go
// 创建用户
user := sensorswave.User{
    LoginID: "user-123",
    AnonID:  "device-456",
}

// 添加 A/B 定向属性（不可变模式）
user = user.WithABUserProperty(sensorswave.PspAppVer, "11.0")
user = user.WithABUserProperty("is_premium", true)

// 或一次添加多个属性
user = user.WithABUserProperties(sensorswave.Properties{
    sensorswave.PspAppVer: "11.0",
    "is_premium":          true,
})
```

---

## 事件埋点

### 关联用户身份

将匿名 ID 与登录 ID 关联（注册事件）。

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

### 追踪自定义事件

```go
user := sensorswave.User{
    AnonID:  "anon-123",
    LoginID: "user-456",
}

err := client.TrackEvent(user, "Purchase", sensorswave.Properties{
    "product_id":   "SKU-001",
    "total_amount": 99.99,
    "item_count":   2,
})
if err != nil {
    fmt.Printf("Track event failed: %v\n", err)
    return
}
```

### 使用完整事件结构追踪

```go
event := sensorswave.NewEvent("anon-123", "user-456", "PageView").
    WithProperties(sensorswave.NewProperties().
        Set("page_name", "/home").
        Set("referrer", "google.com"))

if err := client.Track(event); err != nil {
    fmt.Printf("Track failed: %v\n", err)
    return
}
```

---

## 用户属性管理

### 设置用户属性

```go
user := sensorswave.User{AnonID: "anon-123", LoginID: "user-456"}

err := client.ProfileSet(user, sensorswave.Properties{
    "name":             "John Doe",
    "email":            "john@example.com",
    "membership_level": 5,
})
if err != nil {
    fmt.Printf("ProfileSet failed: %v\n", err)
    return
}
```

### 首次设置（仅在属性不存在时生效）

```go
err := client.ProfileSetOnce(user, sensorswave.Properties{
    "first_login_date": "2026-01-20",
})
if err != nil {
    fmt.Printf("ProfileSetOnce failed: %v\n", err)
    return
}
```

### 累加数值属性

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

### 追加列表属性

```go
err := client.ProfileAppend(user, sensorswave.ListProperties{
    "tags": []any{"premium"},
})
if err != nil {
    fmt.Printf("ProfileAppend failed: %v\n", err)
    return
}
```

### 合并列表属性（去重）

```go
err := client.ProfileUnion(user, sensorswave.ListProperties{
    "categories": []any{"sports"},
})
if err != nil {
    fmt.Printf("ProfileUnion failed: %v\n", err)
    return
}
```

### 删除属性

```go
err := client.ProfileUnset(user, "temp_field", "old_field")
if err != nil {
    fmt.Printf("ProfileUnset failed: %v\n", err)
    return
}
```

### 清空用户全部属性

```go
err := client.ProfileDelete(user)
if err != nil {
    fmt.Printf("ProfileDelete failed: %v\n", err)
    return
}
```

---

## A/B 测试

### 获取功能配置值

```go
result, err := client.GetFeatureConfig(user, "button_color_config")
if err != nil {
    fmt.Printf("Feature config eval error: %v\n", err)
    return
}

// 获取带默认值的字符串
color := result.GetString("color", "blue")

// 获取带默认值的数值
size := result.GetNumber("size", 14.0)

// 获取带默认值的布尔值
enabled := result.GetBool("enabled", false)

// 获取带默认值的切片
items := result.GetSlice("items", []interface{}{})

// 获取带默认值的 map
settings := result.GetMap("settings", map[string]interface{}{})
```

### 评估实验

```go
result, err := client.GetExperiment(user, "pricing_experiment")
if err != nil {
    fmt.Printf("Experiment eval error: %v\n", err)
    return
}

// 获取实验分组参数
pricingStrategy := result.GetString("strategy", "original")

// 根据实验分组执行不同逻辑
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

## 完整 API 方法参考

### 生命周期管理

| 方法 | 签名 | 说明 | 示例 |
|--------|-----------|-------------|---------|
| **Close** | `Close() error` | 优雅关闭客户端并刷新待发送事件。务必在应用退出前调用。 | `defer client.Close()` |

### 用户身份

| 方法 | 签名 | 参数 | 返回值 | 说明 |
|---|---|---|---|---|
| **Identify** | `Identify(user User) error` | `user`：同时含 AnonID 与 LoginID 的 User | `error` | 创建 `$identify` 事件，关联匿名身份与登录身份 |

### 事件埋点

| 方法 | 签名 | 参数 | 返回值 | 说明 |
|--------|-----------|------------|---------|-------------|
| **TrackEvent** | `TrackEvent(user User, event string, properties Properties) error` | `user`：用户身份<br/>`event`：事件名<br/>`properties`：事件属性 | `error` | 追踪用户行为的主要方法，支持自定义属性 |
| **Track** | `Track(event Event) error` | `event`：完整构造的 Event 结构 | `error` | 用于高级场景的底层 API。常规用法请使用 TrackEvent |

### 用户属性操作

| 方法 | 签名 | 说明 | 使用场景 |
|--------|-----------|-------------|----------|
| **ProfileSet** | `ProfileSet(user User, properties Properties) error` | 设置或覆盖用户属性 | 更新用户名、邮箱、设置 |
| **ProfileSetOnce** | `ProfileSetOnce(user User, properties Properties) error` | 仅在属性不存在时设置 | 记录注册日期、首次来源 |
| **ProfileIncrement** | `ProfileIncrement(user User, properties Properties) error` | 累加数值属性 | 登录次数、积分、分数 |
| **ProfileAppend** | `ProfileAppend(user User, properties ListProperties) error` | 追加到列表属性（允许重复） | 添加购买记录、活动日志 |
| **ProfileUnion** | `ProfileUnion(user User, properties ListProperties) error` | 向列表属性添加唯一值 | 添加兴趣、标签、分类 |
| **ProfileUnset** | `ProfileUnset(user User, propertyKeys ...string) error` | 删除指定属性 | 清除临时或废弃字段 |
| **ProfileDelete** | `ProfileDelete(user User) error` | 清空用户全部属性（不可逆） | GDPR 数据删除请求 |

### A/B 测试

| 方法 | 签名 | 参数 | 返回值 | 说明 |
|---|---|---|---|---|
| **CheckFeatureGate** | `CheckFeatureGate(user User, key string) (bool, error)` | `user`：用户，`key`：功能开关 key | `bool, error` | 评估功能开关。key 不存在或类型不匹配时返回 (false, nil) |
| **GetFeatureConfig** | `GetFeatureConfig(user User, key string) (ABResult, error)` | `user`：用户，`key`：功能配置 key | `ABResult, error` | 评估功能配置。key 不存在或类型不匹配时返回空结果 |
| **GetExperiment** | `GetExperiment(user User, key string) (ABResult, error)` | `user`：用户，`key`：Experiment key | `ABResult, error` | 评估实验。key 不存在或类型不匹配时返回空结果 |
| **GetABSpecs** | `GetABSpecs() ([]byte, error)` | 无 | `[]byte, error` | 将当前 A/B 元数据导出为 JSON，用于缓存以加快启动 |

---

## 复杂属性输入约定

SDK 接受 `Object`（map）和 `Object Array`（map 列表）作为事件属性以及
`profile_set` / `profile_set_once` 的用户属性值。**SDK 采用 pass-through
策略，不对属性值做内容校验**；服务端可能对超出下表限制的值静默截断、
丢弃或做其它 sanitize 处理。调用方应自行约束输入以避免数据被悄无声息
修改。

| 限制项 | 值 | 适用范围 |
|-------|-------|-------|
| 字符串值字节数 | ≤ 1024（UTF-8 字节） | 任意字符串属性值 |
| 单事件 properties 数量 | ≤ 256 | 调用方传入的事件 `properties` map 的 key 数 |
| OBJECT_ARRAY 元素数 | ≤ 100 | 用作属性值的 map 列表 |

超出任一限制时，服务端可能静默截断 / 丢弃。

### 列表型用户属性操作

`ProfileAppend` 与 `ProfileUnion` 是列表操作，**不接受 `Object`（map）或
`Object Array`（map 列表）值**。请仅传入标量。SDK 不会在这里拒绝复杂值，
但服务端会将其推断为 `OBJECT_ARRAY`，与列表语义不符。

---

## 配置选项

### 客户端配置

| 字段 | 说明 | 默认值 |
|-------|-------------|---------|
| `TrackURIPath` | 事件埋点端点路径 | `/in/track` |
| `Transport` | 自定义 HTTP 传输 | 默认 transport |
| `Logger` | 自定义日志器实现 | 控制台日志器 |
| `FlushInterval` | 事件刷新间隔 | 10 秒 |
| `HTTPConcurrency` | 最大 HTTP 并发数 | 1 |
| `HTTPTimeout` | HTTP 请求超时 | 3 秒 |
| `HTTPRetry` | HTTP 重试次数 | 2 |
| `AB` | A/B 测试配置 | nil（禁用） |

### ABConfig

| 字段 | 说明 | 默认值 |
|---|---|---|
| `ProjectSecret` | 用于签名认证的项目密钥 | 必填 |
| `MetaEndpoint` | A/B 元数据服务地址 | 使用主端点 |
| `MetaURIPath` | A/B 元数据路径 | `/ab/all4eval` |
| `MetaLoadInterval` | 元数据轮询间隔 | 1 分钟（最小有效间隔：30 秒） |
| `LoadABSpecs` | 通过 `GetABSpecs()` 缓存的 A/B 快照，用于快速启动 | nil |
| `StickyHandler` | 自定义粘性会话处理器 | nil |
| `MetaLoader` | 自定义元数据加载器 | nil |

## 高级用法：缓存 A/B 快照

为提升启动性能，你可以缓存 A/B 规则并在客户端初始化时加载。

```go
// 1. 从已初始化的客户端获取快照
specs, err := client.GetABSpecs()
if err != nil {
    // 处理错误
}

// 2. 将快照保存到持久化存储（如文件、数据库、Redis）
// saveToStorage(specs)

// 3. 创建新客户端时加载快照
savedSpecs := loadFromStorage()

cfg := sensorswave.Config{
    AB: &sensorswave.ABConfig{
        ProjectSecret: "your-project-secret",
        LoadABSpecs:   savedSpecs, // 注入缓存的快照
    },
}

// 客户端将立即使用缓存的快照进行 A/B 评估
client, err := sensorswave.NewWithConfig(..., cfg)
```

---

## 预定义属性

SDK 为事件埋点和用户属性提供预定义属性常量：

```go
const (
    // 设备与系统属性
    PspAppVer        = "$app_version"     // 应用版本
    PspBrowser       = "$browser"         // 浏览器名
    PspBrowserVer    = "$browser_version" // 浏览器版本
    PspModel         = "$model"           // 设备型号
    PspIP            = "$ip"              // IP 地址
    PspOS            = "$os"              // 操作系统：ios/android/harmony
    PspOSVer         = "$os_version"      // 操作系统版本
    
    // 地理位置属性
    PspCountry       = "$country"         // 国家
    PspProvince      = "$province"        // 省 / 州
    PspCity          = "$city"            // 城市
)
```

在事件中使用：

```go
err := client.TrackEvent(user, "Purchase", sensorswave.Properties{
    sensorswave.PspAppVer: "2.1.0",
    sensorswave.PspCountry: "US",
    "product_id": "SKU-001",
})
```

在 A/B 测试中使用：

```go
user = user.WithABUserProperty(sensorswave.PspAppVer, "2.1.0")
user = user.WithABUserProperty(sensorswave.PspCountry, "US")
```

---

## 运行示例

事件埋点 / 身份关联 / 用户属性设置示例：

```bash
go run -tags=track_example ./example \
    --source-token=your_token \
    --endpoint=your_event_tracking_endpoint
```

A/B 测试示例：

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

## 许可证

详见 LICENSE 文件。
