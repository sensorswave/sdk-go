package sensorswave // SDK information

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Client is the main interface for interacting with the SDK.
// It provides methods for event tracking and A/B test evaluation.
type Client interface {
	// Close gracefully shuts down the client, flushing any pending events.
	Close() (err error)

	// Identify links an anonymous ID with a login ID.
	Identify(anonID string, loginID string) error

	// TrackEvent asynchronously submits an event with custom properties.
	TrackEvent(anonID string, loginID string, event string, props Properties) error

	// ProfileSet sets user profile properties ($set).
	ProfileSet(anonID string, loginID string, props Properties) error
	// ProfileSetOnce sets user profile properties only if they don't already exist ($set_once).
	ProfileSetOnce(anonID string, loginID string, props Properties) error
	// ProfileIncrement increments numeric user profile properties ($increment).
	ProfileIncrement(anonID string, loginID string, props Properties) error
	// ProfileAppend appends values to a list user profile property ($append).
	ProfileAppend(anonID string, loginID string, props Properties) error
	// ProfileUnion adds unique values to a list user profile property ($union).
	ProfileUnion(anonID string, loginID string, props Properties) error
	// ProfileUnset removes user profile properties ($unset).
	ProfileUnset(anonID string, loginID string, properties ...string) error
	// ProfileDelete deletes a user profile ($delete).
	ProfileDelete(anonID string, loginID string) error

	// Track submits a fully populated Event structure.
	Track(event Event) error

	// ABEval evaluates a single feature flag or A/B test for a user.
	ABEval(user ABUser, key string, withLog ...bool) (ABResult, error)

	// ABEvalAll evaluates all applicable feature flags and A/B tests for a user.
	ABEvalAll(user ABUser) (results []ABResult, err error)

	// ABStorageForFastBoot exports the current A/B testing state for faster startup in future sessions.
	ABStorageForFastBoot() (storage []byte, err error)

	// GetABSpecs retrieves the raw A/B testing specifications and their update time.
	GetABSpecs() ([]ABSpec, int64, error)
}

var _ Client = (*client)(nil)

// New creates and starts a new SDK Client with the provided configuration.
func New(cfg *Config) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if cfg.endpoint == "" {
		if cfg.ab == nil || (cfg.ab.metaLoader == nil && cfg.ab.metaEndpoint == "") {
			return nil, fmt.Errorf("endpoint is required")
		}
		cfg.logger.Warnf("endpoint is empty; tracking is disabled")
	}

	c := &client{
		cfg:     cfg,
		h:       NewHTTPClient(cfg.transport),
		quit:    make(chan struct{}),
		msgchan: make(chan []byte, maxEventChanSize),
		sem:     make(chan struct{}, cfg.httpConcurrency),
	}
	for i := 0; i < cfg.httpConcurrency; i++ {
		c.sem <- struct{}{}
	}

	c.wg.Add(1)
	go c.loop()

	if c.cfg.ab != nil {
		c.abCore = NewABCore(c.cfg, c.h)
		c.abCore.start()
		c.cfg.logger.Debugf("sdk client initialized with ab")
	} else {
		c.cfg.logger.Debugf("sdk client initialized")
	}

	return c, nil
}

type client struct {
	cfg     *Config
	h       *httpClient
	quit    chan struct{}
	msgchan chan []byte
	wg      sync.WaitGroup
	abCore  *abCore
	sem     chan struct{}
	// sourceToken string
	// closed  bool
	// mu      sync.RWMutex
}

func (c *client) Close() (err error) {
	if c == nil {
		return
	}
	close(c.quit)
	if c.abCore != nil {
		c.abCore.stop()
	}

	c.wg.Wait()
	c.cfg.logger.Debugf("sdk client closed")
	return
}

func (c *client) Identify(anonID string, loginID string) error {
	e := NewEvent(anonID, loginID, PseSignUp)
	return c.Track(e)
}

func (c *client) TrackEvent(anonID string, loginID string, event string, props Properties) error {
	e := NewEvent(anonID, loginID, event).WithProperties(NewProperties().Merge(props))
	return c.Track(e)
}

func (c *client) Track(event Event) (err error) {
	if len(event.AnonID) == 0 && len(event.LoginID) == 0 {
		return ErrEmptyUserIDs
	}

	err = event.Normalize()
	if err != nil {
		c.cfg.logger.Errorf("event normalize error: %v", err)
		return
	}

	msg, err := json.Marshal(event)
	if err != nil {
		c.cfg.logger.Errorf("event json marchal error: %v", err)
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cannot track event with closed client")
		}
	}()

	c.msgchan <- msg
	return
}

func (c *client) ProfileSet(anonID string, loginID string, props Properties) error {
	upo := NewUserPropertyOpts()
	for k, v := range props {
		upo.Set(k, v)
	}

	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSet))

	return c.Track(e)
}

func (c *client) ProfileSetOnce(anonID string, loginID string, props Properties) error {
	upo := NewUserPropertyOpts()
	for k, v := range props {
		upo.SetOnce(k, v)
	}

	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeSetOnce))

	return c.Track(e)
}

func (c *client) ProfileIncrement(anonID string, loginID string, props Properties) error {
	upo := NewUserPropertyOpts()
	for k, v := range props {
		upo.Increment(k, v)
	}

	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeIncrement))

	return c.Track(e)
}

func (c *client) ProfileAppend(anonID string, loginID string, props Properties) error {
	upo := NewUserPropertyOpts()
	for k, v := range props {
		upo.Append(k, v)
	}

	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeAppend))

	return c.Track(e)
}

func (c *client) ProfileUnion(anonID string, loginID string, props Properties) error {
	upo := NewUserPropertyOpts()
	for k, v := range props {
		upo.Increment(k, v)
	}

	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnion))

	return c.Track(e)
}

func (c *client) ProfileUnset(anonID string, loginID string, properties ...string) error {
	upo := NewUserPropertyOpts()
	for _, k := range properties {
		upo.Unset(k)
	}

	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeUnset))

	return c.Track(e)
}

func (c *client) ProfileDelete(anonID string, loginID string) error {
	upo := NewUserPropertyOpts().Delete()
	e := NewEvent(anonID, loginID, PseUserSet).
		WithUserPropertyOpts(upo).
		WithProperties(NewProperties().Set(PspUserSetType, UserSetTypeDelete))

	return c.Track(e)
}

func (c *client) ABEval(user ABUser, key string, withoutLogImpress ...bool) (result ABResult, err error) {
	if c.abCore == nil {
		return ABResult{}, ErrABNotInited // eval to server?
	}
	if c.abCore.storage() == nil {
		return ABResult{}, ErrABNotReady
	}

	if c.isUserInvalid(&user) {
		return ABResult{}, ErrEmptyUserIDs
	}

	result, err = c.abCore.eval(user, key)
	if err != nil {
		c.cfg.logger.Errorf("AB:%s eval error: %v", key, err)
		return ABResult{}, err
	}

	logImpress := !result.DisableImpress // log impress event by default, true
	if len(withoutLogImpress) > 0 {
		logImpress = withoutLogImpress[0]
	}

	anonID := user.AnonID
	loginID := user.LoginID

	if logImpress && result.Key != "" {
		var up UserPropertyOpts
		// Use the $ab_{id} format directly.
		key := FormatABPropertyName(result.ID)
		val := result.VariantID
		if val != nil {
			up = NewUserPropertyOpts().Set(key, *val)
		} else {
			up = NewUserPropertyOpts().Unset(key)
		}

		event := Event{
			AnonID:         anonID,
			LoginID:        loginID,
			Event:          PseABImpress,
			UserProperties: up,
		}
		err := c.Track(event)
		if err != nil {
			c.cfg.logger.Errorf("AB result track error: %v", err)
		}
	}
	return
}

func (c *client) ABEvalAll(user ABUser) (results []ABResult, err error) {
	if c.abCore == nil {
		return results, ErrABNotInited
	}
	if c.abCore.storage() == nil {
		return results, ErrABNotReady
	}
	if c.isUserInvalid(&user) {
		return results, ErrEmptyUserIDs
	}

	return c.abCore.evalAll(user)
}

func (c *client) ABStorageForFastBoot() (storage []byte, err error) {
	if c.abCore == nil {
		return storage, ErrABNotInited
	}
	if ptr := c.abCore.storage(); ptr == nil {
		return storage, ErrABNotReady
	} else {
		storage, err = json.Marshal(ptr)
	}

	return
}

func (c *client) GetABSpecs() (specs []ABSpec, updateTime int64, err error) {
	if c.abCore == nil {
		return nil, 0, ErrABNotInited
	}
	return c.abCore.GetABSpecs()
}
