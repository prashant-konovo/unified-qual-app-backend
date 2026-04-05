package shared

import (
"encoding/json"
"net/http"
"strings"

"github.com/InCrowd/unified-qual-api/internal/dto"
"github.com/InCrowd/unified-qual-api/internal/service"
"github.com/InCrowd/unified-qual-api/internal/middleware"
)

// AuthHandler handles authentication and authorization endpoints.
type AuthHandler struct{ *service.Deps }

// ──────────────────────────────────────────────
// Auth — delegates to AuthService for business logic
// ──────────────────────────────────────────────

func (h *AuthHandler) AuthLogin(w http.ResponseWriter, r *http.Request) {
var req dto.LoginRequest
if errs := dto.DecodeAndValidate(r, &req); errs != nil {
dto.WriteError(w, errs)
return
}

result := h.AuthService.Login(r.Context(), req)

if result.NeedsTerms {
dto.WriteJSON(w, 203, map[string]any{
"termsAcceptedRes": map[string]any{
"termsAccepted": false,
"userId":        result.TermsUserID,
},
})
return
}

if result.RawBody != nil {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(result.RawStatus)
_, _ = w.Write(result.RawBody)
return
}

if result.ErrorStatus > 0 {
dto.WriteJSON(w, result.ErrorStatus, result.ErrorBody)
return
}

dto.WriteJSON(w, http.StatusOK, result.Response)
}

func (h *AuthHandler) AuthMagicLink(w http.ResponseWriter, r *http.Request) {
dto.WriteJSON(w, http.StatusNotImplemented, map[string]any{
"error": "magic link authentication not yet implemented",
})
}

func (h *AuthHandler) AuthRefresh(w http.ResponseWriter, r *http.Request) {
var req dto.RefreshRequest
if errs := dto.DecodeAndValidate(r, &req); errs != nil {
dto.WriteError(w, errs)
return
}

result := h.AuthService.Refresh(r.Context(), req)

if result.Error != nil {
status := http.StatusBadRequest
if strings.Contains(result.Error.Error(), "token refresh error") {
status = http.StatusBadGateway
}
dto.WriteJSON(w, status, map[string]any{"error": result.Error.Error()})
return
}

if result.Success {
dto.WriteJSON(w, http.StatusOK, result.Response)
return
}

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(result.RawStatus)
_, _ = w.Write(result.RawBody)
}

func (h *AuthHandler) AuthPassword(w http.ResponseWriter, r *http.Request) {
var req dto.ChangePasswordRequest
if errs := dto.DecodeAndValidate(r, &req); errs != nil {
dto.WriteError(w, errs)
return
}

user := middleware.GetUser(r)

newPwd := req.NewPassword
if newPwd == "" {
newPwd = req.Password
}
if newPwd == "" {
dto.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "newPassword is required"})
return
}

// Derive user ID
targetUserID := req.UserID
if targetUserID == 0 && user != nil && user.Email != "" {
targetUserID = h.AuthService.ResolveUserID(r.Context(), user.Email)
}

// Extract IC-Auth token
icAuthToken := ""
if icAuth := r.Header.Get("IC-Auth"); icAuth != "" {
if parts := strings.SplitN(icAuth, ":", 2); len(parts) == 2 {
icAuthToken = parts[1]
}
}

result, err := h.AuthService.ChangePassword(r.Context(), targetUserID, icAuthToken, req.OldPassword, newPwd)
if err != nil {
status := http.StatusInternalServerError
if strings.Contains(err.Error(), "unavailable") {
status = http.StatusServiceUnavailable
}
dto.WriteJSON(w, status, map[string]any{"error": err.Error()})
return
}

dto.WriteJSON(w, http.StatusOK, result)
}

// AuthMe returns the authenticated user's claims from the JWT.
func (h *AuthHandler) AuthMe(w http.ResponseWriter, r *http.Request) {
user := middleware.GetUser(r)
if user == nil {
dto.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "not authenticated"})
return
}
dto.WriteJSON(w, http.StatusOK, map[string]any{
"sub":      user.Sub,
"email":    user.Email,
"username": user.Username,
"groups":   user.Groups,
"roles":    user.Roles,
})
}

// AuthLogout revokes the user's session via AuthService.
func (h *AuthHandler) AuthLogout(w http.ResponseWriter, r *http.Request) {
var req dto.LogoutRequest
_ = json.NewDecoder(r.Body).Decode(&req)

h.AuthService.Logout(r.Context(), req.ICUserID, req.ICAuthToken)
dto.WriteJSON(w, http.StatusOK, map[string]any{"message": "logged out"})
}

// AuthAcceptTerms proxies terms acceptance to AuthService.
func (h *AuthHandler) AuthAcceptTerms(w http.ResponseWriter, r *http.Request) {
var req dto.AcceptTermsRequest
if errs := dto.DecodeAndValidate(r, &req); errs != nil {
dto.WriteError(w, errs)
return
}

h.AuthService.AcceptTerms(r.Context(), req.UserID)
dto.WriteJSON(w, http.StatusOK, map[string]any{"message": "terms accepted"})
}

// ──────────────────────────────────────────────
// SSO — delegates to AuthService
// ──────────────────────────────────────────────

func (h *AuthHandler) AuthSSOConfig(w http.ResponseWriter, r *http.Request) {
override := r.URL.Query().Get("redirectUri")
ssoConfig, err := h.AuthService.GetSSOConfig(override)
if err != nil {
dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
return
}

dto.WriteJSON(w, http.StatusOK, map[string]any{
"data": map[string]any{
"authorizeUrl": ssoConfig.AuthorizeURL,
"redirectUri":  ssoConfig.RedirectURI,
},
})
}

func (h *AuthHandler) AuthSSOCallback(w http.ResponseWriter, r *http.Request) {
var req dto.SSOCallbackRequest
if errs := dto.DecodeAndValidate(r, &req); errs != nil {
dto.WriteError(w, errs)
return
}

tokens, err := h.AuthService.ExchangeSSOCode(r.Context(), req.Code, req.RedirectURI)
if err != nil {
dto.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "SSO authentication failed"})
return
}

dto.WriteJSON(w, http.StatusOK, map[string]any{
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
