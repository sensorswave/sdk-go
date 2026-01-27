package sensorswave

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Client is the main interface for interacting with the SDK.
// It provides methods for event tracking and A/B test evaluation.
type Client interface {
	// Close gracefully shuts down the client, flushing any pending events.
	Close() error

	// ========== User Identity ==========

	// Identify links an anonymous ID with a login ID.
	Identify(user User) error

	// ========== Event Tracking ==========

	// TrackEvent tracks a custom event with properties.
	TrackEvent(user User, event string, properties Properties) error

	// ========== User Profile Operations ==========

	// SetUserProperties sets user profile properties ($set).
	SetUserProperties(user User, properties Properties) error

	// SetUserPropertiesOnce sets user profile properties only if they don't already exist ($set_once).
	SetUserPropertiesOnce(user User, properties Properties) error

	// IncrementUserProperties increments numeric user profile properties ($increment).
	IncrementUserProperties(user User, properties Properties) error

	// AppendUserProperties appends values to list user profile properties ($append).
	AppendUserProperties(user User, properties Properties) error

	// UnionUserProperties adds unique values to list user profile properties ($union).
	UnionUserProperties(user User, properties Properties) error

	// UnsetUserProperties removes user profile properties ($unset).
	UnsetUserProperties(user User, propertyKeys ...string) error

	// DeleteUserProfile deletes the entire user profile ($delete).
	DeleteUserProfile(user User) error

	// ========== A/B Testing ==========

	// ABEvaluate evaluates a single gate/config/experiment for a user.
	// The withImpressionLog parameter controls whether to log an impression event (default: true).
	ABEvaluate(user User, key string, withImpressionLog ...bool) (ABResult, error)

	// ABEvaluateAll evaluates all applicable gates/configs/experiments for a user.
	ABEvaluateAll(user User) ([]ABResult, error)

	// GetABSpecStorage exports the current A/B testing state for faster startup in future sessions.
	GetABSpecStorage() ([]byte, error)

	// ========== Low-level API ==========

	// Track submits a fully populated Event structure directly.
	// Use this for advanced scenarios; prefer TrackEvent for normal usage.
	Track(event Event) error
}

var _ Client = (*client)(nil)

// New creates a new SDK Client with the required endpoint and token.
func New(endpoint Endpoint, token SourceToken) (Client, error) {
	return NewWithConfig(endpoint, token, Config{})
}

// NewWithConfig creates a new SDK Client with the specified configuration.
func NewWithConfig(endpoint Endpoint, token SourceToken, cfg Config) (Client, error) {
	// Normalize configuration and apply defaults
	normalizeConfig(&cfg)
	normalizedEndpoint, err := normalizeEndpoint(string(endpoint))
	if err != nil {
		cfg.Logger.Errorf("endpoint normalize error: %v", err)
		return nil, err
	}
	if normalizedEndpoint == "" {
		if cfg.AB == nil || (cfg.AB.MetaLoader == nil && cfg.AB.MetaEndpoint == "") {
			return nil, fmt.Errorf("endpoint is required")
		}
		cfg.Logger.Warnf("endpoint is empty; tracking is disabled")
	}

	c := &client{
		endpoint:    normalizedEndpoint,
		sourceToken: string(token),
		cfg:         &cfg,
		h:           NewHTTPClient(cfg.Transport),
		quit:        make(chan struct{}),
		msgchan:     make(chan []byte, maxEventChanSize),
		sem:         make(chan struct{}, cfg.HTTPConcurrency),
	}
	for i := 0; i < cfg.HTTPConcurrency; i++ {
		c.sem <- struct{}{}
	}

	// Initialize A/B Core if configured
	if c.cfg.AB != nil {
		abc, err := NewABCore(c.endpoint, c.sourceToken, c.cfg, c.h)
		if err != nil {
			cfg.Logger.Errorf("ab core init error: %v", err)
			return nil, err
		}
		c.abCore = abc
		c.abCore.Start()
		c.cfg.Logger.Infof("sdk client initialized with A/B testing")
	} else {
		c.cfg.Logger.Infof("sdk client initialized")
	}

	// Start background loops only after all components are successfully initialized
	c.wg.Add(1)
	go c.loop()

	return c, nil
}

type client struct {
	endpoint    string
	sourceToken string
	cfg         *Config
	h           *httpClient
	quit        chan struct{}
	msgchan     chan []byte
	wg          sync.WaitGroup
	abCore      *ABCore
	sem         chan struct{}
}

func (c *client) Close() error {
	if c == nil {
		return nil
	}
	close(c.quit)
	if c.abCore != nil {
		c.abCore.Stop()
	}

	c.wg.Wait()
	c.cfg.Logger.Debugf("sdk client closed")
	return nil
}

// ========== User Identity ==========

func (c *client) Identify(user User) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	event := NewEvent(user.AnonID, user.LoginID, PseIdentify)
	return c.Track(event)
}

// ========== Event Tracking ==========

func (c *client) TrackEvent(user User, eventName string, properties Properties) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	event := NewEvent(user.AnonID, user.LoginID, eventName).
		WithProperties(NewProperties().Merge(properties))
	return c.Track(event)
}

func (c *client) Track(event Event) error {
	if len(event.AnonID) == 0 && len(event.LoginID) == 0 {
		return ErrEmptyUserIDs
	}

	if err := event.Normalize(); err != nil {
		c.cfg.Logger.Errorf("event normalize error: %v", err)
		return err
	}

	msg, err := json.Marshal(event)
	if err != nil {
		c.cfg.Logger.Errorf("event json marshal error: %v", err)
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cannot track event with closed client")
		}
	}()

	c.msgchan <- msg
	return nil
}

// ========== User Profile Operations ==========

func (c *client) SetUserProperties(user User, properties Properties) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts()
	for key, value := range properties {
		userPropertyOpts.Set(key, value)
	}

	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSet))

	return c.Track(event)
}

func (c *client) SetUserPropertiesOnce(user User, properties Properties) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts()
	for key, value := range properties {
		userPropertyOpts.SetOnce(key, value)
	}

	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSetOnce))

	return c.Track(event)
}

func (c *client) IncrementUserProperties(user User, properties Properties) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts()
	for key, value := range properties {
		userPropertyOpts.Increment(key, value)
	}

	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeIncrement))

	return c.Track(event)
}

func (c *client) AppendUserProperties(user User, properties Properties) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts()
	for key, value := range properties {
		userPropertyOpts.Append(key, value)
	}

	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeAppend))

	return c.Track(event)
}

func (c *client) UnionUserProperties(user User, properties Properties) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts()
	for key, value := range properties {
		userPropertyOpts.Union(key, value)
	}

	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnion))

	return c.Track(event)
}

func (c *client) UnsetUserProperties(user User, propertyKeys ...string) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts()
	for _, key := range propertyKeys {
		userPropertyOpts.Unset(key)
	}

	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnset))

	return c.Track(event)
}

func (c *client) DeleteUserProfile(user User) error {
	if err := c.validateUser(user); err != nil {
		return err
	}
	userPropertyOpts := NewUserPropertyOpts().Delete()
	event := NewEvent(user.AnonID, user.LoginID, PseUserSet).
		WithUserPropertyOpts(userPropertyOpts).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeDelete))

	return c.Track(event)
}

// ========== A/B Testing ==========

func (c *client) ABEvaluate(user User, key string, withImpressionLog ...bool) (ABResult, error) {
	if c.abCore == nil {
		return ABResult{}, ErrABNotInited
	}
	if c.abCore.storage() == nil {
		return ABResult{}, ErrABNotReady
	}
	if err := c.validateUser(user); err != nil {
		return ABResult{}, err
	}

	// Evaluate the gate/config/experiment with the AB core.
	result, err := c.abCore.Evaluate(user, key)
	if err != nil {
		c.cfg.Logger.Errorf("A/B test %s evaluation error: %v", key, err)
		return ABResult{}, err
	}

	// Log impression events by default unless explicitly disabled.
	logImpression := !result.DisableImpress
	if len(withImpressionLog) > 0 {
		logImpression = withImpressionLog[0]
	}

	if logImpression && result.Key != "" {
		c.logABImpression(user, result)
	}

	return result, nil
}

func (c *client) ABEvaluateAll(user User) ([]ABResult, error) {
	if c.abCore == nil {
		return nil, ErrABNotInited
	}
	if c.abCore.storage() == nil {
		return nil, ErrABNotReady
	}
	if err := c.validateUser(user); err != nil {
		return nil, err
	}

	return c.abCore.EvaluateAll(user)
}

func (c *client) GetABSpecStorage() ([]byte, error) {
	if c.abCore == nil {
		return nil, ErrABNotInited
	}
	return c.abCore.GetStorageSnapshot()
}

// ========== Internal Helpers ==========

func (c *client) validateUser(user User) error {
	if user.AnonID == "" && user.LoginID == "" {
		return ErrEmptyUserIDs
	}
	return nil
}

func (c *client) logABImpression(user User, result ABResult) {
	var userPropertyOpts UserPropertyOpts
	propertyKey := FormatABPropertyName(result.ID)

	if result.VariantID != nil {
		userPropertyOpts = NewUserPropertyOpts().Set(propertyKey, *result.VariantID)
	} else {
		userPropertyOpts = NewUserPropertyOpts().Unset(propertyKey)
	}

	event := Event{
		AnonID:         user.AnonID,
		LoginID:        user.LoginID,
		Event:          PseABImpress,
		UserProperties: userPropertyOpts,
	}

	if err := c.Track(event); err != nil {
		c.cfg.Logger.Errorf("A/B impression tracking error: %v", err)
	}
}
