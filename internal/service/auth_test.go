package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	mocks "github.com/InCrowd/unified-qual-api/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── Test helpers ────────────────────────────────────────────────────────────

func testConfig() *config.Config {
	return &config.Config{
		Environment: "test",
		Port:        "8080",
		Cognito: config.CognitoConfig{
			UserPoolID:      "us-east-1_test",
			AllClientIDs:    []string{"test-client-id"},
			SSOClientID:     "test-sso-client",
			SSOClientSecret: "test-sso-secret",
			SSORedirectURI:  "http://localhost:3000/login/sso-callback",
			Domain:          "test-auth.example.com",
			Region:          "us-east-1",
		},
		AuthAPIURL: "http://mock-auth-api",
		AuthAPIKey: "test-api-key",
	}
}

// mockICAuth implements ICAuthClient for testing.
type mockICAuth struct {
	configured      bool
	loginResp       *integration.ICLoginResponse
	loginStatus     int
	loginErr        error
	refreshToken    string
	refreshErr      error
	acceptTermsErr  error
	logoutErr       error
	changePwdErr    error

	loginCalled     bool
	refreshCalled   bool
	acceptCalled    bool
	logoutCalled    bool
	changePwdCalled bool

	lastLogoutUserID    int64
	lastAcceptUserID    int64
}

func (m *mockICAuth) Configured() bool { return m.configured }

func (m *mockICAuth) Login(_ context.Context, _, _ string, _ int64) (*integration.ICLoginResponse, int, error) {
	m.loginCalled = true
	return m.loginResp, m.loginStatus, m.loginErr
}

func (m *mockICAuth) RefreshToken(_ context.Context, _ int64, _ string) (string, error) {
	m.refreshCalled = true
	return m.refreshToken, m.refreshErr
}

func (m *mockICAuth) AcceptTerms(_ context.Context, userID int64) error {
	m.acceptCalled = true
	m.lastAcceptUserID = userID
	return m.acceptTermsErr
}

func (m *mockICAuth) Logout(_ context.Context, userID int64, _ string) error {
	m.logoutCalled = true
	m.lastLogoutUserID = userID
	return m.logoutErr
}

func (m *mockICAuth) ChangePassword(_ context.Context, _ int64, _, _, _ string) error {
	m.changePwdCalled = true
	return m.changePwdErr
}

func boolPtr(b bool) *bool { return &b }

func sqlStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// ── GetSSOConfig tests ──────────────────────────────────────────────────────

func TestGetSSOConfig_ReturnsAuthorizeURL(t *testing.T) {
	svc := NewAuthService(testConfig(), nil, nil)

	cfg, err := svc.GetSSOConfig("")
	assert.NoError(t, err)
	assert.Contains(t, cfg.AuthorizeURL, "test-auth.example.com")
	assert.Contains(t, cfg.AuthorizeURL, "client_id=test-sso-client")
	assert.Equal(t, "http://localhost:3000/login/sso-callback", cfg.RedirectURI)
}

func TestGetSSOConfig_RedirectOverride(t *testing.T) {
	svc := NewAuthService(testConfig(), nil, nil)

	cfg, err := svc.GetSSOConfig("http://custom:3000/callback")
	assert.NoError(t, err)
	assert.Equal(t, "http://custom:3000/callback", cfg.RedirectURI)
	assert.Contains(t, cfg.AuthorizeURL, "http%3A%2F%2Fcustom%3A3000%2Fcallback")
}

func TestGetSSOConfig_NotConfigured(t *testing.T) {
	cfg := testConfig()
	cfg.Cognito.Domain = ""
	svc := NewAuthService(cfg, nil, nil)

	_, err := svc.GetSSOConfig("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SSO is not configured")
}

// ── ResolveUserID tests ─────────────────────────────────────────────────────

func TestResolveUserID_Found(t *testing.T) {
	userRepo := mocks.NewMockQsUserRepository(t)
	userRepo.EXPECT().GetByEmail(mock.Anything, "john@example.com").Return(
		&qs.UserWithRoles{User: qs.User{ID: 42}}, nil,
	)

	svc := NewAuthService(testConfig(), nil, userRepo)
	id := svc.ResolveUserID(context.Background(), "john@example.com")
	assert.Equal(t, int64(42), id)
}

func TestResolveUserID_NotFound(t *testing.T) {
	userRepo := mocks.NewMockQsUserRepository(t)
	userRepo.EXPECT().GetByEmail(mock.Anything, "nobody@example.com").Return(
		nil, fmt.Errorf("not found"),
	)

	svc := NewAuthService(testConfig(), nil, userRepo)
	id := svc.ResolveUserID(context.Background(), "nobody@example.com")
	assert.Equal(t, int64(0), id)
}

func TestResolveUserID_EmptyEmail(t *testing.T) {
	svc := NewAuthService(testConfig(), nil, nil)
	id := svc.ResolveUserID(context.Background(), "")
	assert.Equal(t, int64(0), id)
}

// ── Login tests (IC path) ───────────────────────────────────────────────────

func TestLogin_ICPath_Success(t *testing.T) {
	icAuth := &mockICAuth{
		configured: true,
		loginResp: &integration.ICLoginResponse{
			ID:            100,
			Email:         "user@example.com",
			FirstName:     "John",
			LastName:      "Doe",
			CognitoToken:  "id-token-123",
			AccessToken:   "access-token-456",
			AcceptedTerms: true,
			Roles:         []string{"admin"},
		},
	}

	userRepo := mocks.NewMockQsUserRepository(t)
	userRepo.EXPECT().GetByEmail(mock.Anything, "user@example.com").Return(
		&qs.UserWithRoles{User: qs.User{
			ID:        100,
			FirstName: sqlStr("John"),
			LastName:  sqlStr("Doe"),
		}}, nil,
	)

	svc := NewAuthService(testConfig(), icAuth, userRepo)
	result := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "user@example.com",
		Password: "secret",
	})

	assert.True(t, result.Success)
	assert.False(t, result.NeedsTerms)
	body := result.Response["body"].(map[string]any)["body"].(map[string]any)
	assert.Equal(t, "test-api-key", body["apiKey"])
	assert.Equal(t, int64(100), body["icUserId"])
	assert.Equal(t, "id-token-123", body["IdToken"])
}

func TestLogin_ICPath_NeedsTerms(t *testing.T) {
	icAuth := &mockICAuth{
		configured: true,
		loginResp: &integration.ICLoginResponse{
			ID:            100,
			AcceptedTerms: false,
		},
	}

	svc := NewAuthService(testConfig(), icAuth, nil)
	result := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "user@example.com",
		Password: "secret",
	})

	assert.False(t, result.Success)
	assert.True(t, result.NeedsTerms)
	assert.Equal(t, int64(100), result.TermsUserID)
}

func TestLogin_ICPath_AcceptTermsInline(t *testing.T) {
	icAuth := &mockICAuth{
		configured: true,
		loginResp: &integration.ICLoginResponse{
			ID:            100,
			Email:         "user@example.com",
			AcceptedTerms: false,
			CognitoToken:  "tok",
			AccessToken:   "acc",
		},
	}

	userRepo := mocks.NewMockQsUserRepository(t)
	userRepo.EXPECT().AcceptTerms(mock.Anything, int64(100)).Return(nil)
	userRepo.EXPECT().GetByEmail(mock.Anything, "user@example.com").Return(
		&qs.UserWithRoles{User: qs.User{ID: 100, FirstName: sqlStr("J"), LastName: sqlStr("D")}}, nil,
	)

	svc := NewAuthService(testConfig(), icAuth, userRepo)
	result := svc.Login(context.Background(), dto.LoginRequest{
		Email:         "user@example.com",
		Password:      "secret",
		TermsAccepted: boolPtr(true),
	})

	assert.True(t, result.Success)
	assert.True(t, icAuth.acceptCalled)
	assert.Equal(t, int64(100), icAuth.lastAcceptUserID)
}

func TestLogin_ICPath_Unauthorized(t *testing.T) {
	icAuth := &mockICAuth{
		configured:  true,
		loginStatus: 401,
		loginErr:    fmt.Errorf("unauthorized"),
	}

	svc := NewAuthService(testConfig(), icAuth, nil)
	result := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "user@example.com",
		Password: "wrong",
	})

	assert.False(t, result.Success)
	assert.Equal(t, 401, result.ErrorStatus)
	assert.Equal(t, "Invalid email or password", result.ErrorBody["error"])
}

// ── Refresh tests ───────────────────────────────────────────────────────────

func TestRefresh_ICPath_Success(t *testing.T) {
	icAuth := &mockICAuth{
		configured:   true,
		refreshToken: "new-id-token",
	}

	svc := NewAuthService(testConfig(), icAuth, nil)
	result := svc.Refresh(context.Background(), dto.RefreshRequest{
		ICUserID:    100,
		ICAuthToken: "old-auth-token",
	})

	assert.True(t, result.Success)
	assert.Equal(t, "new-id-token", result.Response["IdToken"])
	assert.Equal(t, "old-auth-token", result.Response["AccessToken"])
}

func TestRefresh_NoTokens_Error(t *testing.T) {
	svc := NewAuthService(testConfig(), nil, nil)
	result := svc.Refresh(context.Background(), dto.RefreshRequest{})

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "refreshToken or icUserId+icAuthToken is required")
}

// ── AcceptTerms tests ───────────────────────────────────────────────────────

func TestAcceptTerms_BothPaths(t *testing.T) {
	icAuth := &mockICAuth{configured: true}
	userRepo := mocks.NewMockQsUserRepository(t)
	userRepo.EXPECT().AcceptTerms(mock.Anything, int64(42)).Return(nil)

	svc := NewAuthService(testConfig(), icAuth, userRepo)
	svc.AcceptTerms(context.Background(), 42)

	assert.True(t, icAuth.acceptCalled)
	assert.Equal(t, int64(42), icAuth.lastAcceptUserID)
}

// ── Logout tests ────────────────────────────────────────────────────────────

func TestLogout_CallsICAuth(t *testing.T) {
	icAuth := &mockICAuth{configured: true}

	svc := NewAuthService(testConfig(), icAuth, nil)
	svc.Logout(context.Background(), 42, "auth-token")

	assert.True(t, icAuth.logoutCalled)
	assert.Equal(t, int64(42), icAuth.lastLogoutUserID)
}

func TestLogout_SkipsWhenNotConfigured(t *testing.T) {
	icAuth := &mockICAuth{configured: false}

	svc := NewAuthService(testConfig(), icAuth, nil)
	svc.Logout(context.Background(), 42, "auth-token")

	assert.False(t, icAuth.logoutCalled)
}

// ── ChangePassword tests ────────────────────────────────────────────────────

func TestChangePassword_Success_WithUserInfo(t *testing.T) {
	icAuth := &mockICAuth{configured: true}
	userRepo := mocks.NewMockQsUserRepository(t)
	userRepo.EXPECT().GetByID(mock.Anything, int64(42)).Return(
		&qs.UserWithRoles{User: qs.User{
			ID:        42,
			FirstName: sqlStr("John"),
			LastName:  sqlStr("Doe"),
			Email:     sqlStr("john@example.com"),
		}}, nil,
	)

	svc := NewAuthService(testConfig(), icAuth, userRepo)
	resp, err := svc.ChangePassword(context.Background(), 42, "token", "old", "new")

	assert.NoError(t, err)
	assert.Equal(t, int64(42), resp["id"])
	assert.Equal(t, "John", resp["first_name"])
}

func TestChangePassword_NotConfigured(t *testing.T) {
	svc := NewAuthService(testConfig(), nil, nil)
	_, err := svc.ChangePassword(context.Background(), 42, "token", "old", "new")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password change service unavailable")
}

func TestChangePassword_ICAuthFails(t *testing.T) {
	icAuth := &mockICAuth{
		configured:   true,
		changePwdErr: fmt.Errorf("ic error"),
	}

	svc := NewAuthService(testConfig(), icAuth, nil)
	_, err := svc.ChangePassword(context.Background(), 42, "token", "old", "new")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password change failed")
}
