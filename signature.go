package sensorswave

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	SignatureAlgorithm = "ACS3-HMAC-SHA256"
)

// SignRequest signs the request and returns the Authorization header value.
// This function modifies the input headers map to add signing-related headers:
// - x-content-sha256: SHA256 hash of the body
// - x-auth-timestamp: request timestamp (if not already provided)
// - x-auth-nonce: unique nonce (if not already provided)
// - projecttoken: the project token
// - Authorization: the final signature header
func SignRequest(method, uri, queryString string, headers map[string]string, body []byte, sourceToken, projectSecret string) string {
	// Create internal map for signing with normalized lowercase keys
	signHeaders := make(map[string]string, len(headers)+4)

	// Normalize header keys to lowercase
	for key, value := range headers {
		lowerKey := strings.ToLower(key)
		signHeaders[lowerKey] = value
	}

	// Calculate SHA256 hash of the body
	hashedPayload := sha256Hex(body)
	signHeaders["x-content-sha256"] = hashedPayload

	// Add timestamp and nonce if not provided
	if _, ok := signHeaders["x-auth-timestamp"]; !ok {
		signHeaders["x-auth-timestamp"] = fmt.Sprintf("%d", time.Now().UnixMilli())
	}
	if _, ok := signHeaders["x-auth-nonce"]; !ok {
		signHeaders["x-auth-nonce"] = generateNonce()
	}

	// Build canonical request
	canonicalRequest := buildCanonicalRequest(method, uri, queryString, signHeaders, hashedPayload)

	// Build string to sign
	stringToSign := fmt.Sprintf("%s\n%s", SignatureAlgorithm, sha256Hex([]byte(canonicalRequest)))

	// Calculate signature
	signature := hmacSHA256Hex(projectSecret, stringToSign)

	// Build Authorization header
	signedHeadersStr := getSortedHeaderKeys(signHeaders)
	authorization := fmt.Sprintf("%s Credential=%s,SignedHeaders=%s,Signature=%s",
		SignatureAlgorithm, sourceToken, signedHeadersStr, signature)

	// Write back signing headers to the input map for caller to use
	headers["x-content-sha256"] = signHeaders["x-content-sha256"]
	headers["x-auth-timestamp"] = signHeaders["x-auth-timestamp"]
	headers["x-auth-nonce"] = signHeaders["x-auth-nonce"]
	headers["Authorization"] = authorization

	return authorization
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256Hex(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func generateNonce() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func buildCanonicalRequest(method, uri, queryString string, headers map[string]string, hashedPayload string) string {
	var sb strings.Builder
	sb.WriteString(method)
	sb.WriteByte('\n')
	sb.WriteString(uri)
	sb.WriteByte('\n')
	sb.WriteString(queryString)
	sb.WriteByte('\n')

	// CanonicalHeaders
	sortedKeys := getSortedHeaderKeys(headers)
	keys := strings.Split(sortedKeys, ";")
	for _, k := range keys {
		v := headers[k]
		sb.WriteString(strings.ToLower(k))
		sb.WriteByte(':')
		sb.WriteString(strings.TrimSpace(v))
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')

	// SignedHeaders
	sb.WriteString(sortedKeys)
	sb.WriteByte('\n')

	// HashedPayload
	sb.WriteString(hashedPayload) // use calculated payload hash

	return sb.String()
}

func getSortedHeaderKeys(headers map[string]string) string {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, strings.ToLower(k))
	}
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

// URLEncode encodes URL parameters
func URLEncode(s string) string {
	// Go's url.QueryEscape escapes spaces to "+", but we generally want "%20" for strict S3-like auth
	// However, stdlib's QueryEscape is usually fine for general query params.
	// Implementing strict RFC 3986 here if needed, but QueryEscape is safer default.
	return url.QueryEscape(s)
}
