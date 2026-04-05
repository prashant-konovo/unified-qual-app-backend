//go:build integration

package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/InCrowd/unified-qual-api/integrationtests/testserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth_FullStack_ReturnsHealthy(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])
	assert.Equal(t, "test", body["environment"])
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "checks")
}

func TestHealth_FullStack_ChecksContainDBStatus(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	checks, ok := body["checks"].(map[string]any)
	require.True(t, ok, "checks should be a map")
	assert.Contains(t, checks, "incrowdDB")
	assert.Contains(t, checks, "qstoolDB")
}

func TestHealth_FullStack_ContentTypeJSON(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}
