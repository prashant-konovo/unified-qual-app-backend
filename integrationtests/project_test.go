//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/integrationtests/testserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Authenticated project listing ───────────────────────────────

func TestProjects_AdminToken_ReturnsProjects(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	ts.IrisProjectRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{
			{ID: 1, Name: "Test Project"},
		}, 1, nil)

	req, _ := http.NewRequest("GET", ts.URL()+"/v1/projects?source=iris", nil)
	req.Header.Set("Authorization", "Bearer "+ts.AdminToken())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "data")

	ts.IrisProjectRepo.AssertExpectations(t)
}

func TestProjects_ManagerToken_Allowed(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	ts.IrisProjectRepo.On("List", mock.Anything, 1, 20, (*int)(nil), "").
		Return([]iris.ProjectListRow{}, 0, nil)

	req, _ := http.NewRequest("GET", ts.URL()+"/v1/projects?source=iris", nil)
	req.Header.Set("Authorization", "Bearer "+ts.ManagerToken())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	ts.IrisProjectRepo.AssertExpectations(t)
}

func TestProjects_NoToken_Unauthorized(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	resp, err := http.Get(ts.URL() + "/v1/projects")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestProjects_MalformedToken_Unauthorized(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL()+"/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.expired")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── CORS headers ────────────────────────────────────────────────

func TestCORS_OptionsRequest_ReturnsHeaders(t *testing.T) {
	ts := testserver.New()
	defer ts.Close()

	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "OPTIONS", ts.URL()+"/v1/projects", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
}
