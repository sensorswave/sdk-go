package conformance

import (
	"encoding/json"
	"testing"

	. "github.com/sensorswave/sdk-go" //lint:ignore ST1001 dot import intentional; see conformance pkg doc.

	"github.com/stretchr/testify/require"
)

// TestRequestSigningACS3Conformance — A 类完全派生测试。
//
// 输入与期望值都来自 conformance/{fixtures,golden}/request-signing-acs3.json
// 不在本测试代码里硬编码任何 spec 字面量或 expected 值。
func TestRequestSigningACS3Conformance(t *testing.T) {
	cases, expectedByID := loadConformance(t, "request-signing-acs3")
	require.NotEmpty(t, cases, "fixture must have at least one case")

	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			var input struct {
				Method        string            `json:"method"`
				URI           string            `json:"uri"`
				QueryString   string            `json:"query_string"`
				Body          string            `json:"body"`
				Headers       map[string]string `json:"headers"`
				SourceToken   string            `json:"source_token"`
				ProjectSecret string            `json:"project_secret"`
			}
			require.NoError(t, json.Unmarshal(c.Raw, &input), "unmarshal case input")

			var expected struct {
				Authorization string            `json:"authorization"`
				Headers       map[string]string `json:"headers"`
			}
			require.NoError(t, json.Unmarshal(expectedByID[c.ID], &expected), "unmarshal expected")

			// 复制 headers，避免污染 fixture 解析结果
			headers := make(map[string]string, len(input.Headers))
			for k, v := range input.Headers {
				headers[k] = v
			}

			authorization := SignRequest(
				input.Method, input.URI, input.QueryString,
				headers, []byte(input.Body),
				input.SourceToken, input.ProjectSecret,
			)

			require.Equal(t, expected.Authorization, authorization,
				"authorization should match golden")

			// 校验 SignRequest 是否按预期回填了 headers map
			for k, v := range expected.Headers {
				require.Equal(t, v, headers[k], "header %q should match golden", k)
			}
		})
	}
}
