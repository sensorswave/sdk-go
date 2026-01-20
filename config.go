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
	sourceToken        string
	endpoint           string // base url, without path
	trackURIPath       string
	transport          *http.Transport
	logger             Logger        // default: defaultLogger
	flushInterval      time.Duration // default 10s
	httpConcurrency    int           // default 10, max concurrent requests
	httpTimeout        time.Duration // default 3s, for each request
	httpRetry          int           // default 2, do 3 requests at most in total
	onTrackFailHandler OnTrackFailHandler
	// debug              bool

	// AB configuration
	ab *ABConfig // default nil, without Feature Flag
}

// DefaultConfig creates a new Config with default values and the provided source token.
func DefaultConfig(endpoint string, sourceToken string) *Config {
	cfg := &Config{
		sourceToken:     sourceToken,
		endpoint:        "",
		trackURIPath:    defaultTrackPath,
		logger:          &defaultLogger{},
		flushInterval:   10 * time.Second,
		httpConcurrency: 10,
		httpTimeout:     3 * time.Second,
		httpRetry:       2,
	}

	cfg.WithEndpoint(endpoint)
	return cfg
}

// WithEndpoint sets the base endpoint (host).
// It validates that the scheme is http or https, and removes any path.
func (cfg *Config) WithEndpoint(endpoint string) *Config {
	normalized, err := normalizeEndpoint(endpoint)
	if err != nil {
		fmt.Printf("sensorswave: invalid endpoint '%s': %v. Using as is.\n", endpoint, err)
		cfg.endpoint = endpoint
	} else {
		cfg.endpoint = normalized
	}
	return cfg
}

func (cfg *Config) WithTrackURIPath(trackURIPath string) *Config {
	normalized, err := normalizeURIPath(trackURIPath, defaultTrackPath)
	if err != nil {
		fmt.Printf("sensorswave: invalid track path '%s': %v. Using default.\n", trackURIPath, err)
	}
	cfg.trackURIPath = normalized
	return cfg
}

// WithTransport sets a custom HTTP transport.
func (cfg *Config) WithTransport(transport *http.Transport) *Config {
	cfg.transport = transport
	return cfg
}

// WithLogger sets a custom logger.
func (cfg *Config) WithLogger(logger Logger) *Config {
	cfg.logger = logger
	return cfg
}

// WithFlushInterval sets the interval for flushing events.
func (cfg *Config) WithFlushInterval(flushInterval time.Duration) *Config {
	cfg.flushInterval = flushInterval
	return cfg
}

// WithHTTPConcurrency sets the maximum number of concurrent HTTP requests.
func (cfg *Config) WithHTTPConcurrency(httpConcurrency int) *Config {
	cfg.httpConcurrency = httpConcurrency
	return cfg
}

// WithHTTPTimeout sets the timeout for each HTTP request.
func (cfg *Config) WithHTTPTimeout(httpTimeout time.Duration) *Config {
	cfg.httpTimeout = httpTimeout
	return cfg
}

// WithHTTPRetry sets the number of retries for HTTP requests.
func (cfg *Config) WithHTTPRetry(httpRetry int) *Config {
	cfg.httpRetry = httpRetry
	return cfg
}

// WithABConfig sets the A/B testing configuration.
func (cfg *Config) WithABConfig(abconfig *ABConfig) *Config {
	cfg.ab = abconfig
	return cfg
}

// ABConfig defines the configuration for A/B testing and feature flags.
type ABConfig struct {
	sourceToken             string
	projectSecret           string
	metaLoader              IABMetaLoader // Metadata loader
	metaEndpoint            string        // default empty, use Config.endpoint + metaURIPath
	metaURIPath             string        // URI path for signature
	loadMetaInterval        time.Duration
	localStorageForFastBoot []byte // json storage
	stickyHandler           IABStickyHandler
}

// DefaultABConfig creates a new ABConfig with default values.
func DefaultABConfig(sourceToken, projectSecret string) *ABConfig {
	abconfig := &ABConfig{
		sourceToken:      sourceToken,
		projectSecret:    projectSecret,
		metaEndpoint:     "",
		metaURIPath:      defaultABMetaPath,
		loadMetaInterval: time.Second * 10,
	}
	return abconfig
}

// WithMetaEndpoint sets the A/B metadata server endpoint.
// It validates that the scheme is http or https, and removes any path.
func (abcfg *ABConfig) WithMetaEndpoint(metaEndpoint string) *ABConfig {
	normalized, err := normalizeEndpoint(metaEndpoint)
	if err != nil {
		fmt.Printf("sensorswave: invalid meta endpoint '%s': %v. Using as is.\n", metaEndpoint, err)
		abcfg.metaEndpoint = metaEndpoint
	} else {
		abcfg.metaEndpoint = normalized
	}
	// Also attempt to extract path if the user mistakenly provided one, before normalization?
	// The user said "if filled path need to remove".
	// normalizeEndpoint does exactly that.
	return abcfg
}

// WithMetaURIPath sets the URI path specifically for signature verification.
func (abcfg *ABConfig) WithMetaURIPath(metaURIPath string) *ABConfig {
	normalized, err := normalizeURIPath(metaURIPath, defaultABMetaPath)
	if err != nil {
		fmt.Printf("sensorswave: invalid meta path '%s': %v. Using default.\n", metaURIPath, err)
	}
	abcfg.metaURIPath = normalized
	return abcfg
}

// WithMetaLoader sets a custom metadata loader.
func (abcfg *ABConfig) WithMetaLoader(loader IABMetaLoader) *ABConfig {
	abcfg.metaLoader = loader
	return abcfg
}

// WithLoadMetaInterval sets the interval for polling remote metadata.
func (abcfg *ABConfig) WithLoadMetaInterval(loadMetaInterval time.Duration) *ABConfig {
	abcfg.loadMetaInterval = loadMetaInterval
	return abcfg
}

// WithLocalStorageForFastBoot provides initial JSON metadata for faster startup.
func (abcfg *ABConfig) WithLocalStorageForFastBoot(localStorageForFastBoot []byte) *ABConfig {
	abcfg.localStorageForFastBoot = localStorageForFastBoot
	return abcfg
}

// WithStickyHandler sets a custom handler for sticky sessions.
func (abcfg *ABConfig) WithStickyHandler(stickyHandler IABStickyHandler) *ABConfig {
	abcfg.stickyHandler = stickyHandler
	return abcfg
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
	// TODO Change Default Production Endpoint Before Release
	defaultTrackPath  = "/in/track"
	defaultABMetaPath = "/api/abol/all4eval"
)

// OnTrackFailHandler is called when event tracking fails.
type OnTrackFailHandler func([]Event, error)

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
