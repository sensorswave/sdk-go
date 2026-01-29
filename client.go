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

	// ProfileSet sets user profile properties ($set).
	ProfileSet(user User, properties Properties) error

	// ProfileSetOnce sets user profile properties only if they don't already exist ($set_once).
	ProfileSetOnce(user User, properties Properties) error

	// ProfileIncrement increments numeric user profile properties ($increment).
	ProfileIncrement(user User, properties Properties) error

	// ProfileAppend appends values to list user profile properties ($append).
	// Allows duplicates in the list.
	ProfileAppend(user User, properties ListProperties) error

	// ProfileUnion adds unique values to list user profile properties ($union).
	ProfileUnion(user User, properties ListProperties) error

	// ProfileUnset removes user profile properties ($unset).
	ProfileUnset(user User, propertyKeys ...string) error

	// ProfileDelete deletes the entire user profile ($delete).
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

	// GetABSpecs exports the current A/B testing state for faster startup in future sessions.
	GetABSpecs() ([]byte, error)

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

// Identify links an anonymous ID with a login ID.
// Both AnonID and LoginID must be non-empty.
func (c *client) Identify(user User) error {
	if user.AnonID == "" || user.LoginID == "" {
		return ErrIdentifyRequiredBothIDs
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
	if event.AnonID == "" && event.LoginID == "" {
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

func (c *client) ProfileSet(user User, properties Properties) error {
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

func (c *client) ProfileSetOnce(user User, properties Properties) error {
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

func (c *client) ProfileIncrement(user User, properties Properties) error {
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

func (c *client) ProfileAppend(user User, properties ListProperties) error {
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

func (c *client) ProfileUnion(user User, properties ListProperties) error {
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

func (c *client) ProfileUnset(user User, propertyKeys ...string) error {
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

func (c *client) ProfileDelete(user User) error {
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

func (c *client) CheckFeatureGate(user User, key string) (bool, error) {
	if c.abCore == nil {
		return false, ErrABNotInited
	}
	if c.abCore.storage() == nil {
		return false, ErrABNotReady
	}
	if err := c.validateUser(user); err != nil {
		return false, err
	}

	result, err := c.abCore.Evaluate(user, key, ABTypGate)
	if err != nil {
		c.cfg.Logger.Errorf("feature gate %s evaluation error: %v", key, err)
		return false, err
	}

	if !result.DisableImpress && result.Key != "" {
		c.logABImpression(user, result)
	}

	return result.CheckFeatureGate(), nil
}

func (c *client) GetFeatureConfig(user User, key string) (ABResult, error) {
	if c.abCore == nil {
		return ABResult{}, ErrABNotInited
	}
	if c.abCore.storage() == nil {
		return ABResult{}, ErrABNotReady
	}
	if err := c.validateUser(user); err != nil {
		return ABResult{}, err
	}

	result, err := c.abCore.Evaluate(user, key, ABTypConfig)
	if err != nil {
		c.cfg.Logger.Errorf("feature config %s evaluation error: %v", key, err)
		return ABResult{}, err
	}

	if !result.DisableImpress && result.Key != "" {
		c.logABImpression(user, result)
	}

	return result, nil
}

func (c *client) GetExperiment(user User, key string) (ABResult, error) {
	if c.abCore == nil {
		return ABResult{}, ErrABNotInited
	}
	if c.abCore.storage() == nil {
		return ABResult{}, ErrABNotReady
	}
	if err := c.validateUser(user); err != nil {
		return ABResult{}, err
	}

	result, err := c.abCore.Evaluate(user, key, ABTypExp)
	if err != nil {
		c.cfg.Logger.Errorf("experiment %s evaluation error: %v", key, err)
		return ABResult{}, err
	}

	if !result.DisableImpress && result.Key != "" {
		c.logABImpression(user, result)
	}

	return result, nil
}

func (c *client) GetABSpecs() ([]byte, error) {
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
