package sensorswave

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config defines the configuration for the SDK client.
type Config struct {
	// TrackURIPath is the URI path for event tracking. Default: "/in/track"
	TrackURIPath string

	// Transport is a custom HTTP transport. If nil, the default transport is used.
	Transport *http.Transport

	// Logger is a custom logger. If nil, the default logger is used.
	Logger Logger

	// FlushInterval is the interval for flushing buffered events. Default: 10s
	FlushInterval time.Duration

	// HTTPConcurrency is the maximum number of concurrent HTTP requests. Default: 10
	HTTPConcurrency int

	// HTTPTimeout is the timeout for each HTTP request. Default: 3s
	HTTPTimeout time.Duration

	// HTTPRetry is the number of retry attempts for failed HTTP requests. Default: 2
	HTTPRetry int

	// OnTrackFailHandler is called when event tracking fails.
	OnTrackFailHandler OnTrackFailHandler

	// AB is the A/B testing configuration. If nil, A/B testing is disabled.
	AB *ABConfig
}

// ABConfig defines the configuration for A/B testing functionality.
type ABConfig struct {
	// ProjectSecret is the secret key for A/B testing authentication.
	// Required when using the default HTTPSignatureMetaLoader.
	ProjectSecret string

	// MetaEndpoint is the endpoint for fetching A/B test metadata.
	// If empty, uses the main endpoint.
	MetaEndpoint string

	// MetaURIPath is the URI path for A/B test metadata. Default: "/ab/all4eval"
	MetaURIPath string

	// MetaLoadInterval is the interval for refreshing A/B test metadata. Default: 10s
	MetaLoadInterval time.Duration

	// StickyHandler is a custom handler for sticky session persistence.
	StickyHandler IABStickyHandler

	// MetaLoader is a custom metadata loader. If set, MetaEndpoint is ignored.
	MetaLoader IABMetaLoader

	// LocalStorageForFastBoot is JSON metadata for faster initial startup.
	LocalStorageForFastBoot []byte
}

// IABStickyHandler is the interface for persisting traffic assignment results.
type IABStickyHandler interface {
	GetStickyResult(key string) (string, error)
	SetStickyResult(key string, result string) error
}

// Logger is the interface for SDK logging.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// config default
const (
	defaultTrackPath  = "/in/track"
	defaultABMetaPath = "/ab/all4eval"
)

// OnTrackFailHandler is called when event tracking fails.
type OnTrackFailHandler func([]Event, error)

// Endpoint is a type alias for the API endpoint URL.
// Using a distinct type prevents accidentally swapping endpoint and token parameters.
type Endpoint string

// SourceToken is a type alias for the source authentication token.
// Using a distinct type prevents accidentally swapping endpoint and token parameters.
type SourceToken string

type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	format = fmt.Sprintf("%s [DEBUG] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), format)
	fmt.Printf(format, args...)
}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	format = fmt.Sprintf("%s [INFO] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), format)
	fmt.Printf(format, args...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	format = fmt.Sprintf("%s [WARN] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), format)
	fmt.Printf(format, args...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("%s [ERROR] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), format)
	fmt.Printf(format, args...)
}

// normalizeEndpoint validates and normalizes the endpoint URL.
// It ensures the scheme is http or https, and removes any path/query/fragment.
func normalizeEndpoint(endpoint string) (string, error) {
	if endpoint == "" {
		return "", nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}

	// Reconstruct URL with only Scheme and Host
	normalized := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	return normalized, nil
}

func normalizeURIPath(path string, defaultPath string) (string, error) {
	if path == "" {
		return defaultPath, nil
	}
	if path[0] != '/' {
		return defaultPath, fmt.Errorf("path must start with '/'")
	}
	if strings.Contains(path, "://") {
		return defaultPath, fmt.Errorf("path must not contain scheme")
	}
	if strings.ContainsAny(path, "?#") {
		return defaultPath, fmt.Errorf("path must not contain query or fragment")
	}
	if strings.ContainsAny(path, " \t\r\n") {
		return defaultPath, fmt.Errorf("path must not contain spaces")
	}
	return path, nil
}

// normalizeConfig normalizes the Config fields and applies defaults.
func normalizeConfig(config *Config) {
	// Normalize track URI path
	if normalized, err := normalizeURIPath(config.TrackURIPath, defaultTrackPath); err == nil {
		config.TrackURIPath = normalized
	} else {
		config.TrackURIPath = defaultTrackPath
	}

	// Apply defaults
	if config.Logger == nil {
		config.Logger = &defaultLogger{}
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 10 * time.Second
	}
	if config.HTTPConcurrency == 0 {
		config.HTTPConcurrency = 10
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 3 * time.Second
	}
	if config.HTTPRetry == 0 {
		config.HTTPRetry = 2
	}

	// Normalize AB config
	if config.AB != nil {
		normalizeABConfig(config.AB)
	}
}

// normalizeABConfig normalizes the ABConfig fields and applies defaults.
func normalizeABConfig(cfg *ABConfig) {
	// Normalize meta endpoint
	if cfg.MetaEndpoint != "" {
		if normalized, err := normalizeEndpoint(cfg.MetaEndpoint); err == nil {
			cfg.MetaEndpoint = normalized
		}
	}

	// Normalize meta URI path
	if normalized, err := normalizeURIPath(cfg.MetaURIPath, defaultABMetaPath); err == nil {
		cfg.MetaURIPath = normalized
	} else {
		cfg.MetaURIPath = defaultABMetaPath
	}

	// Apply defaults
	if cfg.MetaLoadInterval == 0 {
		cfg.MetaLoadInterval = 1 * time.Minute
	}
}
