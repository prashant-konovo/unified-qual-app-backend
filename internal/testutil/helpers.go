// Package testutil provides shared test helpers and mock constructors for unit tests.
package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/database"
	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/stretchr/testify/assert"
)

// TestConfig returns a minimal config suitable for handler tests.
func TestConfig() *config.Config {
	return &config.Config{
		Environment: "test",
		Port:        "8080",
		Cognito: config.CognitoConfig{
			UserPoolID: "us-east-1_test",
			AllClientIDs: []string{"test-client-id"},
			SSOClientID:    "test-sso-client",
			SSOClientSecret: "test-sso-secret",
			SSORedirectURI: "http://localhost:3000/login/sso-callback",
			Domain:         "test-auth.example.com",
			Region:         "us-east-1",
		},
		AuthAPIURL: "http://mock-auth-api",
		AuthAPIKey: "test-api-key",
	}
}

// TestDeps creates a *service.Deps with test config and nil repos.
// Callers should set the specific repos they need for their test.
func TestDeps() *service.Deps {
	return &service.Deps{
		Cfg: TestConfig(),
		DB:  &database.DBPair{},
	}
}

// NewJSONRequest creates an *http.Request with a JSON-encoded body.
// Pass nil for bodyless requests (GET, DELETE).
func NewJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// WithUser injects authenticated user claims into the request context.
func WithUser(r *http.Request, claims *middleware.UserClaims) *http.Request {
	return middleware.SetUser(r, claims)
}

// AdminUser returns UserClaims for a test admin user.
func AdminUser() *middleware.UserClaims {
	return &middleware.UserClaims{
		Sub:      "test-sub-admin",
		Email:    "admin@test.com",
		Username: "testadmin",
		Groups:   []string{"admin"},
		Roles:    []string{"admin"},
	}
}

// ManagerUser returns UserClaims for a test manager user.
func ManagerUser() *middleware.UserClaims {
	return &middleware.UserClaims{
		Sub:      "test-sub-manager",
		Email:    "manager@test.com",
		Username: "testmanager",
		Groups:   []string{"manager"},
		Roles:    []string{"manager"},
	}
}

// DecodeJSON decodes a JSON response body into dst.
func DecodeJSON(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

// AssertStatus asserts the response status code.
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	assert.Equal(t, expected, rec.Code, "unexpected status code; body: %s", rec.Body.String())
}

// AssertJSONKey asserts a top-level key exists in the JSON response and returns the decoded map.
func AssertJSONKey(t *testing.T, rec *httptest.ResponseRecorder, key string) map[string]any {
	t.Helper()
	var m map[string]any
	DecodeJSON(t, rec, &m)
	assert.Contains(t, m, key)
	return m
}
