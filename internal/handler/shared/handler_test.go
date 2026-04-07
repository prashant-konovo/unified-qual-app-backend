package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/unittests"
)

// newHandler returns a Handler backed by testutil defaults.
// HealthService wraps nil DBs so HealthCheck reports "not_configured".
func newHandler() *Handler {
	return &Handler{unittests.TestServices()}
}

func callHealth(h *Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.Health(rec, req)
	return rec
}

// TestHealth_AllDBsHealthy verifies that when all DBs are nil (HealthCheck
// returns "not_configured" for each), the handler responds 200 / "healthy".
func TestHealth_AllDBsHealthy(t *testing.T) {
	h := newHandler()
	rec := callHealth(h)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var body map[string]any
	unittests.DecodeJSON(t, rec, &body)
	assert.Equal(t, "healthy", body["status"])

	checks, ok := body["checks"].(map[string]any)
	assert.True(t, ok, "checks should be a map")
	for k, v := range checks {
		assert.Equal(t, "not_configured", v, "expected not_configured for %s", k)
	}
}

// TestHealth_NoDB confirms that a nil-DB HealthService is treated
// as healthy because "not_configured" is an acceptable status.
func TestHealth_NoDB(t *testing.T) {
	deps := unittests.TestServices()
	deps.HealthService = service.NewHealthService(nil)
	h := &Handler{deps}

	rec := callHealth(h)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var body map[string]any
	unittests.DecodeJSON(t, rec, &body)
	assert.Equal(t, "healthy", body["status"])
}

// TestHealth_IncludesEnvironment verifies that the response contains the
// environment value from the injected config.
func TestHealth_IncludesEnvironment(t *testing.T) {
	deps := unittests.TestServices()
	deps.Cfg = &config.Config{Environment: "staging"}
	h := &Handler{deps}

	rec := callHealth(h)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var body map[string]any
	unittests.DecodeJSON(t, rec, &body)
	assert.Equal(t, "staging", body["environment"])
}

// TestHealth_IncludesVersion verifies the hardcoded version string in the
// health response.
func TestHealth_IncludesVersion(t *testing.T) {
	h := newHandler()
	rec := callHealth(h)

	unittests.AssertStatus(t, rec, http.StatusOK)

	var body map[string]any
	unittests.DecodeJSON(t, rec, &body)
	assert.Equal(t, "1.2.0", body["version"])
}

// Verify Handler satisfies the expected embedding.
var _ *service.Services = (&Handler{}).Services
