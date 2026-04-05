package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
)

// okHandler is a simple handler that writes 200 OK when reached.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

func TestRequireRoles_NoUser(t *testing.T) {
	handler := middleware.RequireRoles("admin")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "authentication required")
}

func TestRequireRoles_HasMatchingRole(t *testing.T) {
	handler := middleware.RequireRoles("admin")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = middleware.SetUser(req, &middleware.UserClaims{
		Sub:   "user-1",
		Roles: []string{"admin"},
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRequireRoles_NoMatchingRole(t *testing.T) {
	handler := middleware.RequireRoles("admin")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = middleware.SetUser(req, &middleware.UserClaims{
		Sub:   "user-2",
		Roles: []string{"moderator"},
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "insufficient permissions")
}

func TestRequireRoles_AnyOfMatch(t *testing.T) {
	handler := middleware.RequireRoles("admin", "manager", "moderator")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = middleware.SetUser(req, &middleware.UserClaims{
		Sub:   "user-3",
		Roles: []string{"moderator"},
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRequireRoles_EmptyRoles(t *testing.T) {
	handler := middleware.RequireRoles()(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = middleware.SetUser(req, &middleware.UserClaims{
		Sub:   "user-4",
		Roles: []string{"admin", "manager"},
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "insufficient permissions")
}
