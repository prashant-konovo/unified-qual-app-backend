package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	cfg              *config.Config
	db               *config.DBPair
	irisProjectRepo  *iris.ProjectRepo
	qsProjectRepo   *qs.ProjectRepo
	irisUserRepo    *iris.UserRepo
	qsUserRepo      *qs.UserRepo
}

func New(cfg *config.Config, db *config.DBPair, irisRepo *iris.ProjectRepo, qsRepo *qs.ProjectRepo, irisUserRepo *iris.UserRepo, qsUserRepo *qs.UserRepo) *Handler {
	return &Handler{cfg: cfg, db: db, irisProjectRepo: irisRepo, qsProjectRepo: qsRepo, irisUserRepo: irisUserRepo, qsUserRepo: qsUserRepo}
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Brand", "unified")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func success(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    data,
	})
}

func created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data":    data,
	})
}

func successList(w http.ResponseWriter, data any, total int) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    data,
		"meta": map[string]any{
			"page":       1,
			"pageSize":   20,
			"totalCount": total,
			"totalPages": 1,
		},
	})
}

func id() string { return uuid.New().String() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func parsePagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// ──────────────────────────────────────────────
// Health
// ──────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbChecks := h.db.HealthCheck(ctx)

	status := "healthy"
	httpCode := http.StatusOK
	for _, v := range dbChecks {
		if v != "ok" && v != "not_configured" {
			status = "degraded"
			httpCode = http.StatusServiceUnavailable
			break
		}
	}

	writeJSON(w, httpCode, map[string]any{
		"status":      status,
		"version":     "1.2.0",
		"environment": h.cfg.Environment,
		"checks":      dbChecks,
	})
}

// ──────────────────────────────────────────────
// Auth — proxies to central auth service (API Gateway → Lambda → Cognito)
// ──────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type centralAuthLoginRequest struct {
	UserName   string `json:"UserName"`
	Password   string `json:"Password"`
	UserPoolId string `json:"UserPoolId"`
	ClientId   string `json:"ClientId"`
}

type centralAuthRefreshRequest struct {
	RefreshToken string `json:"RefreshToken"`
	UserPoolId   string `json:"UserPoolId"`
	ClientId     string `json:"ClientId"`
}

func (h *Handler) AuthLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "email and password are required"})
		return
	}

	// Try central auth service first
	if h.cfg.AuthAPIURL != "" {
		payload := centralAuthLoginRequest{
			UserName:   req.Email,
			Password:   req.Password,
			UserPoolId: h.cfg.Cognito.UserPoolID,
			ClientId:   h.cfg.Cognito.AppClientID,
		}
		body, status, err := h.callAuthService(r.Context(), "/authentication/qs/login", payload)
		if err == nil && status >= 200 && status < 500 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}
		slog.Warn("central auth service failed, falling back to direct Cognito", "status", status, "error", err)
	}

	// Fallback: call Cognito ADMIN_USER_PASSWORD_AUTH directly via HTTP
	body, status, err := h.cognitoAdminAuth(r.Context(), req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("cognito auth error: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) AuthMagicLink(w http.ResponseWriter, r *http.Request) {
	// Magic link auth is not yet supported by the central service — return stub.
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error": "magic link authentication not yet implemented",
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) AuthRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "refreshToken is required"})
		return
	}

	// Try central auth service first
	if h.cfg.AuthAPIURL != "" {
		payload := centralAuthRefreshRequest{
			RefreshToken: req.RefreshToken,
			UserPoolId:   h.cfg.Cognito.UserPoolID,
			ClientId:     h.cfg.Cognito.AppClientID,
		}
		body, status, err := h.callAuthService(r.Context(), "/authentication/qs/refresh", payload)
		if err == nil && status >= 200 && status < 400 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}
		slog.Warn("central auth refresh failed, falling back to direct Cognito", "status", status, "error", err)
	}

	// Fallback: call Cognito InitiateAuth with REFRESH_TOKEN flow directly
	body, status, err := h.cognitoRefresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("cognito refresh error: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (h *Handler) AuthPassword(w http.ResponseWriter, r *http.Request) {
	// Password change is not yet wired through central service — return stub.
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error": "password change not yet implemented",
	})
}

// AuthMe returns the authenticated user's claims from the JWT.
func (h *Handler) AuthMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
		return
	}
	success(w, map[string]any{
		"sub":      user.Sub,
		"email":    user.Email,
		"username": user.Username,
		"groups":   user.Groups,
		"roles":    user.Roles,
	})
}

// ──────────────────────────────────────────────
// SSO — Cognito Hosted UI OAuth2 authorization code flow
// ──────────────────────────────────────────────

// AuthSSOConfig returns the SSO authorize URL so the frontend can redirect.
func (h *Handler) AuthSSOConfig(w http.ResponseWriter, r *http.Request) {
	c := h.cfg.Cognito
	if c.SSOClientID == "" || c.Domain == "" || c.SSORedirectURI == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "SSO not configured"})
		return
	}

	authorizeURL := fmt.Sprintf(
		"https://%s/oauth2/authorize?response_type=code&client_id=%s&scope=email+openid&redirect_uri=%s",
		c.Domain, c.SSOClientID, c.SSORedirectURI,
	)

	success(w, map[string]any{
		"authorizeUrl": authorizeURL,
		"clientId":     c.SSOClientID,
		"redirectUri":  c.SSORedirectURI,
	})
}

type ssoCallbackRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirectUri"`
}

// AuthSSOCallback exchanges an OAuth2 authorization code for Cognito tokens.
func (h *Handler) AuthSSOCallback(w http.ResponseWriter, r *http.Request) {
	var req ssoCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "code is required"})
		return
	}

	c := h.cfg.Cognito
	if c.SSOClientID == "" || c.Domain == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "SSO not configured"})
		return
	}

	redirectURI := req.RedirectURI
	if redirectURI == "" {
		redirectURI = c.SSORedirectURI
	}

	body, status, err := h.cognitoTokenExchange(r.Context(), req.Code, redirectURI)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("token exchange error: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// cognitoTokenExchange calls the Cognito /oauth2/token endpoint to exchange
// an authorization code for id_token, access_token, and refresh_token.
func (h *Handler) cognitoTokenExchange(ctx context.Context, code, redirectURI string) ([]byte, int, error) {
	c := h.cfg.Cognito
	tokenURL := fmt.Sprintf("https://%s/oauth2/token", c.Domain)

	form := fmt.Sprintf(
		"grant_type=authorization_code&client_id=%s&client_secret=%s&code=%s&redirect_uri=%s",
		c.SSOClientID, c.SSOClientSecret, code, redirectURI,
	)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(form))
	if err != nil {
		return nil, 0, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("call cognito token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read token response: %w", err)
	}

	// Cognito returns {id_token, access_token, refresh_token, expires_in, token_type}
	// Normalize to our standard shape
	if resp.StatusCode == http.StatusOK {
		var tokenResp struct {
			IDToken      string `json:"id_token"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			TokenType    string `json:"token_type"`
		}
		if err := json.Unmarshal(body, &tokenResp); err == nil && tokenResp.IDToken != "" {
			normalized, _ := json.Marshal(map[string]any{
				"IdToken":      tokenResp.IDToken,
				"AccessToken":  tokenResp.AccessToken,
				"RefreshToken": tokenResp.RefreshToken,
				"ExpiresIn":    tokenResp.ExpiresIn,
				"TokenType":    tokenResp.TokenType,
			})
			return normalized, http.StatusOK, nil
		}
	}

	// On error, return Cognito's error response
	if resp.StatusCode != http.StatusOK {
		var cognitoErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &cognitoErr); err == nil && cognitoErr.Error != "" {
			errorResp, _ := json.Marshal(map[string]any{"error": cognitoErr.Error})
			return errorResp, http.StatusUnauthorized, nil
		}
	}

	return body, resp.StatusCode, nil
}

// callAuthService sends a JSON payload to the central auth API Gateway endpoint.
func (h *Handler) callAuthService(ctx context.Context, path string, payload any) ([]byte, int, error) {
	if h.cfg.AuthAPIURL == "" {
		return nil, 0, fmt.Errorf("AUTH_API_URL not configured")
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.AuthAPIURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.AuthAPIKey != "" {
		req.Header.Set("X-API-Key", h.cfg.AuthAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("call auth service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// cognitoAdminAuth calls the Cognito InitiateAuth API directly via HTTPS.
// This is the fallback when the central auth service Lambda is unavailable.
// Uses USER_PASSWORD_AUTH flow which doesn't require SigV4 signing.
func (h *Handler) cognitoAdminAuth(ctx context.Context, username, password string) ([]byte, int, error) {
	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", h.cfg.Cognito.Region)

	payload := map[string]any{
		"AuthFlow": "USER_PASSWORD_AUTH",
		"ClientId": h.cfg.Cognito.AppClientID,
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
func (h *Handler) cognitoRefresh(ctx context.Context, refreshToken string) ([]byte, int, error) {
	endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", h.cfg.Cognito.Region)

	payload := map[string]any{
		"AuthFlow": "REFRESH_TOKEN_AUTH",
		"ClientId": h.cfg.Cognito.AppClientID,
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

// ──────────────────────────────────────────────
// Projects — Real dual-DB queries (IRIS + QS)
// ──────────────────────────────────────────────

// irisStatusName maps IRIS project_status_id to name.
var irisStatusName = map[int]string{
	1: "Inquiry", 2: "Defining", 3: "In Progress", 4: "Complete", 6: "Paused", 7: "Finalizing",
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	log := slog.With("handler", "ListProjects")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	source := r.URL.Query().Get("source") // "iris", "qs", or "" (both)

	// Support serviceCategory filter (LS→qs, MRA→iris)
	if sc := strings.ToUpper(r.URL.Query().Get("serviceCategory")); sc != "" {
		switch sc {
		case "LS":
			source = "qs"
		case "MRA":
			source = "iris"
		}
	}

	var statusID *int
	if s := r.URL.Query().Get("statusId"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			statusID = &v
		}
	}

	var result []map[string]any

	// IRIS projects
	if source == "" || source == "iris" {
		if h.irisProjectRepo != nil {
			irisProjects, irisTotal, err := h.irisProjectRepo.List(r.Context(), page, pageSize, statusID, search)
			if err != nil {
				log.Error("iris project list failed", "error", err)
			} else {
				for _, p := range irisProjects {
					result = append(result, map[string]any{
						"id":                  p.ID,
						"name":                p.Name,
						"description":         nullStr(p.Description),
						"subscriptionId":      p.SubscriptionID,
						"subscriptionCompany": nullStr(p.SubscriptionCompany),
						"statusId":            p.ProjectStatusID,
						"status":              p.ProjectStatusName,
						"projectTypeId":       p.ProjectTypeID,
						"salesforceProjectId": nullStr(p.SalesforceProjectID),
						"isArchived":          p.IsArchived,
						"createdAt":           p.CreatedOn.Format(time.RFC3339),
						"modifiedAt":          nullTime(p.ModifiedOn),
						"source":              "iris",
						"serviceCategory":     "MRA",
					})
				}
				_ = irisTotal
			}
		}
	}

	// QS projects
	if source == "" || source == "qs" {
		if h.qsProjectRepo != nil {
			qsProjects, qsTotal, err := h.qsProjectRepo.List(r.Context(), page, pageSize, statusID, search)
			if err != nil {
				log.Error("qs project list failed", "error", err)
			} else {
				for _, p := range qsProjects {
					result = append(result, map[string]any{
						"id":                  p.ID,
						"name":                p.Name,
						"salesforceJobNumber": nullStr(p.SalesforceJobNumber),
						"clientId":            nullInt64(p.ClientID),
						"clientCompany":       nullStr(p.ClientCompany),
						"sampleSize":          nullInt64(p.SampleSize),
						"interviewLength":     nullInt64(p.InterviewLength),
						"statusId":            p.ProjectStatusID,
						"status":              p.ProjectStatusName,
						"scheduledCount":      p.ScheduledCount,
						"completedCount":      p.CompletedCount,
						"createdAt":           p.CreatedOn.Format(time.RFC3339),
						"modifiedAt":          nullTime(p.ModifiedOn),
						"source":              "qs",
						"serviceCategory":     "LS",
					})
				}
				_ = qsTotal
			}
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": len(result),
		},
	})
}

type createProjectRequest struct {
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	SubscriptionID      int64   `json:"subscriptionId"`
	SalesforceProjectID string  `json:"salesforceProjectId"`
	SalesforceJobNumber string  `json:"salesforceJobNumber"`
	SampleSize          int64   `json:"sampleSize"`
	InterviewLength     int64   `json:"interviewLength"`
	ClientID            int64   `json:"clientId"`
	PostScreeninBuffer  float64 `json:"postScreeninBuffer"`
	ModeratorBuffer     float64 `json:"moderatorBuffer"`
	Source              string  `json:"source"` // "iris" or "qs"
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
		return
	}
	if req.Source == "" {
		req.Source = "qs" // default to QS
	}

	user := middleware.GetUser(r)

	if req.Source == "iris" && h.irisProjectRepo != nil {
		p := &iris.Project{
			Name:                req.Name,
			Description:         toNullStr(req.Description),
			SubscriptionID:      req.SubscriptionID,
			SalesforceProjectID: toNullStr(req.SalesforceProjectID),
			ProjectStatusID:     2, // Defining
		}
		id, err := h.irisProjectRepo.Create(r.Context(), p)
		if err != nil {
			slog.Error("iris project create failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create project"})
			return
		}
		created(w, map[string]any{"id": id, "source": "iris", "serviceCategory": "MRA"})
		return
	}

	if h.qsProjectRepo != nil {
		p := &qs.Project{
			Name:                req.Name,
			SalesforceJobNumber: toNullStr(req.SalesforceJobNumber),
			SampleSize:          toNullInt64(req.SampleSize),
			InterviewLength:     toNullInt64(req.InterviewLength),
			ClientID:            toNullInt64(req.ClientID),
			PostScreeninBuffer:  toNullStr(fmt.Sprintf("%.2f", req.PostScreeninBuffer)),
			ModeratorBuffer:     toNullStr(fmt.Sprintf("%.2f", req.ModeratorBuffer)),
		}
		_ = user // TODO: set CreatedBy from user lookup
		id, err := h.qsProjectRepo.Create(r.Context(), p)
		if err != nil {
			slog.Error("qs project create failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create project"})
			return
		}
		created(w, map[string]any{"id": id, "source": "qs", "serviceCategory": "LS"})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	source := r.URL.Query().Get("source")

	// Try QS first (or if source=qs)
	if (source == "" || source == "qs") && h.qsProjectRepo != nil {
		p, err := h.qsProjectRepo.GetByID(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project get failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if p != nil {
			scheduled, completed, _ := h.qsProjectRepo.TimeSlotCounts(r.Context(), projectID)
			topics, _ := h.qsProjectRepo.GetTopics(r.Context(), projectID)
			topicNames := make([]string, 0, len(topics))
			for _, t := range topics {
				topicNames = append(topicNames, t.TopicName)
			}
			success(w, map[string]any{
				"id":                    p.ID,
				"name":                  p.Name,
				"externalSurveyId":      nullStr(p.ExternalSurveyID),
				"salesforceJobNumber":   nullStr(p.SalesforceJobNumber),
				"clientId":              nullInt64(p.ClientID),
				"sampleSize":            nullInt64(p.SampleSize),
				"interviewLength":       nullInt64(p.InterviewLength),
				"statusId":              p.ProjectStatusID,
				"schedulerGenerated":    p.SchedulerGenerated,
				"postScreeninBuffer":    nullStr(p.PostScreeninBuffer),
				"moderatorBuffer":       nullStr(p.ModeratorBuffer),
				"topics":               topicNames,
				"scheduledCount":        scheduled,
				"completedCount":        completed,
				"createdAt":             p.CreatedOn.Format(time.RFC3339),
				"modifiedAt":            nullTime(p.ModifiedOn),
				"source":               "qs",
				"serviceCategory":       "LS",
			})
			return
		}
	}

	// Try IRIS (or if source=iris)
	if (source == "" || source == "iris") && h.irisProjectRepo != nil {
		p, err := h.irisProjectRepo.GetByID(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project get failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if p != nil {
			success(w, map[string]any{
				"id":                  p.ID,
				"name":                p.Name,
				"description":         nullStr(p.Description),
				"subscriptionId":      p.SubscriptionID,
				"statusId":            p.ProjectStatusID,
				"status":              irisStatusName[p.ProjectStatusID],
				"projectTypeId":       p.ProjectTypeID,
				"salesforceProjectId": nullStr(p.SalesforceProjectID),
				"isPrivate":           p.IsPrivate,
				"isArchived":          p.IsArchived,
				"createdAt":           p.CreatedOn.Format(time.RFC3339),
				"modifiedAt":          nullTime(p.ModifiedOn),
				"source":              "iris",
				"serviceCategory":     "MRA",
			})
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"error": "project not found"})
}

type updateProjectRequest struct {
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	StatusID            *int    `json:"statusId"`
	SalesforceProjectID string  `json:"salesforceProjectId"`
	SampleSize          *int64  `json:"sampleSize"`
	InterviewLength     *int64  `json:"interviewLength"`
	IsArchived          *bool   `json:"isArchived"`
	Source              string  `json:"source"`
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	fields := map[string]any{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.StatusID != nil {
		fields["project_status_id"] = *req.StatusID
	}

	if req.Source == "iris" && h.irisProjectRepo != nil {
		if req.Description != "" {
			fields["description"] = req.Description
		}
		if req.SalesforceProjectID != "" {
			fields["salesforce_project_id"] = req.SalesforceProjectID
		}
		if req.IsArchived != nil {
			fields["is_archived"] = *req.IsArchived
		}
		if err := h.irisProjectRepo.Update(r.Context(), projectID, fields); err != nil {
			slog.Error("iris project update failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		success(w, map[string]any{"id": projectID, "updated": true, "source": "iris"})
		return
	}

	// Default to QS
	if h.qsProjectRepo != nil {
		if req.SampleSize != nil {
			fields["sample_size"] = *req.SampleSize
		}
		if req.InterviewLength != nil {
			fields["interview_length"] = *req.InterviewLength
		}
		if err := h.qsProjectRepo.Update(r.Context(), projectID, fields); err != nil {
			slog.Error("qs project update failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		success(w, map[string]any{"id": projectID, "updated": true, "source": "qs"})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	// Soft-delete: archive instead of hard delete
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	if h.irisProjectRepo != nil {
		if err := h.irisProjectRepo.Update(r.Context(), projectID, map[string]any{"is_archived": true}); err != nil {
			slog.Error("iris project archive failed", "id", projectID, "error", err)
		}
	}
	success(w, map[string]any{"archived": true, "id": projectID})
}

// Null-safe helpers for JSON serialization
func nullStr(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

func nullInt64(ni sql.NullInt64) any {
	if ni.Valid {
		return ni.Int64
	}
	return nil
}

func nullTime(nt sql.NullTime) any {
	if nt.Valid {
		return nt.Time.Format(time.RFC3339)
	}
	return nil
}

func toNullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func toNullInt64(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}

// ──────────────────────────────────────────────
// Surveys
// ──────────────────────────────────────────────

func dummySurvey(sid string) map[string]any {
	return map[string]any{
		"id":          sid,
		"title":       "Screening Survey - Cardiology",
		"description": "Pre-screening questionnaire for cardiology study",
		"status":      "active",
		"questions":   []map[string]any{},
		"rules":       []map[string]any{},
		"createdAt":   "2026-03-01T10:00:00Z",
	}
}

func (h *Handler) ListSurveys(w http.ResponseWriter, r *http.Request) {
	surveys := []map[string]any{
		dummySurvey("surv-1"),
		dummySurvey("surv-2"),
	}
	surveys[1]["title"] = "Screening Survey - Oncology"
	writeJSON(w, http.StatusOK, surveys)
}

func (h *Handler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	s := dummySurvey("surv-" + id()[:8])
	s["createdAt"] = now()
	created(w, s)
}

func (h *Handler) UpdateSurvey(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "id")
	s := dummySurvey(sid)
	s["updatedAt"] = now()
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

func (h *Handler) GetPublicSurvey(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "surveyId")
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          sid,
		"projectId":   "proj-101",
		"projectName": "Cardiology Study Q1",
		"questions": []map[string]any{
			{"id": "q1", "type": "single_choice", "text": "What is your specialty?",
				"choices": []string{"Cardiology", "Oncology", "Neurology", "Other"}},
			{"id": "q2", "type": "text", "text": "Years of experience?"},
		},
		"rules": []map[string]any{},
	})
}

// ──────────────────────────────────────────────
// Survey Responses
// ──────────────────────────────────────────────

func (h *Handler) GetSurveyResponses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified",
			"submittedAt": "2026-03-15T10:00:00Z",
		},
	})
}

func (h *Handler) SubmitSurveyResponse(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": "resp-" + id()[:8], "status": "qualified", "submittedAt": now(),
	})
}

func (h *Handler) SubmitParticipantSurvey(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": "resp-" + id()[:8], "status": "qualified", "submittedAt": now(),
	})
}

func (h *Handler) GetParticipantSurveyResponse(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified"},
	})
}

// ──────────────────────────────────────────────
// Timeslots
// ──────────────────────────────────────────────

func dummyTimeslot(tid string) map[string]any {
	return map[string]any{
		"id":            tid,
		"moderatorId":   "mod-201",
		"moderatorName": "Jane Smith",
		"start":         "2026-04-01T09:00:00Z",
		"end":           "2026-04-01T09:30:00Z",
		"type":          "availability",
		"projectId":     "proj-101",
		"timezone":      "America/New_York",
		"createdAt":     "2026-03-20T10:00:00Z",
	}
}

func (h *Handler) ListTimeslots(w http.ResponseWriter, r *http.Request) {
	slots := []map[string]any{
		dummyTimeslot("ts-1"),
		dummyTimeslot("ts-2"),
		dummyTimeslot("ts-3"),
	}
	slots[1]["start"] = "2026-04-01T10:00:00Z"
	slots[1]["end"] = "2026-04-01T10:30:00Z"
	slots[1]["type"] = "interview"
	slots[1]["participant"] = "Alice Johnson"
	slots[2]["start"] = "2026-04-02T09:00:00Z"
	slots[2]["end"] = "2026-04-02T17:00:00Z"
	writeJSON(w, http.StatusOK, slots)
}

func (h *Handler) CreateTimeslot(w http.ResponseWriter, r *http.Request) {
	ts := dummyTimeslot("ts-" + id()[:8])
	ts["createdAt"] = now()
	created(w, ts)
}

func (h *Handler) GetTimeslot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dummyTimeslot(chi.URLParam(r, "id")))
}

func (h *Handler) UpdateTimeslot(w http.ResponseWriter, r *http.Request) {
	ts := dummyTimeslot(chi.URLParam(r, "id"))
	ts["updatedAt"] = now()
	writeJSON(w, http.StatusOK, ts)
}

func (h *Handler) DeleteTimeslot(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

// ──────────────────────────────────────────────
// Interview Slots
// ──────────────────────────────────────────────

func (h *Handler) GenerateSlots(w http.ResponseWriter, r *http.Request) {
	slots := []map[string]any{
		{"id": "slot-1", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T09:00:00Z", "end": "2026-04-01T09:30:00Z", "capacity": 1},
		{"id": "slot-2", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T09:30:00Z", "end": "2026-04-01T10:00:00Z", "capacity": 1},
		{"id": "slot-3", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T10:00:00Z", "end": "2026-04-01T10:30:00Z", "capacity": 1},
	}
	writeJSON(w, http.StatusOK, slots)
}

func (h *Handler) GetAvailableSlots(w http.ResponseWriter, r *http.Request) {
	slots := []map[string]any{
		{"id": "slot-1", "projectId": "proj-101", "moderatorId": "mod-201", "moderatorName": "Jane Smith",
			"start": "2026-04-01T09:00:00Z", "end": "2026-04-01T09:30:00Z", "capacity": 1},
		{"id": "slot-2", "projectId": "proj-101", "moderatorId": "mod-201", "moderatorName": "Jane Smith",
			"start": "2026-04-01T10:00:00Z", "end": "2026-04-01T10:30:00Z", "capacity": 1},
	}
	writeJSON(w, http.StatusOK, slots)
}

func (h *Handler) GetAISuggestions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"projectId": r.URL.Query().Get("projectId"),
		"suggestions": []map[string]any{
			{"suggestedStart": "2026-04-01T09:00:00Z", "suggestedEnd": "2026-04-01T09:30:00Z", "participantCount": 3},
			{"suggestedStart": "2026-04-01T14:00:00Z", "suggestedEnd": "2026-04-01T14:30:00Z", "participantCount": 5},
		},
	})
}

// ──────────────────────────────────────────────
// Moderators (real dual-DB)
// ──────────────────────────────────────────────

// qsRoleName maps QS role_id to a human-readable name.
func qsRoleName(id int) string {
	switch id {
	case 1:
		return "moderator"
	case 2:
		return "manager"
	case 3:
		return "admin"
	default:
		return fmt.Sprintf("role_%d", id)
	}
}

// parseRoleCSV splits a comma-separated role-id string from GROUP_CONCAT.
func parseRoleCSV(csv string) []int {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			ids = append(ids, v)
		}
	}
	return ids
}

func (h *Handler) ListModerators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var combined []map[string]any

	// QS moderators (role_id = 1)
	if h.qsUserRepo != nil {
		mods, err := h.qsUserRepo.GetModerators(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "list QS moderators failed", "error", err)
		} else {
			for _, m := range mods {
				roles := []string{}
				for _, rid := range parseRoleCSV(m.RoleIDs) {
					roles = append(roles, qsRoleName(rid))
				}
				combined = append(combined, map[string]any{
					"id":        m.ID,
					"name":      strings.TrimSpace(m.FirstName.String + " " + m.LastName.String),
					"email":     m.Email.String,
					"role":      "moderator",
					"roles":     roles,
					"status":    "active",
					"source":    "qs",
					"serviceCategory": "LS",
					"timezone":  m.TimeZone.String,
					"updatedAt": m.ModifiedOn.Format(time.RFC3339),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, combined)
}

func (h *Handler) CreateModerator(w http.ResponseWriter, r *http.Request) {
	// Create moderator is still a stub — user creation requires Cognito + DB coordination
	m := map[string]any{
		"id":        "mod-" + id()[:8],
		"name":      "New Moderator",
		"email":     "new@konovo.com",
		"role":      "moderator",
		"status":    "active",
		"createdAt": now(),
	}
	created(w, m)
}

func (h *Handler) GetModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modID := chi.URLParam(r, "id")
	nid, err := strconv.ParseInt(modID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid moderator id"})
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		source = "qs" // default to QS for moderators
	}

	switch source {
	case "qs":
		if h.qsUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
			return
		}
		u, err := h.qsUserRepo.GetByID(ctx, nid)
		if err != nil {
			slog.ErrorContext(ctx, "get QS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "moderator not found"})
			return
		}
		roles := []string{}
		for _, rid := range u.RoleIDs {
			roles = append(roles, qsRoleName(rid))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":              u.ID,
			"firstName":       u.FirstName.String,
			"lastName":        u.LastName.String,
			"email":           u.Email.String,
			"roles":           roles,
			"timezone":        u.TimeZone.String,
			"moderatorBuffer": u.ModeratorBuffer.Int64,
			"termsAccepted":   u.TermsAccepted == 1,
			"source":          "qs",
			"serviceCategory": "LS",
			"modifiedOn":      u.ModifiedOn.Format(time.RFC3339),
		})
	case "iris":
		if h.irisUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "IRIS database unavailable"})
			return
		}
		u, err := h.irisUserRepo.GetByID(ctx, nid)
		if err != nil {
			slog.ErrorContext(ctx, "get IRIS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "moderator not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":              u.ID,
			"firstName":       u.FirstName,
			"lastName":        u.LastName,
			"email":           u.Email.String,
			"roles":           u.RoleNames,
			"timezone":        u.TimeZone.String,
			"source":          "iris",
			"serviceCategory": "MRA",
			"lastLogin":       u.LastLogin.Time.Format(time.RFC3339),
			"registeredAt":    u.RegistrationDate.Format(time.RFC3339),
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid source, use qs or iris"})
	}
}

func (h *Handler) UpdateModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modID := chi.URLParam(r, "id")
	nid, err := strconv.ParseInt(modID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid moderator id"})
		return
	}

	var body struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		TimeZone  string `json:"timezone"`
		Source    string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	if body.Source == "" {
		body.Source = "qs"
	}

	switch body.Source {
	case "qs":
		if h.qsUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
			return
		}
		if err := h.qsUserRepo.Update(ctx, nid, body.FirstName, body.LastName, body.TimeZone); err != nil {
			slog.ErrorContext(ctx, "update QS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
	case "iris":
		if h.irisUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "IRIS database unavailable"})
			return
		}
		if err := h.irisUserRepo.Update(ctx, nid, body.FirstName, body.LastName, body.TimeZone); err != nil {
			slog.ErrorContext(ctx, "update IRIS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid source"})
		return
	}

	success(w, map[string]any{"updated": true, "id": nid})
}

func (h *Handler) DeleteModerator(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

func (h *Handler) BulkUploadModerators(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"created": 0, "failed": 0, "errors": []any{}, "message": "bulk upload not yet implemented",
	})
}

func (h *Handler) GetModeratorTimeslots(w http.ResponseWriter, r *http.Request) {
	modID := chi.URLParam(r, "moderatorId")
	slots := []map[string]any{
		dummyTimeslot("ts-m-1"),
		dummyTimeslot("ts-m-2"),
	}
	slots[0]["moderatorId"] = modID
	slots[1]["moderatorId"] = modID
	slots[1]["start"] = "2026-04-02T09:00:00Z"
	slots[1]["end"] = "2026-04-02T17:00:00Z"
	writeJSON(w, http.StatusOK, slots)
}

// ──────────────────────────────────────────────
// Participants
// ──────────────────────────────────────────────

func dummyParticipant(pid string) map[string]any {
	return map[string]any{
		"id":        pid,
		"name":      "Alice Johnson",
		"email":     "alice@hospital.org",
		"phone":     "+15559876543",
		"role":      "participant",
		"status":    "active",
		"createdAt": "2026-03-10T10:00:00Z",
	}
}

func (h *Handler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	parts := []map[string]any{
		dummyParticipant("par-301"),
		dummyParticipant("par-302"),
		dummyParticipant("par-303"),
	}
	parts[1]["name"] = "Bob Patient"
	parts[1]["email"] = "bob@clinic.org"
	parts[2]["name"] = "Carol Respondent"
	parts[2]["email"] = "carol@lab.org"
	writeJSON(w, http.StatusOK, parts)
}

func (h *Handler) CreateParticipant(w http.ResponseWriter, r *http.Request) {
	p := dummyParticipant("par-" + id()[:8])
	p["createdAt"] = now()
	created(w, p)
}

func (h *Handler) GetParticipant(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	p := dummyParticipant(pid)
	p["surveyResponse"] = map[string]any{
		"id": "resp-1", "projectId": "proj-101", "status": "qualified",
		"submittedAt": "2026-03-15T10:00:00Z",
		"answers":     map[string]any{"q1": "Cardiology", "q2": "10"},
	}
	p["booking"] = map[string]any{
		"id": "bk-1", "projectId": "proj-101", "slotId": "ts-1",
		"moderatorId": "mod-201", "moderatorName": "Jane Smith",
		"slotStart": "2026-04-01T09:00:00Z", "slotEnd": "2026-04-01T09:30:00Z",
		"status": "scheduled",
	}
	writeJSON(w, http.StatusOK, p)
}

// ──────────────────────────────────────────────
// Bookings
// ──────────────────────────────────────────────

func dummyBooking(bid string) map[string]any {
	return map[string]any{
		"id":              bid,
		"userId":          "par-301",
		"projectId":       "proj-101",
		"projectName":     "Cardiology Study Q1",
		"slotId":          "ts-1",
		"moderatorId":     "mod-201",
		"moderatorName":   "Jane Smith",
		"participantName": "Alice Johnson",
		"slotStart":       "2026-04-01T09:00:00Z",
		"slotEnd":         "2026-04-01T09:30:00Z",
		"meetingLink":     "https://meet.example.com/abc",
		"status":          "scheduled",
		"rewardPoints":    150,
		"rewardStatus":    "not_credited",
		"createdAt":       "2026-03-20T10:00:00Z",
	}
}

func (h *Handler) ListBookings(w http.ResponseWriter, r *http.Request) {
	bookings := []map[string]any{
		dummyBooking("bk-1"),
		dummyBooking("bk-2"),
		dummyBooking("bk-3"),
	}
	bookings[1]["participantName"] = "Bob Patient"
	bookings[1]["status"] = "completed"
	bookings[1]["rewardStatus"] = "credited"
	bookings[2]["participantName"] = "Carol Respondent"
	bookings[2]["status"] = "cancelled"
	writeJSON(w, http.StatusOK, bookings)
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	b := dummyBooking("bk-" + id()[:8])
	b["createdAt"] = now()
	created(w, b)
}

func (h *Handler) GetBookingsByUser(w http.ResponseWriter, r *http.Request) {
	b := dummyBooking("bk-1")
	b["userId"] = chi.URLParam(r, "userId")
	writeJSON(w, http.StatusOK, []map[string]any{b})
}

func (h *Handler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	b := dummyBooking(chi.URLParam(r, "id"))
	b["updatedAt"] = now()
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) UpdateBookingReward(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"updated": true})
}

// ──────────────────────────────────────────────
// Subscriptions
// ──────────────────────────────────────────────

func dummySubscription(sid string) map[string]any {
	return map[string]any{
		"id":                  sid,
		"company":             "Konovo Health",
		"plan":                "enterprise",
		"shortCode":           "KH",
		"serviceType":         "full-service",
		"businessType":        "pharmaceutical",
		"salesforceAccount":   "SF-ACC-001",
		"salesContact":        "sales@konovo.com",
		"pmContact":           "pm@konovo.com",
		"csUser":              "cs@konovo.com",
		"currency":            "USD",
		"markets":             "US,EU",
		"panels":              "HCP",
		"phone":               "+15551234567",
		"aeConsent":           true,
		"aeReporting":         "quarterly",
		"skipSfValidation":    false,
		"createdAt":           "2025-01-01T00:00:00Z",
	}
}

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs := []map[string]any{
		dummySubscription("sub-1"),
		dummySubscription("sub-2"),
	}
	subs[1]["company"] = "Apollo Research"
	subs[1]["shortCode"] = "AR"
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	s := dummySubscription("sub-" + id()[:8])
	s["createdAt"] = now()
	created(w, s)
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, dummySubscription(chi.URLParam(r, "id")))
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	s := dummySubscription(chi.URLParam(r, "id"))
	s["updatedAt"] = now()
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

// ──────────────────────────────────────────────
// Waiting Queue
// ──────────────────────────────────────────────

func (h *Handler) GetWaitingQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "wq-1", "userId": "par-301", "projectId": "proj-101",
			"participantName": "Alice Johnson", "participantEmail": "alice@hospital.org",
			"projectName": "Cardiology Study Q1",
			"preferredStart": "09:00", "preferredEnd": "17:00",
			"preferredDays": []string{"Monday", "Wednesday", "Friday"},
			"timezone": "America/New_York", "status": "waiting",
			"waitingSince": "2026-03-18T10:00:00Z",
		},
		{
			"id": "wq-2", "userId": "par-302", "projectId": "proj-101",
			"participantName": "Bob Patient", "participantEmail": "bob@clinic.org",
			"projectName": "Cardiology Study Q1",
			"preferredStart": "10:00", "preferredEnd": "14:00",
			"preferredDays": []string{"Tuesday", "Thursday"},
			"timezone": "America/Chicago", "status": "waiting",
			"waitingSince": "2026-03-19T10:00:00Z",
		},
	})
}

func (h *Handler) AddToWaitingQueue(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"id": "wq-" + id()[:8], "status": "waiting", "waitingSince": now(),
	})
}

func (h *Handler) RemoveFromWaitingQueue(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"deleted": true})
}

func (h *Handler) TriggerMatching(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"assigned": 2, "invited": 3, "message": "Matching complete. 2 assigned, 3 invited.",
	})
}

// ──────────────────────────────────────────────
// Scheduler (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ScheduleInterview(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{
		"interviewId":    "int-" + id()[:8],
		"timeSlotId":     301,
		"conferenceLink": "https://chime.aws/meeting/abc123",
		"startTime":      "2026-04-01T14:00:00Z",
		"endTime":        "2026-04-01T14:30:00Z",
		"status":         "scheduled",
	})
}

func (h *Handler) CancelInterview(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"interviewId": chi.URLParam(r, "id"), "status": "canceled",
	})
}

func (h *Handler) RescheduleInterview(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"interviewId": chi.URLParam(r, "id"), "status": "rescheduled",
	})
}

// ──────────────────────────────────────────────
// Moderator Availability (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modID := chi.URLParam(r, "id")
	nid, err := strconv.ParseInt(modID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid moderator id"})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var clientID *int64
	if cid := r.URL.Query().Get("clientId"); cid != "" {
		v, err := strconv.ParseInt(cid, 10, 64)
		if err == nil {
			clientID = &v
		}
	}
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	avails, err := h.qsUserRepo.ListModeratorAvailability(ctx, nid, clientID, startDate, endDate)
	if err != nil {
		slog.ErrorContext(ctx, "list moderator availability failed", "error", err, "moderatorId", nid)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to fetch availability"})
		return
	}

	items := make([]map[string]any, 0, len(avails))
	for _, a := range avails {
		items = append(items, map[string]any{
			"id":          a.ID,
			"moderatorId": a.ModeratorID,
			"clientId":    a.ClientID,
			"startTime":   a.StartTime.Format(time.RFC3339),
			"endTime":     a.EndTime.Format(time.RFC3339),
			"isImported":  false,
		})
	}
	success(w, map[string]any{"availabilities": items})
}

func (h *Handler) PostModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modID := chi.URLParam(r, "id")
	nid, err := strconv.ParseInt(modID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid moderator id"})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		ClientID  int64  `json:"clientId"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	st, err := time.Parse(time.RFC3339, body.StartTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid startTime format, use RFC3339"})
		return
	}
	et, err := time.Parse(time.RFC3339, body.EndTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid endTime format, use RFC3339"})
		return
	}

	avail, err := h.qsUserRepo.CreateModeratorAvailability(ctx, nid, body.ClientID, st, et)
	if err != nil {
		slog.ErrorContext(ctx, "create moderator availability failed", "error", err, "moderatorId", nid)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create availability"})
		return
	}

	created(w, map[string]any{
		"id":          avail.ID,
		"moderatorId": avail.ModeratorID,
		"startTime":   avail.StartTime.Format(time.RFC3339),
		"endTime":     avail.EndTime.Format(time.RFC3339),
	})
}

func (h *Handler) GetTimeslotModeratorOptions(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"availableModerators": []map[string]any{
			{"id": "mod-201", "firstName": "Jane", "lastName": "Smith",
				"hasConflict": false, "availabilityId": 801},
			{"id": "mod-202", "firstName": "Bob", "lastName": "Wilson",
				"hasConflict": true, "conflictReason": "Overlapping interview at 14:00-14:30"},
		},
	})
}

// ──────────────────────────────────────────────
// Conference (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) MeetingAction(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"meetingId": chi.URLParam(r, "meetingId"),
		"action":    chi.URLParam(r, "action"),
		"result":    "success",
	})
}

func (h *Handler) MeetingUniversalJoin(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"joinUrl":    "https://chime.aws/join/" + chi.URLParam(r, "meetingId"),
		"attendeeId": "att-" + id()[:8],
	})
}

// ──────────────────────────────────────────────
// Payments (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	created(w, map[string]any{"paymentId": 7001, "status": "PENDING"})
}

func (h *Handler) CreateCustomHonorarium(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"timeSlotId": 301, "honorarium": 200.00, "reasonId": 2})
}

func (h *Handler) GetPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"payments": []map[string]any{
			{"timeSlotId": 301, "respondentName": "Alice Johnson", "amount": 150.00,
				"currency": "USD", "status": "PENDING", "source": "QS",
				"updatedAt": "2026-03-20T12:00:00Z"},
		},
	})
}

// ──────────────────────────────────────────────
// Translations (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetLocales(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"locales": []map[string]string{
			{"code": "en_us", "name": "English (US)"},
			{"code": "es_es", "name": "Spanish"},
			{"code": "fr_fr", "name": "French"},
			{"code": "fr_ca", "name": "French (Canada)"},
			{"code": "de_de", "name": "German"},
			{"code": "it_it", "name": "Italian"},
			{"code": "pt_pt", "name": "Portuguese"},
		},
	})
}

func (h *Handler) UpdateTopicTranslations(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"projectId":         chi.URLParam(r, "projectId"),
		"translationsCount": 2,
	})
}

// ──────────────────────────────────────────────
// Notifications (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{
		"subject":      "Your Interview Has Been Rescheduled",
		"body":         "<html><body><p>Dear {{.Name}}, your interview has been rescheduled.</p></body></html>",
		"templateType": r.URL.Query().Get("type"),
		"language":     r.URL.Query().Get("language"),
	})
}

func (h *Handler) SendReminder(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]any{"sent": true, "recipientCount": 2})
}

// ──────────────────────────────────────────────
// Admin (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, pageSize := parsePagination(r)
	search := r.URL.Query().Get("search")
	source := r.URL.Query().Get("source") // "qs", "iris", or "" (both)

	var allUsers []map[string]any

	// QS users
	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		users, total, err := h.qsUserRepo.List(ctx, page, pageSize, nil, search)
		if err != nil {
			slog.ErrorContext(ctx, "list QS users failed", "error", err)
		} else {
			for _, u := range users {
				roles := []string{}
				for _, rid := range parseRoleCSV(u.RoleIDs) {
					roles = append(roles, qsRoleName(rid))
				}
				allUsers = append(allUsers, map[string]any{
					"id":              u.ID,
					"firstName":       u.FirstName.String,
					"lastName":        u.LastName.String,
					"email":           u.Email.String,
					"roles":           roles,
					"source":          "qs",
					"serviceCategory": "LS",
					"updatedAt":       u.ModifiedOn.Format(time.RFC3339),
				})
			}
			_ = total // used when source-specific pagination implemented
		}
	}

	// IRIS users
	if (source == "" || source == "iris") && h.irisUserRepo != nil {
		users, total, err := h.irisUserRepo.List(ctx, page, pageSize, nil, search)
		if err != nil {
			slog.ErrorContext(ctx, "list IRIS users failed", "error", err)
		} else {
			for _, u := range users {
				lastLogin := ""
				if u.LastLogin.Valid {
					lastLogin = u.LastLogin.Time.Format(time.RFC3339)
				}
				allUsers = append(allUsers, map[string]any{
					"id":              u.ID,
					"firstName":       u.FirstName,
					"lastName":        u.LastName,
					"email":           u.Email.String,
					"roles":           strings.Split(u.RoleNames, ","),
					"source":          "iris",
					"serviceCategory": "MRA",
					"lastLogin":       lastLogin,
					"registeredAt":    u.RegistrationDate.Format(time.RFC3339),
				})
			}
			_ = total
		}
	}

	successList(w, map[string]any{"users": allUsers}, len(allUsers))
}

func (h *Handler) CreateAdminUser(w http.ResponseWriter, r *http.Request) {
	// User creation requires Cognito coordination — stub for now
	created(w, map[string]any{
		"id": 0, "message": "user creation requires Cognito coordination, not yet implemented",
	})
}
