// Package sensorswave provides the AB evaluation core logic.
package sensorswave

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// httpResponseFFLoadRemoteMeta defines the API response structure for loading remote metadata.
type httpResponseABLoadRemoteMeta struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    ABDataResp `json:"data"`
}

// ABDataResp contains the actual AB configuration data.
type ABDataResp struct {
	Update     bool     `json:"update"`
	UpdateTime int64    `json:"update_time"`
	ABEnv      ABEnv    `json:"ab_env,omitempty"`
	ABSpecs    []ABSpec `json:"ab_specs,omitempty"`
}

var (
	VariantIDPass = "pass"
	VariantIDFail = "fail"
)

// loadRemoteMeta fetches the AB metadata from the remote server.
func (abc *ABCore) loadRemoteMeta() {
	if abc.abCfg.MetaLoader == nil {
		return
	}

	abData, err := abc.abCfg.MetaLoader.LoadMeta()
	if err != nil {
		abc.logger.Errorf("[%s] ab core loadRemoteMeta failed: %v", abc.sourceToken, err)
		return
	}

	needupdate := abData.Update
	if !needupdate {
		storage := abc.storage()
		if storage != nil {
			if storage.UpdateTime != abData.UpdateTime {
				needupdate = true
			}
		} else {
			needupdate = true
		}
	}

	if !needupdate {
		abc.logger.Debugf("[%s] ab core loadRemoteMeta from server without new info", abc.sourceToken)
		return
	}

	s := storage{
		UpdateTime: abData.UpdateTime,
		ABEnv:      abData.ABEnv,
		ABSpecs:    make(map[string]ABSpec),
	}
	for _, spec := range abData.ABSpecs {
		// Parse variant payload []byte into variant value map[string]any
		for vid, payload := range spec.VariantPayloads {
			if len(payload) > 0 {
				value := make(map[string]any)
				if err = json.Unmarshal(payload, &value); err != nil {
					abc.logger.Errorf("[%s] ab core json.Unmarshal VariantPayload error: %v, payload:%s", abc.sourceToken, err, payload)
					return
				}
				if spec.VariantValues == nil {
					spec.VariantValues = make(map[string]map[string]any)
				}
				spec.VariantValues[vid] = value
			}
		}
		spec.VariantPayloads = nil // Free memory
		s.ABSpecs[spec.Key] = spec
	}

	abc.setStorage(&s)
	abc.logger.Debugf("[%s] ab core ffLoadRemoteMeta from server: [%v]", abc.sourceToken, s)
}

// ABEnv contains environment-level configurations for AB evaluations.
type ABEnv struct {
	AlwaysTrack bool `json:"always_track"` // track event even if evaluation does not pass, for accurate analysis but cost more, default false
}

// storage maintains the current AB state in memory.
type storage struct { // read only
	UpdateTime int64             // ms
	ABEnv      ABEnv             // some config from remote server
	ABSpecs    map[string]ABSpec // [key]ABSpec
}

const maxRecursionDepth = 10

// ABCore is the heart of the AB evaluation engine.
// It can be used independently for AB testing without the full Client (which includes event tracking).
type ABCore struct {
	sourceToken   string
	projectSecret string
	abCfg         *ABConfig      // AB-specific configuration
	logger        Logger         // Logger for AB operations
	storagePtr    unsafe.Pointer // unsafe.Pointer(*storage)
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	h             *httpClient
}

// NewABCore creates a new ABCore instance with the provided configuration and HTTP client.
func NewABCore(endpoint, sourceToken string, config *Config, h *httpClient) (*ABCore, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if config.AB == nil {
		return nil, fmt.Errorf("ab config is required")
	}
	if config.Logger == nil {
		config.Logger = &defaultLogger{}
	}
	normalizeABConfig(config.AB)
	abc := &ABCore{
		sourceToken: sourceToken,
		abCfg:       config.AB,
		logger:      config.Logger,
		h:           h,
	}

	abc.projectSecret = abc.abCfg.ProjectSecret
	// Initialize default meta loader if not provided
	if abc.abCfg.MetaLoader == nil {
		metaEndpoint := abc.abCfg.MetaEndpoint
		if metaEndpoint == "" {
			metaEndpoint = endpoint
		}
		// Ensure meta endpoint is normalized if it came from abc.endpoint
		if normalized, err := normalizeEndpoint(metaEndpoint); err == nil {
			metaEndpoint = normalized
		}

		metaPath := abc.abCfg.MetaURIPath
		if metaPath == "" {
			metaPath = defaultABMetaPath
		}

		if abc.abCfg.ProjectSecret == "" {
			return nil, fmt.Errorf("project secret is required when MetaLoader is nil")
		}
		abc.abCfg.MetaLoader = &HTTPSignatureMetaLoader{
			Endpoint:      metaEndpoint,
			URIPath:       metaPath,
			SourceToken:   abc.sourceToken,
			ProjectSecret: abc.abCfg.ProjectSecret,
			HTTPClient:    h,
		}
		abc.logger.Infof("ab core initialized with meta loader: [%v]", abc.abCfg.MetaLoader)
	}

	abc.ctx, abc.cancel = context.WithCancel(context.Background())
	if len(abc.abCfg.LocalStorageForFastBoot) > 0 {
		s := storage{}
		if json.Unmarshal(abc.abCfg.LocalStorageForFastBoot, &s) == nil {
			abc.setStorage(&s)
		}
	}

	return abc, nil
}

// Start initiates the meta data loading loop.
// This must be called after creating an ABCore instance to begin fetching AB metadata.
func (abc *ABCore) Start() {
	if abc.storage() == nil {
		abc.loadRemoteMeta() // fetch once at startup
	}
	abc.wg.Add(1)
	go abc.loadRemoteMetaLoop()
}

// loadRemoteMetaLoop runs periodically to refresh AB metadata.
func (abc *ABCore) loadRemoteMetaLoop() {
	defer abc.wg.Done()

	tick := time.NewTicker(abc.abCfg.MetaLoadInterval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			abc.loadRemoteMeta()
		case <-abc.ctx.Done():
			abc.logger.Debugf("ff load meta loop closed")
			return
		}
	}
}

// Stop gracefully shuts down the ABCore service.
func (abc *ABCore) Stop() {
	abc.cancel()
	abc.wg.Wait()
}

// storage returns the current AB config storage.
func (abc *ABCore) storage() *storage {
	return (*storage)(atomic.LoadPointer(&abc.storagePtr))
}

// setStorage atomically updates the internal storage pointer.
func (abc *ABCore) setStorage(s *storage) {
	atomic.StorePointer(&abc.storagePtr, unsafe.Pointer(s))
}

// getABSpec retrieves a ABSpec by its key.
func (abc *ABCore) getABSpec(key string) *ABSpec {
	storage := abc.storage()
	if storage != nil {
		if spec, ok := storage.ABSpecs[key]; ok {
			return &spec
		}
	}
	return nil
}

// Evaluate evaluates a specific AB spec for a user.
// This is the public API for single AB evaluation.
func (abc *ABCore) Evaluate(user User, key string) (result ABResult, err error) {
	spec := abc.getABSpec(key)
	if spec == nil {
		return ABResult{}, nil
	}
	return abc.evalAB(user, spec, 0)
}

// EvaluateAll evaluates all active AB specs for a user.
// This is the public API for batch AB evaluation.
func (abc *ABCore) EvaluateAll(user User) (results []ABResult, err error) {
	results = make([]ABResult, 0, 10)
	storage := abc.storage()
	if storage == nil {
		return results, nil
	}

	for _, spec := range storage.ABSpecs {
		var ret ABResult
		ret, err = abc.evalAB(user, &spec, 0)
		if err != nil {
			return
		} else {
			if ret.ID > 0 {
				// return user eval result in all spec, if not pass , variant ID is null
				results = append(results, ret)
			}
		}
	}

	return
}

// evalAB is the core evaluation logic for a single AB spec.
func (abc *ABCore) evalAB(user User, spec *ABSpec, index int) (result ABResult, err error) {
	if index >= maxRecursionDepth { // Prevent infinite recursion
		return
	}
	index++
	if !spec.Enabled {
		return // spec is disabled
	}

	var evalID string
	switch {
	case strings.EqualFold(spec.SubjectID, "anon_id"):
		evalID = user.AnonID
	case strings.EqualFold(spec.SubjectID, "login_id"):
		evalID = user.LoginID
	default:
		evalID = fmt.Sprintf("%s", user.ABUserProperties[spec.SubjectID])
	}
	if len(evalID) == 0 {
		return // empty evalID
	}

	if spec.Sticky {
		if abc.abCfg.StickyHandler != nil {
			stickyDataKey := fmt.Sprintf("%d-%s", spec.ID, evalID)
			var cacheResult string
			cacheResult, err = abc.abCfg.StickyHandler.GetStickyResult(stickyDataKey)
			if err == nil {
				if len(cacheResult) > 0 { // cache hit
					cache := abResultCache{}
					err = json.Unmarshal([]byte(cacheResult), &cache)
					if err == nil { // Parse success, supplement required fields
						result.ID = spec.ID
						result.Key = spec.Key
						result.Typ = spec.Typ
						result.DisableImpress = spec.DisableImpress
						if cache.VariantID != nil {
							result.VariantID = cache.VariantID
							result.VariantParamValue = spec.VariantValues[*cache.VariantID]
						}
						return // Success
					}
				}
			} else {
				return // Cache error
			}

			defer func() { // Cache result on exit via defer
				if err == nil && result.VariantID != nil {
					cache := abResultCache{}
					cache.VariantID = result.VariantID
					b, _ := json.Marshal(cache)
					err = abc.abCfg.StickyHandler.SetStickyResult(stickyDataKey, string(b))
				}
			}()
		} else {
			err = ErrABWithoutSticky
			return // No sticky handler registered
		}
	}

	result.ID = spec.ID
	result.Key = spec.Key
	result.Typ = spec.Typ
	result.DisableImpress = spec.DisableImpress

	// check rules
	pass := false // whether rules pass
	// Note: rules passing does not always mean a "true" result

	defer func() {
		// gate variant ids must be standardized to "pass"/"fail"
		if spec.Typ == int(ABTypGate) {
			switch {
			// normal case
			case result.VariantID == nil:
				if pass {
					result.VariantID = &VariantIDPass
				} else {
					result.VariantID = &VariantIDFail
				}
			}
		}
	}()

	// 1. check override rules (highest priority)
	if rules, ok := spec.Rules[RuleOverride]; ok {
		for _, rule := range rules {
			if pass, err = abc.evalRule(&user, &rule, evalID, index); err != nil {
				return
			} else {
				if pass && rule.Override != nil {
					// only gate overrides are "pass/fail", others are variant_id
					result.VariantID = rule.Override
					if spec.VariantValues != nil {
						result.VariantParamValue = spec.VariantValues[*rule.Override]
					}
					return
				}
			}
		}
	}

	// 2. check traffic rules
	if rules, ok := spec.Rules[RuleTraffic]; ok {
		for _, rule := range rules {
			pass, err = abc.evalRule(&user, &rule, evalID, index)
			if err != nil {
				return
			}
			// traffic must pass for evaluation to continue.
			// e.g., holdout pass means user is NOT in holdout.
			if !pass {
				if rule.Override != nil { // holdout rule requires override value
					result.VariantID = rule.Override
				}

				return
			}
		}
	}

	// 3. check gate rules
	pass = false // assume fail until proven otherwise
	if rules, ok := spec.Rules[RuleGate]; ok {
		for _, rule := range rules {
			if pass, err = abc.evalRule(&user, &rule, evalID, index); err != nil {
				return
			}
			if pass {
				if rule.Override != nil {
					result.VariantID = rule.Override
					result.VariantParamValue = spec.VariantValues[*rule.Override]
				}
				break // first match wins
			}
		}
	}

	if spec.Typ != int(ABTypExp) {
		return // Done for non-experiment types
	}

	// For Experiments, if gate failed, stop here
	if !pass {
		return
	}
	if result.VariantID != nil {
		return // already has an override (e.g. forced rollout)
	}

	// 4. check group rules (only for experiments)
	if rules, ok := spec.Rules[RuleGroup]; ok {
		for _, rule := range rules {
			if pass, err = abc.evalRule(&user, &rule, evalID, index); err != nil {
				return
			}
			if pass {
				if rule.Override != nil {
					result.VariantID = rule.Override
					result.VariantParamValue = spec.VariantValues[*rule.Override]
				}
				break // first match wins
			}
		}
	}

	return
}

// evalRule evaluates all conditions within a rule and applies rollout logic.
func (abc *ABCore) evalRule(user *User, rule *Rule, evalID string, index int) (pass bool, err error) {
	pass = true
	if rule.Rollout == 0.0 {
		return false, nil
	}
	for _, cond := range rule.Conditions {
		pass, err = abc.evalCond(user, &cond, evalID, index)
		if err != nil {
			return false, err
		}
		if !pass {
			return false, nil
		}
	}

	// check rollout if all conditions pass
	if rule.Rollout == 100.0 {
		return true, nil
	} else {
		h64 := hashUint64(evalID, rule.Salt)
		pass = h64%10000 < uint64(rule.Rollout*100)
	}
	return
}

// evalCond evaluates a single condition.
func (abc *ABCore) evalCond(user *User, cond *Condition, evalID string, index int) (pass bool, err error) {
	// Preprocess left value
	var left, right any
	ok := false
	switch {
	case strings.EqualFold(cond.FieldClass, "common"):
		if strings.EqualFold(cond.Field, "public") {
			return true, nil
		} else {
			return false, fmt.Errorf("unknown common field: %s", cond.Field)
		}
	case strings.EqualFold(cond.FieldClass, "ffuser"):
		if strings.EqualFold(cond.Field, "login_id") && user.LoginID != "" {
			left = user.LoginID
		} else if strings.EqualFold(cond.Field, "anon_id") && user.AnonID != "" {
			left = user.AnonID
		} else {
			left = nil // user attribute missing, set left to nil for matching
		}
	case strings.EqualFold(cond.FieldClass, "props"):
		if left, ok = user.ABUserProperties[cond.Field]; ok {
		} else {
			left = nil // props missing, set left to nil
		}
	case strings.EqualFold(cond.FieldClass, "target"):
		left = abc.targetValue(evalID, cond.Field)
	default:
		left = cond.Field
	}

	right = cond.Value
	op := cond.Opt
	switch {
	case strings.EqualFold(op, "gt"):
		pass = compareNumbers(left, right, func(x, y float64) bool { return x > y })
	case strings.EqualFold(op, "gte"):
		pass = compareNumbers(left, right, func(x, y float64) bool { return x >= y })
	case strings.EqualFold(op, "lt"):
		pass = compareNumbers(left, right, func(x, y float64) bool { return x < y })
	case strings.EqualFold(op, "lte"):
		pass = compareNumbers(left, right, func(x, y float64) bool { return x <= y })
	case strings.EqualFold(op, "version_gt"):
		pass = compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) > 0 })
	case strings.EqualFold(op, "version_gte"):
		pass = compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) >= 0 })
	case strings.EqualFold(op, "version_lt"):
		pass = compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) < 0 })
	case strings.EqualFold(op, "version_lte"):
		pass = compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) <= 0 })
	case strings.EqualFold(op, "version_eq"):
		pass = compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) == 0 })
	case strings.EqualFold(op, "version_neq"):
		pass = compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) != 0 })
	case strings.EqualFold(op, "any_of_case_insensitive"):
		pass = arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return strings.EqualFold(s1, s2) })
		})
	case strings.EqualFold(op, "none_of_case_insensitive"):
		pass = !arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return strings.EqualFold(s1, s2) })
		})
	case strings.EqualFold(op, "any_of_case_sensitive"):
		pass = arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})
	case strings.EqualFold(op, "none_of_case_sensitive"):
		pass = !arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})
	case strings.EqualFold(op, "is_null"):
		pass = (left == nil)
	case strings.EqualFold(op, "is_not_null"):
		pass = (left != nil)
	case strings.EqualFold(op, "is_true"):
		if b, ok := left.(bool); ok {
			pass = b
		}
	case strings.EqualFold(op, "is_false"):
		if b, ok := left.(bool); ok {
			pass = !b
		}
	case strings.EqualFold(op, "eq"): // strict equality
		pass = deepEqual(left, right)
	case strings.EqualFold(op, "neq"):
		pass = !deepEqual(left, right)
	case strings.EqualFold(op, "before"): // time
		pass = getTime(left).Before(getTime(right))
	case strings.EqualFold(op, "after"):
		pass = getTime(left).After(getTime(right))

	// special operator for experiment
	case strings.EqualFold(op, "bucket_set"):
		if bucket, ok := right.(string); ok {
			bucketBit := hashUint64(evalID, cond.Field) % 1000
			bucketBitmap := NewBucketBitmap(1000)
			bucketBitmap.LoadNetworkByteOrderString(bucket)
			pass = bucketBitmap.GetBit(int(bucketBit)) == 1
		} else {
			return false, fmt.Errorf("unknown bucket_set type: %T", right)
		}
	case strings.EqualFold(op, "gate_pass"):
		gate := abc.getABSpec(cond.Field)
		if gate != nil {
			result, err1 := abc.evalAB(*user, gate, index)
			if err1 != nil {
				return false, err1
			} else {
				return result.CheckGate(), nil
			}
		}
	case strings.EqualFold(op, "gate_fail"):
		gate := abc.getABSpec(cond.Field)
		if gate != nil {
			result, err1 := abc.evalAB(*user, gate, index)
			if err1 != nil {
				return false, err1
			} else {
				return !result.CheckGate(), nil
			}
		}

	default:
		return false, fmt.Errorf("unknown operator: %s", cond.Opt)
	}

	return
}

// targetValue retrieves target classification values.
func (abc *ABCore) targetValue(evalID string, targetKey string) any {
	// TODO: Integrate cohort/tag handlers here
	var val any
	return val
}

// GetABSpecs retrieves the cached AB specs (for abol export)
func (abc *ABCore) GetABSpecs() ([]ABSpec, int64, error) {
	storage := abc.storage()
	if storage == nil {
		return nil, 0, nil
	}

	specs := make([]ABSpec, 0, len(storage.ABSpecs))
	for _, spec := range storage.ABSpecs {
		specs = append(specs, spec)
	}

	return specs, storage.UpdateTime, nil
}

// GetStorageSnapshot returns the complete AB storage state as JSON bytes.
// This is useful for exporting the full AB metadata including ABEnv and UpdateTime.
// It serializes the entire storage structure, which is heavier than GetABSpecs but contains complete information.
func (abc *ABCore) GetStorageSnapshot() ([]byte, error) {
	storage := abc.storage()
	if storage == nil {
		return nil, ErrABNotReady
	}
	return json.Marshal(storage)
}
