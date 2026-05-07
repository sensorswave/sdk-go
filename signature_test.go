package sensorswave

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignatureDifferentSecretsFail(t *testing.T) {
	sourceToken := "project-abc"
	clientSecret := "client-secret"
	serverSecret := "server-secret-different"
	method := "GET"
	uri := "/api/test"
	queryString := ""
	body := []byte{}

	clientHeaders := make(map[string]string)
	clientHeaders["x-auth-timestamp"] = "1736668800000"
	clientHeaders["x-auth-nonce"] = "nonce-123"

	clientAuth := SignRequest(method, uri, queryString, clientHeaders, body, sourceToken, clientSecret)

	serverHeaders := make(map[string]string)
	serverHeaders["x-auth-timestamp"] = clientHeaders["x-auth-timestamp"]
	serverHeaders["x-auth-nonce"] = clientHeaders["x-auth-nonce"]
	serverHeaders["x-content-sha256"] = clientHeaders["x-content-sha256"]

	serverAuth := SignRequest(method, uri, queryString, serverHeaders, body, sourceToken, serverSecret)

	require.NotEqual(t, clientAuth, serverAuth, "Different secrets should produce different signatures")
}

func TestSignatureTamperedBodyFails(t *testing.T) {
	sourceToken := "project-abc"
	projectSecret := "secret-xyz"
	method := "POST"
	uri := "/api/data"
	queryString := ""
	originalBody := []byte(`{"original": true}`)
	tamperedBody := []byte(`{"original": false}`)

	clientHeaders := make(map[string]string)
	clientHeaders["x-auth-timestamp"] = "1736668800000"
	clientHeaders["x-auth-nonce"] = "nonce-456"

	clientAuth := SignRequest(method, uri, queryString, clientHeaders, originalBody, sourceToken, projectSecret)

	// Server receives the same headers but with tampered body
	serverHeaders := make(map[string]string)
	serverHeaders["x-auth-timestamp"] = clientHeaders["x-auth-timestamp"]
	serverHeaders["x-auth-nonce"] = clientHeaders["x-auth-nonce"]
	// Note: x-content-sha256 will be recalculated with tampered body

	// Recalculate with tampered body - the body hash will be different
	serverAuth := SignRequest(method, uri, queryString, serverHeaders, tamperedBody, sourceToken, projectSecret)

	require.NotEqual(t, clientAuth, serverAuth, "Tampered body should produce different signature")
}

// TestSignatureWithPrecomputedHash tests that providing x-content-sha256 header uses that value
func TestSignatureWithPrecomputedHash(t *testing.T) {
	sourceToken := "project-abc"
	projectSecret := "secret-xyz"
	method := "POST"
	uri := "/api/data"
	queryString := ""
	body := []byte(`{"test": "data"}`)

	// Precompute body hash
	hash := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(hash[:])

	// Client provides precomputed hash
	clientHeaders := make(map[string]string)
	clientHeaders["x-auth-timestamp"] = "1736668800000"
	clientHeaders["x-auth-nonce"] = "nonce-789"
	clientHeaders["x-content-sha256"] = bodyHash

	clientAuth := SignRequest(method, uri, queryString, clientHeaders, body, sourceToken, projectSecret)

	// Server also provides the same precomputed hash
	serverHeaders := make(map[string]string)
	serverHeaders["x-auth-timestamp"] = clientHeaders["x-auth-timestamp"]
	serverHeaders["x-auth-nonce"] = clientHeaders["x-auth-nonce"]
	serverHeaders["x-content-sha256"] = clientHeaders["x-content-sha256"]

	serverAuth := SignRequest(method, uri, queryString, serverHeaders, body, sourceToken, projectSecret)

	require.Equal(t, clientAuth, serverAuth, "Precomputed hash should produce matching signatures")
}
