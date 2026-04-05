package ls

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// LS User handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Password Management (MRA #10, #11)
// ──────────────────────────────────────────────

func (h *Handler) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Trigger Cognito ForgotPassword flow
	if h.UserService.CognitoRegion() != "" && h.UserService.CognitoAppClientID() != "" {
		endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", h.UserService.CognitoRegion())
		payload := map[string]any{
			"ClientId": h.UserService.CognitoAppClientID(),
			"Username": req.Email,
		}
		payloadBytes, _ := json.Marshal(payload)
		cogReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint,
			strings.NewReader(string(payloadBytes)))
		if err == nil {
			cogReq.Header.Set("Content-Type", "application/x-amz-json-1.1")
			cogReq.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService.ForgotPassword")
			resp, err := http.DefaultClient.Do(cogReq)
			if err != nil {
				slog.Warn("cognito forgot password call failed", "error", err)
			} else {
				resp.Body.Close()
				slog.Info("cognito forgot password triggered", "email", req.Email, "status", resp.StatusCode)
			}
		}
	}

	// Return legacy Lambda proxy result shape
	slog.Info("password reset requested", "email", req.Email)
	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"status":          200,
		"headers":         map[string]string{"Content-Type": "application/json"},
		"body":            map[string]any{"message": "Password reset email sent"},
		"isBase64Encoded": false,
	})
}

func (h *Handler) CheckUserIsQsToolAndI2(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	if h.UserService.QsAvailable() {
		result, err := h.UserService.CheckUserIsQsToolAndI2(r.Context(), req.Email)
		if err != nil {
			slog.Error("check qs/i2 failed", "error", err)
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": err.Error(),
			})
			return
		}
		// Return legacy Lambda proxy result shape
		dto.WriteJSON(w, http.StatusOK, map[string]any{
			"status":          200,
			"headers":         map[string]string{"Content-Type": "application/json"},
			"body":            result,
			"isBase64Encoded": false,
		})
		return
	}
	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"status":          200,
		"headers":         map[string]string{"Content-Type": "application/json"},
		"body":            map[string]any{"isQsTool": false, "isI2": false, "exists": false},
		"isBase64Encoded": false,
	})
}

func (h *Handler) CheckUserCommPreference(w http.ResponseWriter, r *http.Request) {
	userID, err := dto.ParseIDParam(r, "userId")
	if err != nil {
		dto.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !h.UserService.QsAvailable() {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "no database available",
		})
		return
	}

	// Legacy: SELECT allow_contact_by_email FROM user_communication_preferences WHERE user_id = :userId
	pref, err := h.UserService.GetUserCommPreference(r.Context(), userID)
	if err != nil {
		slog.Error("get comm pref failed", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy returns {data: records} where records = [{allow_contact_by_email: 0/1}]
	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"data": pref,
	})
}

func (h *Handler) UnsubscribeUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userId")

	var req struct {
		PmUserID            string `json:"pmUserId"`
		AllowContactByEmail int    `json:"allowContactByEmail"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	// Legacy: if pmUserId is empty, default to userId
	if req.PmUserID == "" {
		req.PmUserID = userIDStr
	}

	if !h.UserService.QsAvailable() {
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "no database available",
		})
		return
	}

	// Legacy: 4-query transaction on user_communication_preferences using cognito_user_id
	result, err := h.UserService.UpdateUserCommPreference(r.Context(), userIDStr, req.PmUserID, req.AllowContactByEmail)
	if err != nil {
		slog.Error("unsubscribe failed", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy returns full transaction result array
	dto.WriteJSON(w, http.StatusOK, result)
}
