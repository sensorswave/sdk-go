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
func (ffc *abCore) loadRemoteMeta() {
	if ffc.cfg.AB.MetaLoader == nil {
		return
	}

	abData, err := ffc.cfg.AB.MetaLoader.LoadMeta()
	if err != nil {
		ffc.cfg.Logger.Errorf("[%s] ab core loadRemoteMeta failed: %v", ffc.sourceToken, err)
		return
	}

	needupdate := abData.Update
	if !needupdate {
		storage := ffc.storage()
		if storage != nil {
			if storage.UpdateTime != abData.UpdateTime {
				needupdate = true
			}
		} else {
			needupdate = true
		}
	}

	if !needupdate {
		ffc.cfg.Logger.Debugf("[%s] ab core loadRemoteMeta from server without new info", ffc.sourceToken)
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
					ffc.cfg.Logger.Errorf("[%s] ab core json.Unmarshal VariantPayload error: %v, payload:%s", ffc.sourceToken, err, payload)
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

	ffc.setStorage(&s)
	ffc.cfg.Logger.Debugf("[%s] ab core ffLoadRemoteMeta from server: [%v]", ffc.sourceToken, s)
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

// abCore is the heart of the AB evaluation engine.
type abCore struct {
	sourceToken   string
	projectSecret string
	cfg           *Config
	storagePtr    unsafe.Pointer // unsafe.Pointer(*storage)
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	h             *httpClient
}

// NewABCore creates a new abCore instance with the provided configuration and HTTP client.
func NewABCore(endpoint, sourceToken string, config *Config, h *httpClient) (*abCore, error) {
	ffc := &abCore{
		sourceToken: sourceToken,
		cfg:         config,
		h:           h,
	}

	ffc.projectSecret = ffc.cfg.AB.ProjectSecret
	// Initialize default meta loader if not provided
	if ffc.cfg.AB.MetaLoader == nil {
		metaEndpoint := ffc.cfg.AB.MetaEndpoint
		if metaEndpoint == "" {
			metaEndpoint = endpoint
		}
		// Ensure meta endpoint is normalized if it came from ffc.endpoint
		if normalized, err := normalizeEndpoint(metaEndpoint); err == nil {
			metaEndpoint = normalized
		}

		metaPath := ffc.cfg.AB.MetaURIPath
		if metaPath == "" {
			metaPath = defaultABMetaPath
		}

		if ffc.cfg.AB.ProjectSecret == "" {
			return nil, fmt.Errorf("project secret is required when MetaLoader is nil")
		}
		ffc.cfg.AB.MetaLoader = &HTTPSignatureMetaLoader{
			Endpoint:      metaEndpoint,
			URIPath:       metaPath,
			SourceToken:   ffc.sourceToken,
			ProjectSecret: ffc.cfg.AB.ProjectSecret,
			HTTPClient:    h,
		}
		ffc.cfg.Logger.Infof("ab core initialized with meta loader: [%v]", ffc.cfg.AB.MetaLoader)
	}

	ffc.ctx, ffc.cancel = context.WithCancel(context.Background())
	if len(ffc.cfg.AB.LocalStorageForFastBoot) > 0 {
		s := storage{}
		if json.Unmarshal(ffc.cfg.AB.LocalStorageForFastBoot, &s) == nil {
			ffc.setStorage(&s)
		}
	}

	return ffc, nil
}

// start initiates the meta data loading loop.
func (ffc *abCore) start() {
	if ffc.storage() == nil {
		ffc.loadRemoteMeta() // fetch once at startup
	}
	ffc.wg.Add(1)
	go ffc.loadRemoteMetaLoop()
}

// loadRemoteMetaLoop runs periodically to refresh AB metadata.
func (ffc *abCore) loadRemoteMetaLoop() {
	defer ffc.wg.Done()

	tick := time.NewTicker(ffc.cfg.AB.MetaLoadInterval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			ffc.loadRemoteMeta()
		case <-ffc.ctx.Done():
			ffc.cfg.Logger.Debugf("ff load meta loop closed")
			return
		}
	}
}

// stop gracefully shuts down the FFCore service.
func (ffc *abCore) stop() {
	ffc.cancel()
	ffc.wg.Wait()
}

// storage returns the current AB config storage.
func (ffc *abCore) storage() *storage {
	return (*storage)(atomic.LoadPointer(&ffc.storagePtr))
}

// setStorage atomically updates the internal storage pointer.
func (ffc *abCore) setStorage(s *storage) {
	atomic.StorePointer(&ffc.storagePtr, unsafe.Pointer(s))
}

// getABSpec retrieves a ABSpec by its key.
func (ffc *abCore) getABSpec(key string) *ABSpec {
	storage := ffc.storage()
	if storage != nil {
		if spec, ok := storage.ABSpecs[key]; ok {
			return &spec
		}
	}
	return nil
}

// eval evaluates a specific AB spec for a user.
func (ffc *abCore) eval(user User, key string) (result ABResult, err error) {
	spec := ffc.getABSpec(key)
	if spec == nil {
		return ABResult{}, nil
	}
	return ffc.evalAB(user, spec, 0)
}

// evalAll evaluates all active AB specs for a user.
func (ffc *abCore) evalAll(user User) (results []ABResult, err error) {
	results = make([]ABResult, 0, 10)
	storage := ffc.storage()

	for _, spec := range storage.ABSpecs {
		var ret ABResult
		ret, err = ffc.evalAB(user, &spec, 0)
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
func (ffc *abCore) evalAB(user User, spec *ABSpec, index int) (result ABResult, err error) {
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
		if ffc.cfg.AB.StickyHandler != nil {
			stickyDataKey := fmt.Sprintf("%d-%s", spec.ID, evalID)
			var cacheResult string
			cacheResult, err = ffc.cfg.AB.StickyHandler.GetStickyResult(stickyDataKey)
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
					err = ffc.cfg.AB.StickyHandler.SetStickyResult(stickyDataKey, string(b))
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
			if pass, err = ffc.evalRule(&user, &rule, evalID, index); err != nil {
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
			pass, err = ffc.evalRule(&user, &rule, evalID, index)
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
			if pass, err = ffc.evalRule(&user, &rule, evalID, index); err != nil {
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
			if pass, err = ffc.evalRule(&user, &rule, evalID, index); err != nil {
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
func (ffc *abCore) evalRule(user *User, rule *Rule, evalID string, index int) (pass bool, err error) {
	pass = true
	if rule.Rollout == 0.0 {
		return false, nil
	}
	for _, cond := range rule.Conditions {
		pass, err = ffc.evalCond(user, &cond, evalID, index)
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
func (ffc *abCore) evalCond(user *User, cond *Condition, evalID string, index int) (pass bool, err error) {
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
		left = ffc.targetValue(evalID, cond.Field)
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
		gate := ffc.getABSpec(cond.Field)
		if gate != nil {
			result, err1 := ffc.evalAB(*user, gate, index)
			if err1 != nil {
				return false, err1
			} else {
				return result.CheckGate(), nil
			}
		}
	case strings.EqualFold(op, "gate_fail"):
		gate := ffc.getABSpec(cond.Field)
		if gate != nil {
			result, err1 := ffc.evalAB(*user, gate, index)
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
func (ffc *abCore) targetValue(evalID string, targetKey string) any {
	// TODO: Integrate cohort/tag handlers here
	var val any
	return val
}

// GetABSpecs retrieves the cached AB specs (for abol export)
func (ffc *abCore) GetABSpecs() ([]ABSpec, int64, error) {
	storage := ffc.storage()
	if storage == nil {
		return nil, 0, nil
	}

	specs := make([]ABSpec, 0, len(storage.ABSpecs))
	for _, spec := range storage.ABSpecs {
		specs = append(specs, spec)
	}

	return specs, storage.UpdateTime, nil
}
