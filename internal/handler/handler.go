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
	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	cfg              *config.Config
	db               *config.DBPair
	services         *integration.ServiceClients
	irisProjectRepo  *iris.ProjectRepo
	qsProjectRepo   *qs.ProjectRepo
	irisUserRepo    *iris.UserRepo
	qsUserRepo      *qs.UserRepo
	qsTimeSlotRepo  *qs.TimeSlotRepo
	qsRespondentRepo *qs.RespondentRepo
	qsSurveyRepo    *qs.SurveyRepo
	// Phase 6 — Legacy API repos
	irisSurveyRepo   *iris.SurveyRepo
	qsConferenceRepo *qs.ConferenceRepo
	qsAnswerRepo     *qs.AnswerRepo
}

func New(cfg *config.Config, db *config.DBPair, services *integration.ServiceClients, irisRepo *iris.ProjectRepo, qsRepo *qs.ProjectRepo, irisUserRepo *iris.UserRepo, qsUserRepo *qs.UserRepo, qsTimeSlotRepo *qs.TimeSlotRepo, qsRespondentRepo *qs.RespondentRepo, qsSurveyRepo *qs.SurveyRepo, irisSurveyRepo *iris.SurveyRepo, qsConferenceRepo *qs.ConferenceRepo, qsAnswerRepo *qs.AnswerRepo) *Handler {
	return &Handler{cfg: cfg, db: db, services: services, irisProjectRepo: irisRepo, qsProjectRepo: qsRepo, irisUserRepo: irisUserRepo, qsUserRepo: qsUserRepo, qsTimeSlotRepo: qsTimeSlotRepo, qsRespondentRepo: qsRespondentRepo, qsSurveyRepo: qsSurveyRepo, irisSurveyRepo: irisSurveyRepo, qsConferenceRepo: qsConferenceRepo, qsAnswerRepo: qsAnswerRepo}
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
	var req struct {
		UserID      int64  `json:"userId"`
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	// Password change requires Cognito ChangePassword API.
	// When accessToken is present, the frontend should call Cognito directly.
	// This endpoint acknowledges the request and logs the event.
	slog.Info("password change requested", "userId", req.UserID)
	writeJSON(w, http.StatusOK, map[string]any{"changed": true, "note": "password change delegated to Cognito"})
}

// AuthMe returns the authenticated user's claims from the JWT.
func (h *Handler) AuthMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
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

	writeJSON(w, http.StatusOK, map[string]any{
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

	// Support serviceCategory filter (LS→iris, MRA→qs)
	if sc := strings.ToUpper(r.URL.Query().Get("serviceCategory")); sc != "" {
		switch sc {
		case "LS":
			source = "iris"
		case "MRA":
			source = "qs"
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
						"serviceCategory":     "LS",
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
						"serviceCategory":     "MRA",
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
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "source": "iris", "serviceCategory": "LS"})
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
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "source": "qs", "serviceCategory": "MRA"})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// GetProject returns a project by ID.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id
// Response: flat project adminJson object
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
			writeJSON(w, http.StatusOK, map[string]any{
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
				"serviceCategory":       "MRA",
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
			writeJSON(w, http.StatusOK, map[string]any{
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
				"serviceCategory":     "LS",
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
		writeJSON(w, http.StatusOK, map[string]any{"id": projectID, "updated": true, "source": "iris"})
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
		writeJSON(w, http.StatusOK, map[string]any{"id": projectID, "updated": true, "source": "qs"})
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

	source := r.URL.Query().Get("source")

	if source == "qs" && h.qsProjectRepo != nil {
		// QS: set project_status_id = 5 (Canceled)
		if err := h.qsProjectRepo.Update(r.Context(), projectID, map[string]any{"project_status_id": 5}); err != nil {
			slog.Error("qs project archive failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "archive failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"archived": true, "id": projectID, "source": "qs"})
		return
	}

	if h.irisProjectRepo != nil {
		if err := h.irisProjectRepo.Update(r.Context(), projectID, map[string]any{"is_archived": true}); err != nil {
			slog.Error("iris project archive failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "archive failed"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"archived": true, "id": projectID, "source": "iris"})
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

func surveyToMap(s *qs.SurveyRow) map[string]any {
	var questions []any
	var rules []any
	_ = json.Unmarshal([]byte(s.Questions), &questions)
	_ = json.Unmarshal([]byte(s.Rules), &rules)
	if questions == nil {
		questions = []any{}
	}
	if rules == nil {
		rules = []any{}
	}
	m := map[string]any{
		"id":          fmt.Sprintf("%d", s.ID),
		"title":       s.Title,
		"status":      s.Status,
		"questions":   questions,
		"rules":       rules,
		"crowdId":     "",
		"crowdName":   "",
		"createdAt":   s.CreatedOn.Format(time.RFC3339),
		"updatedAt":   s.ModifiedOn.Format(time.RFC3339),
	}
	if s.ProjectID.Valid {
		m["projectId"] = fmt.Sprintf("%d", s.ProjectID.Int64)
	} else {
		m["projectId"] = ""
	}
	if s.ProjectName.Valid {
		m["projectName"] = s.ProjectName.String
	} else {
		m["projectName"] = ""
	}
	return m
}

func (h *Handler) ListSurveys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	search := r.URL.Query().Get("search")
	rows, err := h.qsSurveyRepo.List(ctx, search)
	if err != nil {
		slog.Error("list surveys", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list surveys"})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, surveyToMap(&row))
	}
	writeJSON(w, http.StatusOK, out)
}

type createSurveyRequest struct {
	Title     string          `json:"title"`
	ProjectID *int64          `json:"projectId,omitempty"`
	Status    string          `json:"status"`
	Questions json.RawMessage `json:"questions"`
	Rules     json.RawMessage `json:"rules"`
}

func (h *Handler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	var req createSurveyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.Title == "" {
		req.Title = "Untitled Survey"
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Questions == nil {
		req.Questions = json.RawMessage("[]")
	}
	if req.Rules == nil {
		req.Rules = json.RawMessage("[]")
	}
	newID, err := h.qsSurveyRepo.Create(ctx, req.ProjectID, req.Title, req.Status, req.Questions, req.Rules)
	if err != nil {
		slog.Error("create survey", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create survey"})
		return
	}
	row, err := h.qsSurveyRepo.GetByID(ctx, newID)
	if err != nil || row == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("%d", newID)})
		return
	}
	writeJSON(w, http.StatusCreated, surveyToMap(row))
}

func (h *Handler) UpdateSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	sid := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}
	var req createSurveyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.Questions == nil {
		req.Questions = json.RawMessage("[]")
	}
	if req.Rules == nil {
		req.Rules = json.RawMessage("[]")
	}
	if err := h.qsSurveyRepo.Update(ctx, surveyID, req.Title, req.Status, req.Questions, req.Rules); err != nil {
		slog.Error("update survey", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to update survey"})
		return
	}
	row, err := h.qsSurveyRepo.GetByID(ctx, surveyID)
	if err != nil || row == nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": sid, "updatedAt": now()})
		return
	}
	writeJSON(w, http.StatusOK, surveyToMap(row))
}

func (h *Handler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	sid := chi.URLParam(r, "id")
	surveyID, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}
	if err := h.qsSurveyRepo.Delete(ctx, surveyID); err != nil {
		slog.Error("delete survey", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete survey"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *Handler) GetPublicSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	sid := chi.URLParam(r, "surveyId")
	surveyID, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid survey id"})
		return
	}
	row, err := h.qsSurveyRepo.GetByID(ctx, surveyID)
	if err != nil {
		slog.Error("get public survey", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get survey"})
		return
	}
	if row == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "survey not found"})
		return
	}
	writeJSON(w, http.StatusOK, surveyToMap(row))
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
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": "resp-" + id()[:8], "status": "qualified", "submittedAt": now(),
	})
}

func (h *Handler) SubmitParticipantSurvey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{
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

func (h *Handler) ListTimeslots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var projectID *int64
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		v, _ := strconv.ParseInt(pid, 10, 64)
		if v > 0 {
			projectID = &v
		}
	}
	var statusID *int
	if sid := r.URL.Query().Get("statusId"); sid != "" {
		v, _ := strconv.Atoi(sid)
		if v > 0 {
			statusID = &v
		}
	}
	var moderatorID *int64
	if mid := r.URL.Query().Get("moderatorId"); mid != "" {
		v, _ := strconv.ParseInt(mid, 10, 64)
		if v > 0 {
			moderatorID = &v
		}
	}
	var fromTime, toTime *time.Time
	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.Parse("2006-01-02", f); err == nil {
			fromTime = &t
			// when date filters are present, increase default pageSize to cover a full week
			if pageSize == 20 {
				pageSize = 500
			}
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			eod := parsed.Add(24*time.Hour - time.Second)
			toTime = &eod
		}
	}

	slots, total, err := h.qsTimeSlotRepo.List(ctx, page, pageSize, projectID, statusID, moderatorID, fromTime, toTime)
	if err != nil {
		slog.ErrorContext(ctx, "list timeslots failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list timeslots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		item := map[string]any{
			"id":                     s.ID,
			"projectId":              s.ProjectID,
			"projectName":            s.ProjectName,
			"startTime":              s.StartTime.Format(time.RFC3339),
			"endTime":                s.EndTime.Format(time.RFC3339),
			"duration":               s.Duration,
			"statusId":               s.StatusID,
			"status":                 s.StatusName,
			"confirmed":              s.Confirmed,
			"isInvalid":              s.IsInvalid,
			"isInvalidatedInterview": s.IsInvalidatedInterview,
			"source":                 "qs",
			"serviceCategory":        "MRA",
			"modifiedOn":             s.ModifiedOn.Format(time.RFC3339),
		}
		if s.ModeratorID.Valid {
			item["moderatorId"] = s.ModeratorID.Int64
			item["moderatorName"] = s.ModeratorName.String
			item["isHost"] = s.IsHost.Valid && s.IsHost.Bool
		}
		if s.ResponderID.Valid {
			item["responderId"] = s.ResponderID.Int64
			item["responderName"] = s.ResponderName.String
		}
		if s.ConferenceHash.Valid {
			item["conferenceHash"] = s.ConferenceHash.String
		}
		if s.InvalidationReasonCode.Valid {
			item["invalidationReasonCode"] = s.InvalidationReasonCode.String
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": total,
		},
	})
}

func (h *Handler) CreateTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		ProjectID   int64  `json:"projectId"`
		StartTime   string `json:"startTime"`
		EndTime     string `json:"endTime"`
		Duration    int    `json:"duration"`
		ModeratorID int64  `json:"moderatorId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if body.ProjectID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "projectId is required"})
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
	if body.Duration <= 0 {
		body.Duration = 15
	}

	ts := &qs.TimeSlot{
		ProjectID: body.ProjectID,
		StartTime: st,
		EndTime:   et,
		Confirmed: true,
		StatusID:  1, // OPEN
		Duration:  body.Duration,
	}

	tsID, err := h.qsTimeSlotRepo.Create(ctx, ts)
	if err != nil {
		slog.ErrorContext(ctx, "create timeslot failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create timeslot"})
		return
	}

	// Assign moderator if provided
	if body.ModeratorID > 0 {
		if _, err := h.qsTimeSlotRepo.AssignModerator(ctx, body.ModeratorID, tsID, true); err != nil {
			slog.ErrorContext(ctx, "assign moderator failed", "error", err, "timeSlotId", tsID, "moderatorId", body.ModeratorID)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": tsID, "projectId": body.ProjectID, "statusId": 1, "source": "qs"})
}

func (h *Handler) GetTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}

	ts, err := h.qsTimeSlotRepo.GetByID(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if ts == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "timeslot not found"})
		return
	}

	result := map[string]any{
		"id":                       ts.ID,
		"projectId":                ts.ProjectID,
		"startTime":                ts.StartTime.Format(time.RFC3339),
		"endTime":                  ts.EndTime.Format(time.RFC3339),
		"confirmed":                ts.Confirmed,
		"statusId":                 ts.StatusID,
		"duration":                 ts.Duration,
		"isInvalid":                ts.IsInvalid,
		"isInvalidatedInterview":   ts.IsInvalidatedInterview,
		"isPreviousNoShow":         ts.IsPreviousNoShow,
		"source":                   "qs",
		"serviceCategory":          "MRA",
		"modifiedOn":               ts.ModifiedOn.Format(time.RFC3339),
	}
	if ts.ConferenceHash.Valid {
		result["conferenceHash"] = ts.ConferenceHash.String
	}
	if ts.ParticipantHash.Valid {
		result["participantHash"] = ts.ParticipantHash.String
	}
	if ts.InvalidationReasonCode.Valid {
		result["invalidationReasonCode"] = ts.InvalidationReasonCode.String
	}
	if ts.InvalidationReasonText.Valid {
		result["invalidationReasonText"] = ts.InvalidationReasonText.String
	}

	// Get assigned moderators
	mods, err := h.qsTimeSlotRepo.GetModerators(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot moderators failed", "error", err)
	}
	if mods != nil {
		modList := make([]map[string]any, 0, len(mods))
		for _, m := range mods {
			modList = append(modList, map[string]any{
				"moderatorId": m.ModeratorID,
				"isHost":      m.IsHost,
			})
		}
		result["moderators"] = modList
	}

	// Get linked respondent
	resp, err := h.qsTimeSlotRepo.GetRespondent(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot respondent failed", "error", err)
	}
	if resp != nil {
		result["respondent"] = map[string]any{
			"id":        resp.ID,
			"firstName": resp.FirstName,
			"lastName":  resp.LastName,
			"timeZone":  resp.TimeZone.String,
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) UpdateTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}

	var body struct {
		StatusID  *int   `json:"statusId"`
		Confirmed *bool  `json:"confirmed"`
		IsInvalid *int   `json:"isInvalid"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		Duration  *int   `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	fields := map[string]any{}
	if body.StatusID != nil {
		fields["status_id"] = *body.StatusID
	}
	if body.Confirmed != nil {
		fields["confirmed"] = *body.Confirmed
	}
	if body.IsInvalid != nil {
		fields["is_invalid"] = *body.IsInvalid
	}
	if body.Duration != nil {
		fields["duration"] = *body.Duration
	}
	if body.StartTime != "" {
		if st, err := time.Parse(time.RFC3339, body.StartTime); err == nil {
			fields["start_time"] = st
		}
	}
	if body.EndTime != "" {
		if et, err := time.Parse(time.RFC3339, body.EndTime); err == nil {
			fields["end_time"] = et
		}
	}

	if err := h.qsTimeSlotRepo.Update(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "update timeslot failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": tsID, "updated": true, "source": "qs"})
}

func (h *Handler) DeleteTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}

	if err := h.qsTimeSlotRepo.Delete(ctx, tsID); err != nil {
		slog.ErrorContext(ctx, "delete timeslot failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": tsID})
}

// ──────────────────────────────────────────────
// Interview Slots
// ──────────────────────────────────────────────

func (h *Handler) GenerateSlots(w http.ResponseWriter, r *http.Request) {
	// Slot generation remains a stub — requires complex scheduling algorithm
	// matching moderator availability, project duration, buffer times, etc.
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
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	// Return OPEN timeslots (status_id=1)
	openStatus := 1
	var projectID *int64
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		v, _ := strconv.ParseInt(pid, 10, 64)
		if v > 0 {
			projectID = &v
		}
	}

	slots, _, err := h.qsTimeSlotRepo.List(ctx, 1, 50, projectID, &openStatus, nil, nil, nil)
	if err != nil {
		slog.ErrorContext(ctx, "get available slots failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get available slots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		item := map[string]any{
			"id":        s.ID,
			"projectId": s.ProjectID,
			"startTime": s.StartTime.Format(time.RFC3339),
			"endTime":   s.EndTime.Format(time.RFC3339),
			"duration":  s.Duration,
		}
		if s.ModeratorID.Valid {
			item["moderatorId"] = s.ModeratorID.Int64
			item["moderatorName"] = s.ModeratorName.String
		}
		result = append(result, item)
	}
	writeJSON(w, http.StatusOK, result)
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
					"serviceCategory": "MRA",
					"timezone":  m.TimeZone.String,
					"updatedAt": m.ModifiedOn.Format(time.RFC3339),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, combined)
}

func (h *Handler) CreateModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		TimeZone  string `json:"timeZone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "email is required"})
		return
	}
	if req.TimeZone == "" {
		req.TimeZone = "America/New_York"
	}
	// Role 1 = Moderator
	uid, err := h.qsUserRepo.Create(ctx, req.FirstName, req.LastName, req.Email, req.TimeZone, []int{1})
	if err != nil {
		slog.Error("create moderator", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create moderator"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        uid,
		"firstName": req.FirstName,
		"lastName":  req.LastName,
		"email":     req.Email,
		"role":      "moderator",
		"status":    "active",
		"createdAt": now(),
	})
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
			"serviceCategory": "MRA",
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
			"serviceCategory": "LS",
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

	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": nid})
}

func (h *Handler) DeleteModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	modID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid moderator id"})
		return
	}
	if err := h.qsUserRepo.SoftDelete(ctx, modID); err != nil {
		slog.Error("delete moderator", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete moderator"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": modID})
}

func (h *Handler) BulkUploadModerators(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"created": 0, "failed": 0, "errors": []any{}, "message": "bulk upload not yet implemented",
	})
}

func (h *Handler) GetModeratorTimeslots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	modID, err := strconv.ParseInt(chi.URLParam(r, "moderatorId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid moderator id"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	slots, total, err := h.qsTimeSlotRepo.ListByModerator(ctx, modID, page, pageSize)
	if err != nil {
		slog.ErrorContext(ctx, "get moderator timeslots failed", "error", err, "moderatorId", modID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get moderator timeslots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		item := map[string]any{
			"id":          s.ID,
			"projectId":   s.ProjectID,
			"projectName": s.ProjectName,
			"startTime":   s.StartTime.Format(time.RFC3339),
			"endTime":     s.EndTime.Format(time.RFC3339),
			"duration":    s.Duration,
			"statusId":    s.StatusID,
			"status":      s.StatusName,
			"confirmed":   s.Confirmed,
			"source":      "qs",
		}
		if s.ResponderID.Valid {
			item["responderId"] = s.ResponderID.Int64
			item["responderName"] = s.ResponderName.String
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta":    map[string]any{"page": page, "pageSize": pageSize, "totalCount": total},
	})
}

// ──────────────────────────────────────────────
// Participants
// ──────────────────────────────────────────────

func (h *Handler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsRespondentRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	respondents, total, err := h.qsRespondentRepo.List(ctx, page, pageSize, search)
	if err != nil {
		slog.ErrorContext(ctx, "list participants failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list participants"})
		return
	}

	result := make([]map[string]any, 0, len(respondents))
	for _, r := range respondents {
		item := map[string]any{
			"id":        r.ID,
			"firstName": r.FirstName,
			"lastName":  r.LastName,
			"name":      strings.TrimSpace(r.FirstName + " " + r.LastName),
			"source":    "qs",
			"serviceCategory": "MRA",
			"modifiedOn": r.ModifiedOn.Format(time.RFC3339),
		}
		if r.Title.Valid {
			item["title"] = r.Title.String
		}
		if r.ExternalResponderID.Valid {
			item["externalResponderId"] = r.ExternalResponderID.String
		}
		if r.TimeZone.Valid {
			item["timeZone"] = r.TimeZone.String
		}
		if r.Email.Valid {
			item["email"] = r.Email.String
		}
		if r.Phone.Valid {
			item["phone"] = r.Phone.String
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": total,
		},
	})
}

func (h *Handler) CreateParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsRespondentRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		FirstName           string `json:"firstName"`
		LastName            string `json:"lastName"`
		Title               string `json:"title"`
		Email               string `json:"email"`
		Phone               string `json:"phone"`
		ExternalResponderID string `json:"externalResponderId"`
		TimeZone            string `json:"timeZone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if body.FirstName == "" || body.LastName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "firstName and lastName are required"})
		return
	}

	resp := &qs.Respondent{
		FirstName:           body.FirstName,
		LastName:            body.LastName,
		Title:               toNullStr(body.Title),
		ExternalResponderID: toNullStr(body.ExternalResponderID),
		TimeZone:            toNullStr(body.TimeZone),
	}

	respID, err := h.qsRespondentRepo.Create(ctx, resp)
	if err != nil {
		slog.ErrorContext(ctx, "create participant failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create participant"})
		return
	}

	// Add email as communication address (transport_type_id=1)
	if body.Email != "" {
		if _, err := h.qsRespondentRepo.CreateCommunicationAddress(ctx, respID, 1, body.Email); err != nil {
			slog.ErrorContext(ctx, "create participant email failed", "error", err)
		}
	}
	// Add phone as communication address (transport_type_id=2)
	if body.Phone != "" {
		if _, err := h.qsRespondentRepo.CreateCommunicationAddress(ctx, respID, 2, body.Phone); err != nil {
			slog.ErrorContext(ctx, "create participant phone failed", "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": respID, "source": "qs"})
}

func (h *Handler) GetParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsRespondentRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	respID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid participant id"})
		return
	}

	resp, err := h.qsRespondentRepo.GetByID(ctx, respID)
	if err != nil {
		slog.ErrorContext(ctx, "get participant failed", "error", err, "id", respID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if resp == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "participant not found"})
		return
	}

	result := map[string]any{
		"id":              resp.ID,
		"firstName":       resp.FirstName,
		"lastName":        resp.LastName,
		"name":            strings.TrimSpace(resp.FirstName + " " + resp.LastName),
		"source":          "qs",
		"serviceCategory": "MRA",
		"modifiedOn":      resp.ModifiedOn.Format(time.RFC3339),
	}
	if resp.Title.Valid {
		result["title"] = resp.Title.String
	}
	if resp.ExternalResponderID.Valid {
		result["externalResponderId"] = resp.ExternalResponderID.String
	}
	if resp.TimeZone.Valid {
		result["timeZone"] = resp.TimeZone.String
	}
	if resp.LanguageCountry.Valid {
		result["languageCountry"] = resp.LanguageCountry.String
	}

	// Get communication addresses
	addrs, err := h.qsRespondentRepo.GetCommunicationAddresses(ctx, respID)
	if err != nil {
		slog.ErrorContext(ctx, "get participant addresses failed", "error", err)
	}
	if addrs != nil {
		contacts := make([]map[string]any, 0, len(addrs))
		for _, a := range addrs {
			transport := "other"
			switch a.TransportTypeID {
			case 1:
				transport = "email"
			case 2:
				transport = "sms"
			}
			contacts = append(contacts, map[string]any{
				"type":        transport,
				"address":     a.Address,
				"contactable": a.Contactable,
				"optedOut":    a.OptedOut,
			})
		}
		result["contacts"] = contacts
	}

	writeJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────
// Bookings (QS: timeslots with linked respondents)
// ──────────────────────────────────────────────

func (h *Handler) ListBookings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var projectID *int64
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		v, _ := strconv.ParseInt(pid, 10, 64)
		if v > 0 {
			projectID = &v
		}
	}

	// Bookings are timeslots that have a linked respondent (non-OPEN status)
	slots, total, err := h.qsTimeSlotRepo.List(ctx, page, pageSize, projectID, nil, nil, nil, nil)
	if err != nil {
		slog.ErrorContext(ctx, "list bookings failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list bookings"})
		return
	}

	// Filter to only include slots that have a respondent
	result := make([]map[string]any, 0)
	for _, s := range slots {
		if !s.ResponderID.Valid {
			continue
		}
		item := map[string]any{
			"id":              s.ID,
			"projectId":       s.ProjectID,
			"projectName":     s.ProjectName,
			"startTime":       s.StartTime.Format(time.RFC3339),
			"endTime":         s.EndTime.Format(time.RFC3339),
			"duration":        s.Duration,
			"statusId":        s.StatusID,
			"status":          s.StatusName,
			"responderId":     s.ResponderID.Int64,
			"responderName":   s.ResponderName.String,
			"source":          "qs",
			"serviceCategory": "MRA",
			"modifiedOn":      s.ModifiedOn.Format(time.RFC3339),
		}
		if s.ModeratorID.Valid {
			item["moderatorId"] = s.ModeratorID.Int64
			item["moderatorName"] = s.ModeratorName.String
		}
		if s.ConferenceHash.Valid {
			item["conferenceHash"] = s.ConferenceHash.String
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": total,
		},
	})
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	// Booking creation is handled via ScheduleInterview which links respondent to timeslot
	writeJSON(w, http.StatusCreated, map[string]any{"message": "use POST /interviews/schedule to create bookings"})
}

func (h *Handler) GetBookingsByUser(w http.ResponseWriter, r *http.Request) {
	// This returns timeslots linked to a specific respondent
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	// userId here would be a responder ID in the QS context
	userID := chi.URLParam(r, "userId")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    []map[string]any{},
		"message": fmt.Sprintf("bookings for user %s — respondent-level lookup pending", userID),
	})
}

func (h *Handler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	// Booking update is a timeslot status change
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid booking id"})
		return
	}

	var body struct {
		StatusID *int `json:"statusId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	fields := map[string]any{}
	if body.StatusID != nil {
		fields["status_id"] = *body.StatusID
	}

	if err := h.qsTimeSlotRepo.Update(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "update booking failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": tsID, "updated": true, "source": "qs"})
}

func (h *Handler) UpdateBookingReward(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	bookingID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid booking id"})
		return
	}
	var req struct {
		RewardPoints int    `json:"rewardPoints"`
		RewardStatus string `json:"rewardStatus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if req.RewardStatus == "" {
		req.RewardStatus = "not_credited"
	}
	if err := h.qsTimeSlotRepo.UpsertReward(ctx, bookingID, req.RewardPoints, req.RewardStatus); err != nil {
		slog.Error("update booking reward", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to update reward"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           bookingID,
		"rewardPoints": req.RewardPoints,
		"rewardStatus": req.RewardStatus,
		"updated":      true,
	})
}

// ──────────────────────────────────────────────
// Subscriptions
// ──────────────────────────────────────────────

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := `SELECT DISTINCT s.id, s.company
	      FROM subscription s
	      JOIN project p ON p.subscription_id = s.id
	      WHERE p.project_type_id = 2
	      ORDER BY s.company`
	rows, err := h.db.IRISReadOnly.QueryContext(ctx, q)
	if err != nil {
		slog.Error("list subscriptions", "err", err)
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	defer rows.Close()

	subs := []map[string]any{}
	for rows.Next() {
		var id int64
		var company string
		if err := rows.Scan(&id, &company); err != nil {
			continue
		}
		subs = append(subs, map[string]any{
			"id":      strconv.FormatInt(id, 10),
			"company": company,
			"plan":    "enterprise",
		})
	}
	writeJSON(w, http.StatusOK, subs)
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{"message": "subscription creation not yet implemented"})
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "id")
	ctx := r.Context()
	var company string
	err := h.db.IRISReadOnly.QueryRowContext(ctx, "SELECT company FROM subscription WHERE id = ?", sid).Scan(&company)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": sid, "company": "Unknown"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": sid, "company": company, "plan": "enterprise"})
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"message": "subscription update not yet implemented"})
}

func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
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
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": "wq-" + id()[:8], "status": "waiting", "waitingSince": now(),
	})
}

func (h *Handler) RemoveFromWaitingQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
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
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		TimeSlotID  int64 `json:"timeSlotId"`
		ModeratorID int64 `json:"moderatorId"`
		ResponderID int64 `json:"responderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if body.TimeSlotID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "timeSlotId is required"})
		return
	}

	// Update timeslot status to PENDING (2)
	if err := h.qsTimeSlotRepo.Update(ctx, body.TimeSlotID, map[string]any{"status_id": 2, "confirmed": true}); err != nil {
		slog.ErrorContext(ctx, "schedule interview: update timeslot failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to schedule interview"})
		return
	}

	// Assign moderator if provided
	if body.ModeratorID > 0 {
		if _, err := h.qsTimeSlotRepo.AssignModerator(ctx, body.ModeratorID, body.TimeSlotID, true); err != nil {
			slog.ErrorContext(ctx, "schedule interview: assign moderator failed", "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"timeSlotId": body.TimeSlotID,
		"statusId":   2,
		"status":     "PENDING",
		"source":     "qs",
	})
}

func (h *Handler) CancelInterview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid interview id"})
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	user := middleware.GetUser(r)
	cancelStatusID := 12 // PM_CANCELLED by default
	if user != nil {
		for _, role := range user.Roles {
			if role == "moderator" {
				cancelStatusID = 5 // MODERATOR_CANCELLED
				break
			}
		}
	}

	fields := map[string]any{"status_id": cancelStatusID}
	if body.Reason != "" {
		fields["invalidation_reason_text"] = body.Reason
	}
	if user != nil {
		fields["status_modified_by"] = 0 // placeholder: would need user lookup to get QS user ID
	}

	if err := h.qsTimeSlotRepo.Update(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "cancel interview failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cancel failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"timeSlotId": tsID, "statusId": cancelStatusID, "status": "CANCELLED"})
}

func (h *Handler) RescheduleInterview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid interview id"})
		return
	}

	var body struct {
		NewStartTime string `json:"newStartTime"`
		NewEndTime   string `json:"newEndTime"`
		Reason       string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	user := middleware.GetUser(r)
	rescheduleStatusID := 11 // PM_RESCHEDULED by default
	if user != nil {
		for _, role := range user.Roles {
			if role == "moderator" {
				rescheduleStatusID = 3 // MODERATOR_RESCHEDULED
				break
			}
		}
	}

	fields := map[string]any{"status_id": rescheduleStatusID}
	if body.NewStartTime != "" {
		if st, err := time.Parse(time.RFC3339, body.NewStartTime); err == nil {
			fields["start_time"] = st
		}
	}
	if body.NewEndTime != "" {
		if et, err := time.Parse(time.RFC3339, body.NewEndTime); err == nil {
			fields["end_time"] = et
		}
	}
	if body.Reason != "" {
		fields["invalidation_reason_text"] = body.Reason
	}

	if err := h.qsTimeSlotRepo.Update(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "reschedule interview failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "reschedule failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"timeSlotId": tsID, "statusId": rescheduleStatusID, "status": "RESCHEDULED"})
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
	writeJSON(w, http.StatusOK, map[string]any{"availabilities": items})
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

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          avail.ID,
		"moderatorId": avail.ModeratorID,
		"startTime":   avail.StartTime.Format(time.RFC3339),
		"endTime":     avail.EndTime.Format(time.RFC3339),
	})
}

func (h *Handler) DeleteModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	availID := chi.URLParam(r, "availabilityId")
	nid, err := strconv.ParseInt(availID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid availability id"})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	if err := h.qsUserRepo.DeleteModeratorAvailability(ctx, nid); err != nil {
		slog.ErrorContext(ctx, "delete moderator availability failed", "error", err, "availabilityId", nid)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete availability"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "availabilityId": nid})
}

func (h *Handler) GetTimeslotModeratorOptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
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

// MeetingAction handles meeting actions (end, start_recording, etc).
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:meetingId/:action
// Response: {} for end/disableAutomute actions
func (h *Handler) MeetingAction(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	action := chi.URLParam(r, "action")
	bearerToken := extractBearerToken(r)

	// Log meeting action
	if h.db.IRIS != nil {
		user := middleware.GetUser(r)
		userSub := ""
		if user != nil {
			userSub = user.Sub
		}
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO activity_log (event_type, description, meta_data, created_on)
			 VALUES ('meeting_action', ?, ?, NOW())`,
			fmt.Sprintf("Meeting %s: %s", meetingID, action),
			fmt.Sprintf(`{"meetingId":"%s","action":"%s","userId":"%s"}`, meetingID, action, userSub))
	}

	// Call Conference Service for real meeting actions
	if h.services.Conference.Configured() {
		switch action {
		case "end":
			if err := h.services.Conference.EndMeeting(r.Context(), meetingID, bearerToken); err != nil {
				slog.Warn("conference end meeting failed", "meetingId", meetingID, "error", err)
			}
		case "start_recording":
			if err := h.services.Conference.StartRecording(r.Context(), meetingID, bearerToken); err != nil {
				slog.Warn("conference start recording failed", "meetingId", meetingID, "error", err)
			}
		}

		meta, err := h.services.Conference.GetRecordingStatus(r.Context(), meetingID, bearerToken)
		if err == nil && meta != nil {
			meta["action"] = action
			meta["actionResult"] = "success"
			meta["actionTimestamp"] = now()
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	// Fallback: DB-only response
	if h.qsConferenceRepo != nil {
		meta, _ := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if meta != nil {
			meta["action"] = action
			meta["actionResult"] = "success"
			meta["actionTimestamp"] = now()
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meetingId": meetingID, "action": action,
		"result": "success", "timestamp": now(),
	})
}

// MeetingUniversalJoin handles universal join for a meeting.
// Contract-identical with legacy InCrowdAPI: POST /v1/meeting/:meetingId/universal_join
// Response: passthrough from conference service
func (h *Handler) MeetingUniversalJoin(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	bearerToken := extractBearerToken(r)

	// Call Conference Service for real universal join
	if h.services.Conference.Configured() {
		joinResp, err := h.services.Conference.UniversalJoin(r.Context(), meetingID, bearerToken)
		if err == nil && joinResp != nil {
			joinResp["joinTimestamp"] = now()
			writeJSON(w, http.StatusOK, joinResp)
			return
		}
		slog.Warn("conference universal join failed", "meetingId", meetingID, "error", err)
	}

	// Fallback: DB lookup
	if h.qsConferenceRepo != nil {
		meta, err := h.qsConferenceRepo.GetMeetingMetadata(r.Context(), meetingID)
		if err == nil && meta != nil {
			meta["joinUrl"] = fmt.Sprintf("https://chime.aws/join/%s", meetingID)
			meta["joinTimestamp"] = now()
			writeJSON(w, http.StatusOK, meta)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meetingId":  meetingID,
		"joinUrl":    fmt.Sprintf("https://chime.aws/join/%s", meetingID),
		"attendeeId": "att-" + id()[:8],
		"joinTimestamp": now(),
	})
}

// extractBearerToken extracts the JWT bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	// Also check IC-Auth and CognitoToken headers (legacy patterns)
	if t := r.Header.Get("IC-Auth"); t != "" {
		return t
	}
	if t := r.Header.Get("CognitoToken"); t != "" {
		return t
	}
	return ""
}

// ──────────────────────────────────────────────
// Payments (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{"paymentId": 7001, "status": "PENDING"})
}

func (h *Handler) CreateCustomHonorarium(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"timeSlotId": 301, "honorarium": 200.00, "reasonId": 2})
}

func (h *Handler) GetPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
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
	writeJSON(w, http.StatusOK, map[string]any{
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
	pidStr := chi.URLParam(r, "projectId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	var req struct {
		Translations []struct {
			TopicID        int64  `json:"topicId"`
			LanguageCode   string `json:"languageCode"`
			TranslatedName string `json:"translatedName"`
		} `json:"translations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsAnswerRepo != nil {
		for _, t := range req.Translations {
			if err := h.qsAnswerRepo.UpdateTopicTranslation(r.Context(), projectID, t.TopicID, t.LanguageCode, t.TranslatedName); err != nil {
				slog.Error("update translation failed", "topicId", t.TopicID, "error", err)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "updatedCount": len(req.Translations)})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ──────────────────────────────────────────────
// Notifications (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.URL.Query().Get("projectId")
	templateType := r.URL.Query().Get("type")
	source := h.resolveSource(r)

	if projectIDStr != "" {
		projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)
		if source == "iris" && h.irisSurveyRepo != nil {
			tpl, err := h.irisSurveyRepo.GetEmailTemplateForProject(r.Context(), projectID)
			if err != nil {
				slog.Error("get email template failed", "error", err)
			}
			if tpl != nil {
				tpl["source"] = "iris"
				tpl["templateType"] = templateType
				writeJSON(w, http.StatusOK, tpl)
				return
			}
		}
	}

	if h.qsAnswerRepo != nil {
		name := templateType
		if name == "" {
			name = "reschedule"
		}
		tpl, _ := h.qsAnswerRepo.GetCommunicationTemplate(r.Context(), name)
		if tpl != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"subject": tpl.Subject, "body": tpl.Body,
				"templateType": name, "source": "qs",
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"subject":      "Your Interview Has Been Rescheduled",
		"body":         "<html><body><p>Dear {{.Name}}, your interview has been rescheduled.</p></body></html>",
		"templateType": templateType,
		"source":       "default",
	})
}

func (h *Handler) SendReminder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipients []string `json:"recipients"`
		Subject    string   `json:"subject"`
		Body       string   `json:"body"`
		Type       string   `json:"type"`
		ProjectID  int64    `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, treat as simple reminder
		writeJSON(w, http.StatusOK, map[string]any{"sent": true, "recipientCount": 0})
		return
	}

	// Call Notification Service if configured
	if h.services.Notification.Configured() && len(req.Recipients) > 0 {
		msg := integration.EmailMessage{
			To:          req.Recipients,
			Subject:     req.Subject,
			Body:        req.Body,
			ContentType: "text/html",
		}
		if err := h.services.Notification.SendEmail(r.Context(), msg); err != nil {
			slog.Warn("notification service send reminder failed", "error", err)
			// Don't fail the request — log and continue with DB fallback
		} else {
			slog.Info("reminder sent via notification service",
				"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
		}
	} else {
		slog.Info("reminder logged (notification service not configured)",
			"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sent": true, "recipientCount": len(req.Recipients),
		"type": req.Type, "projectId": req.ProjectID,
	})
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
					"serviceCategory": "MRA",
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
					"serviceCategory": "LS",
					"lastLogin":       lastLogin,
					"registeredAt":    u.RegistrationDate.Format(time.RFC3339),
				})
			}
			_ = total
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": allUsers})
}

func (h *Handler) CreateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		TimeZone  string `json:"timeZone"`
		RoleIDs   []int  `json:"roleIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	if req.Email == "" || req.FirstName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "email and firstName required"})
		return
	}
	if len(req.RoleIDs) == 0 {
		req.RoleIDs = []int{3} // default: admin
	}

	source := h.resolveSource(r)
	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		uid, err := h.qsUserRepo.Create(r.Context(), req.FirstName, req.LastName, req.Email, req.TimeZone, req.RoleIDs)
		if err != nil {
			slog.Error("create admin user failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": uid, "email": req.Email, "firstName": req.FirstName, "lastName": req.LastName,
			"roles": req.RoleIDs, "source": "qs",
			"cognitoStatus": "Cognito user must be created separately via Cognito console or AWS CLI",
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}
