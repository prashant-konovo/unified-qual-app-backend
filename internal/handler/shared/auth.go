package shared

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

	"github.com/InCrowd/unified-qual-api/internal/handler/support"
	"github.com/InCrowd/unified-qual-api/internal/dto"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// AuthHandler handles authentication and authorization endpoints.
type AuthHandler struct{ *support.Deps }

// ──────────────────────────────────────────────
// Auth — proxies to central auth service (API Gateway → Lambda → Cognito)
// ──────────────────────────────────────────────

func (h *AuthHandler) AuthLogin(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// ── Primary path: proxy to InCrowdAPI ───────────────────────────
	if h.Services.ICAuth != nil && h.Services.ICAuth.Configured() {
		loginResp, statusCode, err := h.Services.ICAuth.Login(r.Context(), req.Email, req.Password, 1)
		if err == nil && loginResp != nil {
			// Terms acceptance check
			if !loginResp.AcceptedTerms {
				if req.TermsAccepted != nil && *req.TermsAccepted {
					_ = h.Services.ICAuth.AcceptTerms(r.Context(), loginResp.ID)
					if h.QsUserRepo != nil {
						_ = h.QsUserRepo.AcceptTerms(r.Context(), loginResp.ID)
					}
				} else {
					support.WriteJSON(w, 203, map[string]any{
						"termsAcceptedRes": map[string]any{
							"termsAccepted": false,
							"userId":        loginResp.ID,
						},
					})
					return
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
			if h.QsUserRepo != nil {
				u, err := h.QsUserRepo.GetByEmail(r.Context(), req.Email)
				if err == nil && u != nil {
					userInfo["id"] = u.ID
					userInfo["firstName"] = dto.NullStr(u.FirstName)
					userInfo["lastName"] = dto.NullStr(u.LastName)
				}
			}

			support.WriteJSON(w, http.StatusOK, map[string]any{
				"statusCode": 200,
				"body": map[string]any{
					"statusCode": 200,
					"body": map[string]any{
						"userInfo":    userInfo,
						"apiKey":      h.Cfg.AuthAPIKey,
						"icUserId":    loginResp.ID,
						"icAuthToken": loginResp.AccessToken,
						// Tokens at body level for frontend convenience
						"IdToken":      loginResp.CognitoToken,
						"AccessToken":  loginResp.AccessToken,
						"RefreshToken": "",
					},
				},
			})
			return
		}

		// InCrowdAPI returned an error
		if err != nil && statusCode > 0 {
			slog.Warn("InCrowdAPI login failed", "status", statusCode, "error", err)
			if statusCode == http.StatusUnauthorized || statusCode == http.StatusUnprocessableEntity {
				support.WriteJSON(w, http.StatusUnauthorized, map[string]any{
					"error":        "Invalid email or password",
					"errorMessage": "Invalid email or password",
				})
				return
			}
			slog.Warn("InCrowdAPI login error, falling back to direct Cognito", "status", statusCode, "error", err)
		}
	}

	// ── Fallback: direct Cognito auth ───────────────────────────────
	body, status, err := h.cognitoAdminAuth(r.Context(), req.Email, req.Password)
	if err != nil {
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "An error occurred while logging in",
			"errorMessage": "An error occurred while logging in",
		})
		return
	}

	if status != http.StatusOK {
		// Cognito returned an error — forward it
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
		return
	}

	var tokens map[string]any
	_ = json.Unmarshal(body, &tokens)

	userInfo := map[string]any{}
	for k, v := range tokens {
		userInfo[k] = v
	}

	if h.QsUserRepo != nil {
		u, err := h.QsUserRepo.GetByEmail(r.Context(), req.Email)
		if err == nil && u != nil {
			userInfo["id"] = u.ID
			userInfo["firstName"] = dto.NullStr(u.FirstName)
			userInfo["lastName"] = dto.NullStr(u.LastName)

			if req.TermsAccepted != nil && *req.TermsAccepted {
				_ = h.QsUserRepo.AcceptTerms(r.Context(), u.ID)
			} else if u.TermsAccepted != 1 {
				support.WriteJSON(w, 203, map[string]any{
					"termsAcceptedRes": map[string]any{
						"termsAccepted": false,
						"userId":        u.ID,
					},
				})
				return
			}
		}
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"statusCode": 200,
		"body": map[string]any{
			"statusCode": 200,
			"body": map[string]any{
				"userInfo": userInfo,
				"apiKey":   h.Cfg.AuthAPIKey,
				// Tokens at body level for frontend convenience
				"IdToken":      tokens["IdToken"],
				"AccessToken":  tokens["AccessToken"],
				"RefreshToken": tokens["RefreshToken"],
			},
		},
	})
}

func (h *AuthHandler) AuthMagicLink(w http.ResponseWriter, r *http.Request) {
	// Magic link auth is not yet supported by the central service — return stub.
	support.WriteJSON(w, http.StatusNotImplemented, map[string]any{
		"error": "magic link authentication not yet implemented",
	})
}

func (h *AuthHandler) AuthRefresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// ── Primary path: proxy to InCrowdAPI ───────────────────────────
	if h.Services.ICAuth != nil && h.Services.ICAuth.Configured() && req.ICUserID > 0 && req.ICAuthToken != "" {
		newIDToken, err := h.Services.ICAuth.RefreshToken(r.Context(), req.ICUserID, req.ICAuthToken)
		if err == nil && newIDToken != "" {
			support.WriteJSON(w, http.StatusOK, map[string]any{
				"IdToken":     newIDToken,
				"AccessToken": req.ICAuthToken,
			})
			return
		}
		slog.Warn("InCrowdAPI token refresh failed, falling back to direct Cognito", "error", err)
	}

	// ── Fallback: direct Cognito refresh ────────────────────────────
	if req.RefreshToken == "" {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "refreshToken or icUserId+icAuthToken is required"})
		return
	}

	body, status, err := h.cognitoRefresh(r.Context(), req.RefreshToken)
	if err != nil {
		support.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("token refresh error: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *AuthHandler) AuthPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"currentPassword"`
		NewPassword string `json:"newPassword"`
		Password    string `json:"password"`
		Token       string `json:"token"`
		UserID      int64  `json:"userId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	user := middleware.GetUser(r)

	newPwd := req.NewPassword
	if newPwd == "" {
		newPwd = req.Password
	}
	if newPwd == "" {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "newPassword is required"})
		return
	}

	// Derive user ID: prefer JWT-authenticated identity, fall back to request body
	targetUserID := req.UserID
	if targetUserID == 0 && h.QsUserRepo != nil && user != nil && user.Email != "" {
		u, err := h.QsUserRepo.GetByEmail(r.Context(), user.Email)
		if err == nil && u != nil {
			targetUserID = u.ID
		}
	}

	// Proxy to InCrowdAPI
	if h.Services.ICAuth == nil || !h.Services.ICAuth.Configured() {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "password change service unavailable"})
		return
	}

	// Extract IC-Auth token from request header for authentication
	icAuthToken := ""
	if icAuth := r.Header.Get("IC-Auth"); icAuth != "" {
		if parts := strings.SplitN(icAuth, ":", 2); len(parts) == 2 {
			icAuthToken = parts[1]
		}
	}

	err := h.Services.ICAuth.ChangePassword(r.Context(), targetUserID, icAuthToken, req.OldPassword, newPwd)
	if err != nil {
		slog.Warn("InCrowdAPI password change failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "password change failed"})
		return
	}

	slog.Info("password changed", "userId", targetUserID, "sub", user.Sub)

	if targetUserID > 0 && h.QsUserRepo != nil {
		u, err := h.QsUserRepo.GetByID(r.Context(), targetUserID)
		if err == nil && u != nil {
			support.WriteJSON(w, http.StatusOK, map[string]any{
				"id":         u.ID,
				"first_name": dto.NullStr(u.FirstName),
				"last_name":  dto.NullStr(u.LastName),
				"email":      dto.NullStr(u.Email),
			})
			return
		}
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"changed": true})
}

// AuthMe returns the authenticated user's claims from the JWT.
func (h *AuthHandler) AuthMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		support.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{
		"sub":      user.Sub,
		"email":    user.Email,
		"username": user.Username,
		"groups":   user.Groups,
		"roles":    user.Roles,
	})
}

// AuthLogout revokes the user's session via InCrowdAPI and clears server-side state.
func (h *AuthHandler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ICUserID    int64  `json:"icUserId"`
		ICAuthToken string `json:"icAuthToken"`
	}
	// Logout is best-effort — decode errors are not fatal.
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Proxy logout to InCrowdAPI to revoke IC session
	if h.Services.ICAuth != nil && h.Services.ICAuth.Configured() && req.ICUserID > 0 {
		if err := h.Services.ICAuth.Logout(r.Context(), req.ICUserID, req.ICAuthToken); err != nil {
			slog.Warn("InCrowdAPI logout failed", "error", err)
		}
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"message": "logged out"})
}

// AuthAcceptTerms proxies terms acceptance to InCrowdAPI and updates local QS DB.
func (h *AuthHandler) AuthAcceptTerms(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int64 `json:"userId" validate:"required,gt=0"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Proxy to InCrowdAPI
	if h.Services.ICAuth != nil && h.Services.ICAuth.Configured() {
		if err := h.Services.ICAuth.AcceptTerms(r.Context(), req.UserID); err != nil {
			slog.Warn("InCrowdAPI accept terms failed", "error", err)
		}
	}

	// Also update local QS DB
	if h.QsUserRepo != nil {
		_ = h.QsUserRepo.AcceptTerms(r.Context(), req.UserID)
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"message": "terms accepted"})
}

// ──────────────────────────────────────────────
// SSO — OAuth2 Authorization Code flow via Cognito Hosted UI
// ──────────────────────────────────────────────

// AuthSSOConfig returns the Cognito authorize URL so the frontend can redirect.
func (h *AuthHandler) AuthSSOConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.Cfg.Cognito
	if cfg.Domain == "" || cfg.SSOClientID == "" {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "SSO is not configured for this environment",
		})
		return
	}

	// Allow frontend to override redirect URI (for different environments)
	redirectURI := cfg.SSORedirectURI
	if override := r.URL.Query().Get("redirectUri"); override != "" {
		redirectURI = override
	}

	authorizeURL := fmt.Sprintf(
		"https://%s/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=email+openid",
		cfg.Domain,
		cfg.SSOClientID,
		url.QueryEscape(redirectURI),
	)

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"authorizeUrl": authorizeURL,
			"redirectUri":  redirectURI,
		},
	})
}

// AuthSSOCallback exchanges an authorization code for tokens via the Cognito token endpoint.
func (h *AuthHandler) AuthSSOCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"        validate:"required"`
		RedirectURI string `json:"redirectUri" validate:"required"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	cfg := h.Cfg.Cognito
	if cfg.Domain == "" || cfg.SSOClientID == "" {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "SSO is not configured"})
		return
	}

	tokens, err := h.cognitoExchangeCode(r.Context(), req.Code, req.RedirectURI)
	if err != nil {
		slog.Warn("SSO code exchange failed", "error", err)
		support.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "SSO authentication failed"})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{
		"statusCode": 200,
		"body": map[string]any{
			"statusCode": 200,
			"body": map[string]any{
				"userInfo": map[string]any{
					"IdToken": tokens.IDToken,
				},
				"IdToken":      tokens.IDToken,
				"AccessToken":  tokens.AccessToken,
				"RefreshToken": tokens.RefreshToken,
			},
		},
	})
}

// cognitoExchangeCode exchanges an authorization code for tokens via the Cognito OAuth2 token endpoint.
func (h *AuthHandler) cognitoExchangeCode(ctx context.Context, code, redirectURI string) (*ssoTokens, error) {
	cfg := h.Cfg.Cognito
	tokenURL := fmt.Sprintf("https://%s/oauth2/token", cfg.Domain)

	data := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {cfg.SSOClientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	// If client secret is configured, add it as Basic auth
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

	var result ssoTokens
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &result, nil
}

type ssoTokens struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// cognitoAdminAuth calls the Cognito InitiateAuth API directly via HTTPS.
// This is the fallback when the central auth service Lambda is unavailable.
// Uses USER_PASSWORD_AUTH flow which doesn't require SigV4 signing.
func (h *AuthHandler) cognitoAdminAuth(ctx context.Context, username, password string) ([]byte, int, error) {
	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", h.Cfg.Cognito.Region)

	payload := map[string]any{
		"AuthFlow": "USER_PASSWORD_AUTH",
		"ClientId": h.Cfg.Cognito.AppClientID,
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

	// Cognito returns AuthenticationResult on success — normalize to simpler shape
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

	// On error, translate Cognito error to user-friendly response
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

// cognitoRefresh calls the Cognito InitiateAuth API with REFRESH_TOKEN flow.
func (h *AuthHandler) cognitoRefresh(ctx context.Context, refreshToken string) ([]byte, int, error) {
	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", h.Cfg.Cognito.Region)

	payload := map[string]any{
		"AuthFlow": "REFRESH_TOKEN_AUTH",
		"ClientId": h.Cfg.Cognito.AppClientID,
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
