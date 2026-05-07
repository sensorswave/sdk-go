package conformance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// specStorageCache 缓存 testdata 下解析后的 storage JSON。
// ab-core fixture 116 cases 共享 49 个 unique spec_file，缓存后避免 67 次重复读+解析。
var (
	specStorageCache   = make(map[string][]byte)
	specStorageCacheMu sync.Mutex
)

// TestABCoreEvaluationConformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/ab-core-evaluation.json
// + spec_file 间接引用 testdata/{config,gate,exp}/ 下的 spec JSON。
//
// 派生测试在 SubTest 内加载 spec，经 `LoadABSpecs` 公开 API 构造 ABCore，
// 调 `core.Evaluate` 拿评估结果。
// loader（loadConformance）保持 capability-agnostic 不变。
func TestABCoreEvaluationConformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "ab-core-evaluation")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				EvalMode      string            `json:"eval_mode"`
				EvalType      string            `json:"eval_type"`
				SpecFile      string            `json:"spec_file"`
				Key           string            `json:"key"`
				User          *userInput        `json:"user"`
				UserPrefix    string            `json:"user_prefix"`
				UserCount     int               `json:"user_count"`
				StickyCache   map[string]string `json:"sticky_cache"`
				ExpectedError bool              `json:"expected_error"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			core := loadABCoreFromSpecFile(t, input.SpecFile, input.StickyCache)
			evalTyp := parseEvalTypeForConformance(t, input.EvalType)

			if input.EvalMode == "distribution" {
				actual := evaluateDistribution(t, core, input.UserPrefix, input.UserCount, input.Key, evalTyp)
				expected := []map[string]any{}
				require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected))
				require.Equal(t, expected, actual, "distribution result should match golden")
				return
			}

			require.NotNil(t, input.User, "single mode requires user")
			user := User{
				LoginID:          input.User.LoginID,
				AnonID:           input.User.AnonID,
				ABUserProperties: input.User.ABUserProps,
			}
			result, err := core.Evaluate(user, input.Key, evalTyp)

			// expected_error 模式：fixture 标记本 case 应抛错；只验"有 err"，不锁
			// 错误消息（各 SDK 实现差异；与 runner result_comparator 对齐）。
			if input.ExpectedError {
				require.Error(t, err, "expected error, got result %v", result)
				return
			}

			require.NoError(t, err, "Evaluate")

			actual := singleResultToMap(&result)
			expected := map[string]any{}
			require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected))
			require.Equal(t, expected, actual, "single eval result should match golden")
		})
	}
}

// ---------- 辅助类型与函数（仅 ab-core-evaluation 派生测试使用） ----------

type userInput struct {
	LoginID     string         `json:"login_id"`
	AnonID      string         `json:"anon_id"`
	ABUserProps map[string]any `json:"ab_user_props"`
}

// loadABCoreFromSpecFile 加载 testdata 目录下的 spec JSON、构造 storage、
// 通过 LoadABSpecs 公开 API 构造 ABCore。与 conformance Go adapter 同义。
func loadABCoreFromSpecFile(t *testing.T, specFile string, stickyCache map[string]string) *ABCore {
	t.Helper()

	storageJSON := loadStorageJSONForSpec(t, specFile)

	handler := &conformanceMemoryStickyHandler{data: map[string]string{}}
	for k, v := range stickyCache {
		handler.data[k] = v
	}

	cfg := &Config{
		Logger: &noopLogger{},
		AB: &ABConfig{
			ProjectSecret: "test-secret",
			LoadABSpecs:   storageJSON,
			StickyHandler: handler,
		},
	}
	core, err := NewABCore("http://example.com", "test-token", cfg, nil)
	require.NoError(t, err, "NewABCore")
	return core
}

func loadStorageJSONForSpec(t *testing.T, specFile string) []byte {
	t.Helper()

	specStorageCacheMu.Lock()
	if cached, ok := specStorageCache[specFile]; ok {
		specStorageCacheMu.Unlock()
		return cached
	}
	specStorageCacheMu.Unlock()

	specPath := filepath.Join("..", "testdata", specFile)
	specBytes, err := os.ReadFile(specPath)
	require.NoError(t, err, "read spec %s", specPath)

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Update    bool     `json:"update"`
			UpdatedAt int64    `json:"updated_at"`
			ABEnv     ABEnv    `json:"ab_env"`
			ABSpecs   []ABSpec `json:"ab_specs"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(specBytes, &payload), "unmarshal spec %s", specPath)

	type storageFormat struct {
		UpdateTime int64             `json:"UpdateTime"`
		ABEnv      ABEnv             `json:"ABEnv"`
		ABSpecs    map[string]ABSpec `json:"ABSpecs"`
	}
	store := storageFormat{
		UpdateTime: payload.Data.UpdatedAt,
		ABEnv:      payload.Data.ABEnv,
		ABSpecs:    make(map[string]ABSpec, len(payload.Data.ABSpecs)),
	}
	for _, spec := range payload.Data.ABSpecs {
		if spec.VariantPayloads != nil {
			spec.VariantValues = make(map[string]map[string]any)
			for k, raw := range spec.VariantPayloads {
				var m map[string]any
				if json.Unmarshal(raw, &m) == nil {
					spec.VariantValues[k] = m
				}
			}
		}
		store.ABSpecs[spec.Key] = spec
	}
	storageJSON, err := json.Marshal(store)
	require.NoError(t, err, "marshal storage")

	specStorageCacheMu.Lock()
	specStorageCache[specFile] = storageJSON
	specStorageCacheMu.Unlock()
	return storageJSON
}

type conformanceMemoryStickyHandler struct {
	data map[string]string
}

func (m *conformanceMemoryStickyHandler) GetStickyResult(key string) (string, error) {
	return m.data[key], nil
}

func (m *conformanceMemoryStickyHandler) SetStickyResult(key, result string) error {
	m.data[key] = result
	return nil
}

func parseEvalTypeForConformance(t *testing.T, s string) ABTypEnum {
	t.Helper()
	switch s {
	case "gate":
		return ABTypGate
	case "config":
		return ABTypConfig
	case "experiment":
		return ABTypExp
	}
	t.Fatalf("unknown eval_type: %s", s)
	return 0
}

func singleResultToMap(r *ABResult) map[string]any {
	out := map[string]any{
		"check_gate":     r.CheckFeatureGate(),
		"variant_id":     nil,
		"variant_params": nil,
	}
	if r.VariantID != nil {
		out["variant_id"] = *r.VariantID
	}
	if r.VariantParamValue != nil {
		out["variant_params"] = r.VariantParamValue
	}
	return out
}

func evaluateDistribution(t *testing.T, core *ABCore, userPrefix string, userCount int, key string, evalTyp ABTypEnum) []map[string]any {
	t.Helper()
	results := make([]map[string]any, 0, userCount)
	for i := 0; i < userCount; i++ {
		uid := fmt.Sprintf("%s-%d", userPrefix, i)
		user := User{LoginID: uid}
		r, err := core.Evaluate(user, key, evalTyp)
		require.NoError(t, err, "Evaluate user %s", uid)
		entry := map[string]any{"user_id": uid, "variant_id": nil}
		if r.VariantID != nil {
			entry["variant_id"] = *r.VariantID
		}
		results = append(results, entry)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i]["user_id"].(string) < results[j]["user_id"].(string)
	})
	return results
}
