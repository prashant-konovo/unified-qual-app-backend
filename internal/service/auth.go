package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ICAuthClient abstracts InCrowdAPI auth operations for testability.
type ICAuthClient interface {
	Configured() bool
	Login(ctx context.Context, email, password string, brandID int64) (*integration.ICLoginResponse, int, error)
	RefreshToken(ctx context.Context, userID int64, icAuthToken string) (string, error)
	AcceptTerms(ctx context.Context, userID int64) error
	Logout(ctx context.Context, userID int64, icAuthToken string) error
	ChangePassword(ctx context.Context, userID int64, icAuthToken, oldPassword, newPassword string) error
}

// AuthService encapsulates authentication business logic,
// including dual-path auth (InCrowdAPI primary, Cognito fallback),
// terms acceptance, user enrichment, and token management.
type AuthService struct {
	icAuth     ICAuthClient
	qsUserRepo qs.UserRepository
	cfg        *config.Config
}

// NewAuthService creates a new AuthService.
func NewAuthService(cfg *config.Config, icAuth ICAuthClient, qsUserRepo qs.UserRepository) *AuthService {
	return &AuthService{
		icAuth:     icAuth,
		qsUserRepo: qsUserRepo,
		cfg:        cfg,
	}
}

// ── Result types ────────────────────────────────────────────────────────────

// LoginResult contains the outcome of a login attempt.
type LoginResult struct {
	// Success indicates a successful login with tokens.
	Success bool
	// NeedsTerms indicates the user must accept terms before proceeding.
	NeedsTerms bool
	// TermsUserID is the user ID when terms acceptance is needed.
	TermsUserID int64
	// Response is the full JSON-serializable response body on success.
	Response map[string]any
	// ErrorStatus is the HTTP status code on failure (0 on success).
	ErrorStatus int
	// ErrorBody is the JSON-serializable error response body on failure.
	ErrorBody map[string]any
	// RawBody + RawStatus are for forwarding raw Cognito responses.
	RawBody   []byte
	RawStatus int
}

// RefreshResult contains the outcome of a token refresh attempt.
type RefreshResult struct {
	Success   bool
	Response  map[string]any
	RawBody   []byte
	RawStatus int
	Error     error
}

// SSOConfig contains the SSO configuration response.
type SSOConfig struct {
	AuthorizeURL string
	RedirectURI  string
}

// SSOTokens are the tokens returned from a Cognito OAuth2 code exchange.
type SSOTokens struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// ── Login ───────────────────────────────────────────────────────────────────

// Login authenticates a user via InCrowdAPI (primary) or direct Cognito (fallback).
// It handles terms acceptance and user enrichment from the QS database.
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) *LoginResult {
	// ── Primary path: InCrowdAPI ────────────────────────────────────
	if s.icAuth != nil && s.icAuth.Configured() {
		loginResp, statusCode, err := s.icAuth.Login(ctx, req.Email, req.Password, 1)
		if err == nil && loginResp != nil {
			return s.handleICLoginSuccess(ctx, loginResp, req)
		}

		if err != nil && statusCode > 0 {
			slog.Warn("InCrowdAPI login failed", "status", statusCode, "error", err)
			if statusCode == http.StatusUnauthorized || statusCode == http.StatusUnprocessableEntity {
				return &LoginResult{
					ErrorStatus: http.StatusUnauthorized,
					ErrorBody: map[string]any{
						"error":        "Invalid email or password",
						"errorMessage": "Invalid email or password",
					},
				}
			}
			slog.Warn("InCrowdAPI login error, falling back to direct Cognito", "status", statusCode, "error", err)
		}
	}

	// ── Fallback: direct Cognito auth ──────────────────────────────
	body, status, err := s.cognitoAdminAuth(ctx, req.Email, req.Password)
	if err != nil {
		return &LoginResult{
			ErrorStatus: http.StatusInternalServerError,
			ErrorBody: map[string]any{
				"error":        "An error occurred while logging in",
				"errorMessage": "An error occurred while logging in",
			},
		}
	}

	if status != http.StatusOK {
		return &LoginResult{RawBody: body, RawStatus: status}
	}

	return s.handleCognitoLoginSuccess(ctx, body, req)
}

func (s *AuthService) handleICLoginSuccess(ctx context.Context, loginResp *integration.ICLoginResponse, req dto.LoginRequest) *LoginResult {
	// Terms acceptance check
	if !loginResp.AcceptedTerms {
		if req.TermsAccepted != nil && *req.TermsAccepted {
			_ = s.icAuth.AcceptTerms(ctx, loginResp.ID)
			if s.qsUserRepo != nil {
				_ = s.qsUserRepo.AcceptTerms(ctx, loginResp.ID)
			}
		} else {
			return &LoginResult{
				NeedsTerms:  true,
				TermsUserID: loginResp.ID,
			}
		}
	}

	userInfo := map[string]any{
		"IdToken":   loginResp.CognitoToken,
		"id":        loginResp.ID,
		"firstName": loginResp.FirstName,
		"lastName":  loginResp.LastName,
		"email":     loginResp.Email,
		"roles":     loginResp.Roles,
	}

	// Enrich with QS DB data if available
	if s.qsUserRepo != nil {
		u, err := s.qsUserRepo.GetByEmail(ctx, req.Email)
		if err == nil && u != nil {
			userInfo["id"] = u.ID
			userInfo["firstName"] = utilities.NullStr(u.FirstName)
			userInfo["lastName"] = utilities.NullStr(u.LastName)
		}
	}

	return &LoginResult{
		Success: true,
		Response: map[string]any{
			"statusCode": 200,
			"body": map[string]any{
				"statusCode": 200,
				"body": map[string]any{
					"userInfo":     userInfo,
					"apiKey":       s.cfg.AuthAPIKey,
					"icUserId":     loginResp.ID,
					"icAuthToken":  loginResp.AccessToken,
					"IdToken":      loginResp.CognitoToken,
					"AccessToken":  loginResp.AccessToken,
					"RefreshToken": "",
				},
			},
		},
	}
}

func (s *AuthService) handleCognitoLoginSuccess(ctx context.Context, body []byte, req dto.LoginRequest) *LoginResult {
	var tokens map[string]any
	_ = json.Unmarshal(body, &tokens)

	userInfo := map[string]any{}
	for k, v := range tokens {
		userInfo[k] = v
	}

	if s.qsUserRepo != nil {
		u, err := s.qsUserRepo.GetByEmail(ctx, req.Email)
		if err == nil && u != nil {
			userInfo["id"] = u.ID
			userInfo["firstName"] = utilities.NullStr(u.FirstName)
			userInfo["lastName"] = utilities.NullStr(u.LastName)

			if req.TermsAccepted != nil && *req.TermsAccepted {
				_ = s.qsUserRepo.AcceptTerms(ctx, u.ID)
			} else if u.TermsAccepted != 1 {
				return &LoginResult{
					NeedsTerms:  true,
					TermsUserID: u.ID,
				}
			}
		}
	}

	return &LoginResult{
		Success: true,
		Response: map[string]any{
			"statusCode": 200,
			"body": map[string]any{
				"statusCode": 200,
				"body": map[string]any{
					"userInfo":     userInfo,
					"apiKey":       s.cfg.AuthAPIKey,
					"IdToken":      tokens["IdToken"],
					"AccessToken":  tokens["AccessToken"],
					"RefreshToken": tokens["RefreshToken"],
				},
			},
		},
	}
}

// ── Refresh ─────────────────────────────────────────────────────────────────

// Refresh refreshes authentication tokens via InCrowdAPI or direct Cognito.
func (s *AuthService) Refresh(ctx context.Context, req dto.RefreshRequest) *RefreshResult {
	// Primary: InCrowdAPI
	if s.icAuth != nil && s.icAuth.Configured() && req.ICUserID > 0 && req.ICAuthToken != "" {
		newIDToken, err := s.icAuth.RefreshToken(ctx, req.ICUserID, req.ICAuthToken)
		if err == nil && newIDToken != "" {
			return &RefreshResult{
				Success: true,
				Response: map[string]any{
					"IdToken":     newIDToken,
					"AccessToken": req.ICAuthToken,
				},
			}
		}
		slog.Warn("InCrowdAPI token refresh failed, falling back to direct Cognito", "error", err)
	}

	// Fallback: Cognito
	if req.RefreshToken == "" {
		return &RefreshResult{Error: fmt.Errorf("refreshToken or icUserId+icAuthToken is required")}
	}

	body, status, err := s.cognitoRefresh(ctx, req.RefreshToken)
	if err != nil {
		return &RefreshResult{Error: fmt.Errorf("token refresh error: %v", err)}
	}
	return &RefreshResult{RawBody: body, RawStatus: status}
}

// ── Terms ───────────────────────────────────────────────────────────────────

// AcceptTerms records terms acceptance in both InCrowdAPI and local QS DB.
func (s *AuthService) AcceptTerms(ctx context.Context, userID int64) {
	if s.icAuth != nil && s.icAuth.Configured() {
		if err := s.icAuth.AcceptTerms(ctx, userID); err != nil {
			slog.Warn("InCrowdAPI accept terms failed", "error", err)
		}
	}
	if s.qsUserRepo != nil {
		_ = s.qsUserRepo.AcceptTerms(ctx, userID)
	}
}

// ── Logout ──────────────────────────────────────────────────────────────────

// Logout revokes the user's InCrowdAPI session. Best-effort — errors are logged, not returned.
func (s *AuthService) Logout(ctx context.Context, icUserID int64, icAuthToken string) {
	if s.icAuth != nil && s.icAuth.Configured() && icUserID > 0 {
		if err := s.icAuth.Logout(ctx, icUserID, icAuthToken); err != nil {
			slog.Warn("InCrowdAPI logout failed", "error", err)
		}
	}
}

// ── Password ────────────────────────────────────────────────────────────────

// ChangePassword changes a user's password via InCrowdAPI and returns the updated user info.
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, icAuthToken, oldPwd, newPwd string) (map[string]any, error) {
	if s.icAuth == nil || !s.icAuth.Configured() {
		return nil, fmt.Errorf("password change service unavailable")
	}

	if err := s.icAuth.ChangePassword(ctx, userID, icAuthToken, oldPwd, newPwd); err != nil {
		return nil, fmt.Errorf("password change failed")
	}

	slog.Info("password changed", "userId", userID)

	if userID > 0 && s.qsUserRepo != nil {
		u, err := s.qsUserRepo.GetByID(ctx, userID)
		if err == nil && u != nil {
			return map[string]any{
				"id":         u.ID,
				"first_name": utilities.NullStr(u.FirstName),
				"last_name":  utilities.NullStr(u.LastName),
				"email":      utilities.NullStr(u.Email),
			}, nil
		}
	}

	return map[string]any{"changed": true}, nil
}

// ResolveUserID resolves a user's ID from their email using the QS DB.
func (s *AuthService) ResolveUserID(ctx context.Context, email string) int64 {
	if s.qsUserRepo == nil || email == "" {
		return 0
	}
	u, err := s.qsUserRepo.GetByEmail(ctx, email)
	if err != nil || u == nil {
		return 0
	}
	return u.ID
}

// ── SSO ─────────────────────────────────────────────────────────────────────

// GetSSOConfig returns the Cognito SSO authorize URL and redirect URI.
func (s *AuthService) GetSSOConfig(redirectURIOverride string) (*SSOConfig, error) {
	cfg := s.cfg.Cognito
	if cfg.Domain == "" || cfg.SSOClientID == "" {
		return nil, fmt.Errorf("SSO is not configured for this environment")
	}

	redirectURI := cfg.SSORedirectURI
	if redirectURIOverride != "" {
		redirectURI = redirectURIOverride
	}

	authorizeURL := fmt.Sprintf(
		"https://%s/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=email+openid",
		cfg.Domain, cfg.SSOClientID, url.QueryEscape(redirectURI),
	)

	return &SSOConfig{AuthorizeURL: authorizeURL, RedirectURI: redirectURI}, nil
}

// ExchangeSSOCode exchanges an authorization code for tokens via the Cognito token endpoint.
func (s *AuthService) ExchangeSSOCode(ctx context.Context, code, redirectURI string) (*SSOTokens, error) {
	cfg := s.cfg.Cognito
	if cfg.Domain == "" || cfg.SSOClientID == "" {
		return nil, fmt.Errorf("SSO is not configured")
	}
	return s.cognitoExchangeCode(ctx, code, redirectURI)
}

// ── Cognito helpers (private) ───────────────────────────────────────────────

func (s *AuthService) cognitoAdminAuth(ctx context.Context, username, password string) ([]byte, int, error) {
	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", s.cfg.Cognito.Region)

	payload := map[string]any{
		"AuthFlow": "USER_PASSWORD_AUTH",
		"ClientId": s.cfg.Cognito.AppClientID,
		"AuthParameters": map[string]string{
			"USERNAME": username,
			"PASSWORD": password,
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal cognito payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("create cognito request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService.InitiateAuth")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("call cognito: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read cognito response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var cognitoResp struct {
			AuthenticationResult struct {
				IdToken      string `json:"IdToken"`
				AccessToken  string `json:"AccessToken"`
				RefreshToken string `json:"RefreshToken"`
				ExpiresIn    int    `json:"ExpiresIn"`
				TokenType    string `json:"TokenType"`
			} `json:"AuthenticationResult"`
		}
		if err := json.Unmarshal(body, &cognitoResp); err == nil && cognitoResp.AuthenticationResult.IdToken != "" {
			normalized, _ := json.Marshal(map[string]any{
				"IdToken":      cognitoResp.AuthenticationResult.IdToken,
				"AccessToken":  cognitoResp.AuthenticationResult.AccessToken,
				"RefreshToken": cognitoResp.AuthenticationResult.RefreshToken,
				"ExpiresIn":    cognitoResp.AuthenticationResult.ExpiresIn,
				"TokenType":    cognitoResp.AuthenticationResult.TokenType,
			})
			return normalized, http.StatusOK, nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		var cognitoErr struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &cognitoErr); err == nil {
			status := http.StatusUnauthorized
			var msg string
			switch cognitoErr.Type {
			case "NotAuthorizedException", "UserNotFoundException":
				msg = "Invalid email or password"
			case "UserNotConfirmedException":
				msg = "Account not confirmed. Please check your email."
				status = http.StatusForbidden
			case "PasswordResetRequiredException":
				msg = "Password reset required"
				status = http.StatusForbidden
			default:
				msg = cognitoErr.Message
				status = http.StatusBadRequest
			}
			errorResp, _ := json.Marshal(map[string]any{"error": msg})
			return errorResp, status, nil
		}
	}

	return body, resp.StatusCode, nil
}

func (s *AuthService) cognitoRefresh(ctx context.Context, refreshToken string) ([]byte, int, error) {
	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", s.cfg.Cognito.Region)

	payload := map[string]any{
		"AuthFlow": "REFRESH_TOKEN_AUTH",
		"ClientId": s.cfg.Cognito.AppClientID,
		"AuthParameters": map[string]string{
			"REFRESH_TOKEN": refreshToken,
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal cognito payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("create cognito request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService.InitiateAuth")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("call cognito: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read cognito response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var cognitoResp struct {
			AuthenticationResult struct {
				IdToken     string `json:"IdToken"`
				AccessToken string `json:"AccessToken"`
				ExpiresIn   int    `json:"ExpiresIn"`
				TokenType   string `json:"TokenType"`
			} `json:"AuthenticationResult"`
		}
		if err := json.Unmarshal(body, &cognitoResp); err == nil && cognitoResp.AuthenticationResult.IdToken != "" {
			normalized, _ := json.Marshal(map[string]any{
				"IdToken":     cognitoResp.AuthenticationResult.IdToken,
				"AccessToken": cognitoResp.AuthenticationResult.AccessToken,
				"ExpiresIn":   cognitoResp.AuthenticationResult.ExpiresIn,
				"TokenType":   cognitoResp.AuthenticationResult.TokenType,
			})
			return normalized, http.StatusOK, nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		var cognitoErr struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &cognitoErr); err == nil {
			errorResp, _ := json.Marshal(map[string]any{"error": cognitoErr.Message})
			return errorResp, http.StatusUnauthorized, nil
		}
	}

	return body, resp.StatusCode, nil
}

func (s *AuthService) cognitoExchangeCode(ctx context.Context, code, redirectURI string) (*SSOTokens, error) {
	cfg := s.cfg.Cognito
	tokenURL := fmt.Sprintf("https://%s/oauth2/token", cfg.Domain)

	data := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {cfg.SSOClientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if cfg.SSOClientSecret != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(cfg.SSOClientID + ":" + cfg.SSOClientSecret))
		req.Header.Set("Authorization", "Basic "+creds)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call cognito token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cognito token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result SSOTokens
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &result, nil
}
