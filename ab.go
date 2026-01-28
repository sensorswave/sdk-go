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
	for i := range abData.ABSpecs {
		spec := &abData.ABSpecs[i]
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
		s.ABSpecs[spec.Key] = *spec
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
	if len(abc.abCfg.LoadABSpecs) > 0 {
		s := storage{}
		if json.Unmarshal(abc.abCfg.LoadABSpecs, &s) == nil {
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
// If typ is provided, validates that the spec matches the expected type.
// Returns empty result if key not found or type doesn't match.
func (abc *ABCore) Evaluate(user User, key string, typ ...ABTypEnum) (result ABResult, err error) {
	spec := abc.getABSpec(key)
	if spec == nil {
		return ABResult{}, nil
	}
	// Type validation: return empty result if type doesn't match
	if len(typ) > 0 && ABTypEnum(spec.Typ) != typ[0] {
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

	for key := range storage.ABSpecs {
		spec := storage.ABSpecs[key]
		var ret ABResult
		ret, err = abc.evalAB(user, &spec, 0)
		if err != nil {
			return
		}
		if ret.ID > 0 {
			// return user eval result in all spec, if not pass , variant ID is null
			results = append(results, ret)
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

	evalID := abc.getEvalID(user, spec)
	if evalID == "" {
		return // empty evalID
	}

	var stickyDataKey string
	if spec.Sticky {
		var handled bool
		handled, stickyDataKey, err = abc.evalABSticky(spec, evalID, &result)
		if handled || err != nil {
			return
		}

		defer func() {
			if err == nil && result.VariantID != nil {
				cache := abResultCache{VariantID: result.VariantID}
				b, _ := json.Marshal(cache)
				err = abc.abCfg.StickyHandler.SetStickyResult(stickyDataKey, string(b))
			}
		}()
	}

	result.ID = spec.ID
	result.Key = spec.Key
	result.Typ = spec.Typ
	result.DisableImpress = spec.DisableImpress

	// check rules
	if ruleErr := abc.evalABRules(user, spec, evalID, index, &result); ruleErr != nil {
		err = ruleErr
		return
	}
	return
}

func (abc *ABCore) evalABRules(user User, spec *ABSpec, evalID string, index int, result *ABResult) error {
	pass := false
	defer func() {
		// gate variant ids must be standardized to "pass"/"fail"
		if spec.Typ == int(ABTypGate) && result.VariantID == nil {
			if pass {
				result.VariantID = &VariantIDPass
			} else {
				result.VariantID = &VariantIDFail
			}
		}
	}()

	// 1. check override rules (highest priority)
	if handled, err := abc.evalABOverrides(user, spec, evalID, index, result); err != nil {
		return err
	} else if handled {
		return nil
	}

	// 2. check traffic rules
	if handled, err := abc.evalABTraffic(user, spec, evalID, index, result); err != nil {
		return err
	} else if handled {
		return nil
	}

	// 3. check gate rules
	if handled, err := abc.evalABGates(user, spec, evalID, index, result); err != nil {
		return err
	} else if handled {
		pass = true
	}

	if spec.Typ != int(ABTypExp) || !pass || result.VariantID != nil {
		return nil
	}

	// 4. check group rules (only for experiments)
	return abc.evalABExperiments(user, spec, evalID, index, result)
}

func (abc *ABCore) evalABOverrides(user User, spec *ABSpec, evalID string, index int, result *ABResult) (bool, error) {
	if rules, ok := spec.Rules[RuleOverride]; ok {
		for _, rule := range rules {
			pass, err := abc.evalRule(&user, &rule, evalID, index)
			if err != nil {
				return false, err
			}
			if pass && rule.Override != nil {
				result.VariantID = rule.Override
				if spec.VariantValues != nil {
					result.VariantParamValue = spec.VariantValues[*rule.Override]
				}
				return true, nil
			}
		}
	}
	return false, nil
}

func (abc *ABCore) evalABTraffic(user User, spec *ABSpec, evalID string, index int, result *ABResult) (bool, error) {
	if rules, ok := spec.Rules[RuleTraffic]; ok {
		for _, rule := range rules {
			pass, err := abc.evalRule(&user, &rule, evalID, index)
			if err != nil {
				return false, err
			}
			if !pass {
				if rule.Override != nil {
					result.VariantID = rule.Override
				}
				return true, nil
			}
		}
	}
	return false, nil
}

func (abc *ABCore) evalABGates(user User, spec *ABSpec, evalID string, index int, result *ABResult) (bool, error) {
	if rules, ok := spec.Rules[RuleGate]; ok {
		for _, rule := range rules {
			pass, err := abc.evalRule(&user, &rule, evalID, index)
			if err != nil {
				return false, err
			}
			if pass {
				if rule.Override != nil {
					result.VariantID = rule.Override
					result.VariantParamValue = spec.VariantValues[*rule.Override]
				}
				return true, nil
			}
		}
	}
	return false, nil
}

func (abc *ABCore) getEvalID(user User, spec *ABSpec) string {
	switch {
	case strings.EqualFold(spec.SubjectID, "anon_id"):
		return user.AnonID
	case strings.EqualFold(spec.SubjectID, "login_id"):
		return user.LoginID
	default:
		return fmt.Sprintf("%v", user.ABUserProperties[spec.SubjectID])
	}
}

func (abc *ABCore) evalABSticky(spec *ABSpec, evalID string, result *ABResult) (handled bool, stickyDataKey string, err error) {
	if abc.abCfg.StickyHandler == nil {
		return false, "", ErrABWithoutSticky
	}

	stickyDataKey = fmt.Sprintf("%d-%s", spec.ID, evalID)
	cacheResult, err := abc.abCfg.StickyHandler.GetStickyResult(stickyDataKey)
	if err != nil {
		return false, stickyDataKey, err
	}

	if cacheResult != "" {
		cache := abResultCache{}
		if err := json.Unmarshal([]byte(cacheResult), &cache); err == nil {
			result.ID = spec.ID
			result.Key = spec.Key
			result.Typ = spec.Typ
			result.DisableImpress = spec.DisableImpress
			if cache.VariantID != nil {
				result.VariantID = cache.VariantID
				result.VariantParamValue = spec.VariantValues[*cache.VariantID]
			}
			return true, stickyDataKey, nil
		}
	}

	return false, stickyDataKey, nil
}

func (abc *ABCore) evalABExperiments(user User, spec *ABSpec, evalID string, index int, result *ABResult) error {
	if rules, ok := spec.Rules[RuleGroup]; ok {
		for _, rule := range rules {
			pass, err := abc.evalRule(&user, &rule, evalID, index)
			if err != nil {
				return err
			}
			if pass {
				if rule.Override != nil {
					result.VariantID = rule.Override
					result.VariantParamValue = spec.VariantValues[*rule.Override]
				}
				break
			}
		}
	}
	return nil
}

// evalRule evaluates all conditions within a rule and applies rollout logic.
func (abc *ABCore) evalRule(user *User, rule *Rule, evalID string, index int) (pass bool, err error) {
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
		switch {
		case strings.EqualFold(cond.Field, "login_id") && user.LoginID != "":
			left = user.LoginID
		case strings.EqualFold(cond.Field, "anon_id") && user.AnonID != "":
			left = user.AnonID
		default:
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
	pass, err = abc.evalCondMatch(user, cond, left, right, evalID, index)
	return
}

func (abc *ABCore) evalCondMatch(user *User, cond *Condition, left, right any, evalID string, index int) (bool, error) {
	op := cond.Opt
	switch {
	case strings.EqualFold(op, "gt"), strings.EqualFold(op, "gte"), strings.EqualFold(op, "lt"), strings.EqualFold(op, "lte"):
		return abc.evalCondNumberMatch(op, left, right), nil
	case strings.HasPrefix(strings.ToLower(op), "version_"):
		return abc.evalCondVersionMatch(op, left, right), nil
	case strings.Contains(strings.ToLower(op), "_of_"):
		return abc.evalCondArrayMatch(op, left, right), nil
	case strings.EqualFold(op, "is_null"), strings.EqualFold(op, "is_not_null"), strings.EqualFold(op, "is_true"), strings.EqualFold(op, "is_false"), strings.EqualFold(op, "eq"), strings.EqualFold(op, "neq"):
		return abc.evalCondBasicMatch(op, left, right), nil
	case strings.EqualFold(op, "before"), strings.EqualFold(op, "after"):
		return abc.evalCondTimeMatch(op, left, right), nil
	case strings.EqualFold(op, "bucket_set"):
		if bucket, ok := right.(string); ok {
			bucketBit := int(hashUint64(evalID, cond.Field) % 1000) // #nosec G115
			bucketBitmap := NewBucketBitmap(1000)
			if err := bucketBitmap.LoadNetworkByteOrderString(bucket); err != nil {
				return false, fmt.Errorf("load bucket_set failed: %w", err)
			}
			return bucketBitmap.GetBit(bucketBit) == 1, nil
		}
		return false, fmt.Errorf("unknown bucket_set type: %T", right)
	case strings.EqualFold(op, "gate_pass"):
		return abc.evalCondGateMatch(user, cond.Field, index, false)
	case strings.EqualFold(op, "gate_fail"):
		return abc.evalCondGateMatch(user, cond.Field, index, true)
	}
	return false, fmt.Errorf("unknown operator: %s", op)
}

func (abc *ABCore) evalCondNumberMatch(op string, left, right any) bool {
	switch {
	case strings.EqualFold(op, "gt"):
		return compareNumbers(left, right, func(x, y float64) bool { return x > y })
	case strings.EqualFold(op, "gte"):
		return compareNumbers(left, right, func(x, y float64) bool { return x >= y })
	case strings.EqualFold(op, "lt"):
		return compareNumbers(left, right, func(x, y float64) bool { return x < y })
	case strings.EqualFold(op, "lte"):
		return compareNumbers(left, right, func(x, y float64) bool { return x <= y })
	}
	return false
}

func (abc *ABCore) evalCondBasicMatch(op string, left, right any) bool {
	switch {
	case strings.EqualFold(op, "is_null"):
		return (left == nil)
	case strings.EqualFold(op, "is_not_null"):
		return (left != nil)
	case strings.EqualFold(op, "is_true"):
		if b, ok := left.(bool); ok {
			return b
		}
	case strings.EqualFold(op, "is_false"):
		if b, ok := left.(bool); ok {
			return !b
		}
	case strings.EqualFold(op, "eq"):
		return deepEqual(left, right)
	case strings.EqualFold(op, "neq"):
		return !deepEqual(left, right)
	}
	return false
}

func (abc *ABCore) evalCondTimeMatch(op string, left, right any) bool {
	switch {
	case strings.EqualFold(op, "before"):
		return getTime(left).Before(getTime(right))
	case strings.EqualFold(op, "after"):
		return getTime(left).After(getTime(right))
	}
	return false
}

func (abc *ABCore) evalCondVersionMatch(op string, left, right any) bool {
	switch {
	case strings.EqualFold(op, "version_gt"):
		return compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) > 0 })
	case strings.EqualFold(op, "version_gte"):
		return compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) >= 0 })
	case strings.EqualFold(op, "version_lt"):
		return compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) < 0 })
	case strings.EqualFold(op, "version_lte"):
		return compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) <= 0 })
	case strings.EqualFold(op, "version_eq"):
		return compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) == 0 })
	case strings.EqualFold(op, "version_neq"):
		return compareVersions(left, right, func(x, y []int64) bool { return compareVersionsSlice(x, y) != 0 })
	}
	return false
}

func (abc *ABCore) evalCondArrayMatch(op string, left, right any) bool {
	switch {
	case strings.EqualFold(op, "any_of_case_insensitive"):
		return arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, strings.EqualFold)
		})
	case strings.EqualFold(op, "none_of_case_insensitive"):
		return !arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, strings.EqualFold)
		})
	case strings.EqualFold(op, "any_of_case_sensitive"):
		return arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})
	case strings.EqualFold(op, "none_of_case_sensitive"):
		return !arrayAny(left, right, func(x, y interface{}) bool {
			return compareStrings(x, y, false, func(s1, s2 string) bool { return s1 == s2 })
		})
	}
	return false
}

func (abc *ABCore) evalCondGateMatch(user *User, field string, index int, invert bool) (bool, error) {
	gate := abc.getABSpec(field)
	if gate == nil {
		return false, nil
	}
	result, err := abc.evalAB(*user, gate, index)
	if err != nil {
		return false, err
	}
	pass := result.CheckFeatureGate()
	if invert {
		return !pass, nil
	}
	return pass, nil
}

// targetValue retrieves target classification values.
func (abc *ABCore) targetValue(evalID, targetKey string) any {
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
	for key := range storage.ABSpecs {
		specs = append(specs, storage.ABSpecs[key])
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
