package middleware_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
)

func TestSetUser_GetUser_RoundTrip(t *testing.T) {
	claims := &middleware.UserClaims{
		Sub:      "abc-123",
		Email:    "alice@example.com",
		Username: "alice",
		Groups:   []string{"cognito-admins", "cognito-managers"},
		Roles:    []string{"admin", "manager"},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req = middleware.SetUser(req, claims)

	got := middleware.GetUser(req)

	assert.NotNil(t, got)
	assert.Equal(t, "abc-123", got.Sub)
	assert.Equal(t, "alice@example.com", got.Email)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, []string{"cognito-admins", "cognito-managers"}, got.Groups)
	assert.Equal(t, []string{"admin", "manager"}, got.Roles)
}

func TestGetUser_NoUserSet(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	got := middleware.GetUser(req)

	assert.Nil(t, got)
}

func TestSetUser_OverwritesPrevious(t *testing.T) {
	first := &middleware.UserClaims{
		Sub:      "first-user",
		Email:    "first@example.com",
		Username: "first",
		Roles:    []string{"observer"},
	}
	second := &middleware.UserClaims{
		Sub:      "second-user",
		Email:    "second@example.com",
		Username: "second",
		Roles:    []string{"admin"},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req = middleware.SetUser(req, first)
	req = middleware.SetUser(req, second)

	got := middleware.GetUser(req)

	assert.NotNil(t, got)
	assert.Equal(t, "second-user", got.Sub)
	assert.Equal(t, "second@example.com", got.Email)
	assert.Equal(t, "second", got.Username)
	assert.Equal(t, []string{"admin"}, got.Roles)
}
