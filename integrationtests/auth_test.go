//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/InCrowd/unified-qual-api/integrationintegrationtests/testserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Login validation ────────────────────────────────────────────

func TestAuthLogin_MissingEmail_Returns400(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"password": "secret"})
	resp, err := http.Post(ts.URL()+"/v1/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthLogin_MissingPassword_Returns400(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"email": "user@example.com"})
	resp, err := http.Post(ts.URL()+"/v1/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthLogin_InvalidEmail_Returns400(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"email": "not-an-email", "password": "secret"})
	resp, err := http.Post(ts.URL()+"/v1/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthLogin_EmptyBody_Returns400(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Post(ts.URL()+"/v1/auth/login", "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ── SSO config ──────────────────────────────────────────────────

func TestAuthSSOConfig_ReturnsAuthorizeURL(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/v1/auth/sso/config")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data["authorizeUrl"], "test-auth.example.com")
	assert.Contains(t, data["authorizeUrl"], "test-sso-client")
	assert.Equal(t, "http://localhost:3000/login/sso-callback", data["redirectUri"])
}

func TestAuthSSOConfig_RedirectOverride(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/v1/auth/sso/config?redirectUri=http://custom/callback")
	require.NoError(t, err)
	defer resp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	data := body["data"].(map[string]any)
	assert.Equal(t, "http://custom/callback", data["redirectUri"])
}

// ── Protected route without token ───────────────────────────────

func TestProtectedRoute_NoToken_Returns401(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/v1/projects")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestProtectedRoute_InvalidToken_Returns401(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL()+"/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer invalid-garbage-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
