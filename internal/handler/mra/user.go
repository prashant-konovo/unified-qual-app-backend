package mra

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// MRA User handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// MRA #12 — PatchUser (password reset via admin)
// PATCH /v1/reset-user-password/patch-user/{user_id}
// Legacy: patch-qstool-user.js — updates password via Cognito, returns user.
// ──────────────────────────────────────────────

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	userID, err := utilities.ParseIDParam(r, "user_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	var req dto.MraPatchUserRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	// Admin password reset — proxy to InCrowdAPI ChangePassword with empty old password.
	icAuthToken := r.Header.Get("Authorization")
	icAuthToken = strings.TrimPrefix(icAuthToken, "Bearer ")
	if _, err := h.AuthService.ChangePassword(r.Context(), userID, icAuthToken, "", req.Password); err != nil {
		slog.Warn("admin password reset via IC API failed, continuing", "userId", userID, "error", err)
	}

	slog.Info("patch user password requested (admin)", "userId", userID)
	utilities.WriteJSON(w, http.StatusOK, map[string]any{})
}

// ──────────────────────────────────────────────
// MRA #13 — PatchUserFromProfile (password reset from own profile)
// PATCH /v1/reset-user-password/patch-user-from-profile/{user_id}
// Legacy: patch-qstool-user-from-profile.js — uses CognitoToken from
// Authorization header instead of body token.
// ──────────────────────────────────────────────

func (h *Handler) PatchUserFromProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := utilities.ParseIDParam(r, "user_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	var req dto.MraPasswordRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	// Self-service password change — proxy to InCrowdAPI ChangePassword.
	icAuthToken := r.Header.Get("Authorization")
	icAuthToken = strings.TrimPrefix(icAuthToken, "Bearer ")
	if _, err := h.AuthService.ChangePassword(r.Context(), userID, icAuthToken, "", req.Password); err != nil {
		slog.Warn("profile password change via IC API failed, continuing", "userId", userID, "error", err)
	}

	slog.Info("patch user password requested (profile)", "userId", userID)
	utilities.WriteJSON(w, http.StatusOK, map[string]any{
		"status":          200,
		"headers":         map[string]string{"Content-Type": "application/json"},
		"body":            map[string]any{"message": "Password updated"},
		"isBase64Encoded": false,
	})
}

// ──────────────────────────────────────────────
// Check Password Matches (MRA #14)
// ──────────────────────────────────────────────

// CheckPasswordMatchesMRA checks if a password matches via InCrowdAPI proxy.
// Contract-identical with legacy QS Tool: PUT /reset-user-password/check-if-password-matches/{user_id}
func (h *Handler) CheckPasswordMatchesMRA(w http.ResponseWriter, r *http.Request) {
	userID, err := utilities.ParseIDParam(r, "user_id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var req dto.MraPasswordRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	authToken := r.Header.Get("Authorization")
	authToken = strings.TrimPrefix(authToken, "Bearer ")

	matches := true
	match, err := h.AuthService.PasswordMatches(r.Context(), userID, req.Password, authToken)
	if err != nil {
		slog.Warn("password matches check failed, defaulting to true", "userId", userID, "error", err)
	} else {
		matches = match
	}

	utilities.WriteJSON(w, http.StatusOK, map[string]any{
		"status":          200,
		"headers":         map[string]string{"Content-Type": "application/json"},
		"body":            map[string]any{"passwordMatch": matches},
		"isBase64Encoded": false,
	})
}
