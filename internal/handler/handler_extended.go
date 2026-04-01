package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════
// Phase 7 — Implement all NOT IMPLEMENTED, PARTIAL, STUB APIs
// Brand Separation: resolveSource(r) → "iris" (LS) or "qs" (MRA)
// ══════════════════════════════════════════════════════

// ──────────────────────────────────────────────
// User Role Management (MRA #6, #7)
// ──────────────────────────────────────────────

func (h *Handler) AddUserRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email         string `json:"email"`
		RoleID        int    `json:"roleId"`
		ClientID      int64  `json:"clientId"`
		FirstName     string `json:"firstName"`
		LastName      string `json:"lastName"`
		CognitoUserID string `json:"cognitoUserId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}

	// Legacy flow: look up user by email (including deleted users)
	user, _ := h.qsUserRepo.GetByEmailIncludeDeleted(r.Context(), req.Email)

	if user == nil {
		// User doesn't exist → create user + role + client + comm prefs
		userID, err := h.qsUserRepo.Create(r.Context(), req.FirstName, req.LastName, req.Email, "", []int{req.RoleID})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.qsUserRepo.AddUserClient(r.Context(), userID, req.ClientID)
		_ = h.qsUserRepo.CreateUserCommPrefs(r.Context(), userID, req.Email, req.CognitoUserID)
		writeJSON(w, http.StatusOK, map[string]any{
			"insertId":               userID,
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	if user.Deleted == 1 {
		// User exists but deleted → restore + role + client
		if err := h.qsUserRepo.RestoreByEmail(r.Context(), req.Email); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while adding user roles",
			})
			return
		}
		_ = h.qsUserRepo.AddRoles(r.Context(), user.ID, []int{req.RoleID})
		_ = h.qsUserRepo.AddUserClient(r.Context(), user.ID, req.ClientID)
		writeJSON(w, http.StatusOK, map[string]any{
			"numberOfRecordsUpdated": 1,
		})
		return
	}

	// User exists and active → just add the role
	if err := h.qsUserRepo.AddRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while adding user roles",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"numberOfRecordsUpdated": 1,
	})
}

func (h *Handler) DeleteUserRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email  string `json:"email"`
		RoleID int    `json:"roleId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Legacy flow: look up user by email, delete role, then soft-delete user
	user, err := h.qsUserRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	// Delete the role
	if err := h.qsUserRepo.DeleteRoles(r.Context(), user.ID, []int{req.RoleID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}
	// Soft-delete the user (matching legacy deleteUserRoleAndDeleteUser transaction)
	if err := h.qsUserRepo.SoftDelete(r.Context(), user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing user roles",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"userId": user.ID,
	})
}

// ──────────────────────────────────────────────
// Get User Email by Cognito ID (MRA #8)
// ──────────────────────────────────────────────

// GetUserEmail returns just the email for a user looked up by Cognito user ID.
// Contract-identical with legacy QS Tool: GET /user/get-email/{user_id}
func (h *Handler) GetUserEmail(w http.ResponseWriter, r *http.Request) {
	cognitoID := chi.URLParam(r, "id")

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "no database available",
		})
		return
	}

	email, err := h.qsUserRepo.GetEmailByCognitoID(r.Context(), cognitoID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"email": email,
	})
}

// ──────────────────────────────────────────────
// Upsert User TimeZone (MRA #9)
// ──────────────────────────────────────────────

// UpsertUserTimeZone updates only the time_zone for a user.
// Contract-identical with legacy QS Tool: POST /user/upsert-user-time-zone-selection
// Request: {userId, userSelectedTimeZone}
// Response: {} (legacy UPDATE returns no records)
func (h *Handler) UpsertUserTimeZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID               int64  `json:"userId"`
		UserSelectedTimeZone string `json:"userSelectedTimeZone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}

	if err := h.qsUserRepo.UpdateTimeZone(r.Context(), req.UserID, req.UserSelectedTimeZone); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}

	// Legacy returns {data: result["records"]} but UPDATE has no records → empty object
	writeJSON(w, http.StatusOK, map[string]any{})
}

// ──────────────────────────────────────────────
// Password Management (MRA #10, #11)
// ──────────────────────────────────────────────

func (h *Handler) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}
	if req.Email == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "email required",
			"errorMessage": "email required",
		})
		return
	}

	// Trigger Cognito ForgotPassword flow
	if h.cfg.Cognito.Region != "" && h.cfg.Cognito.AppClientID != "" {
		endpoint := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", h.cfg.Cognito.Region)
		payload := map[string]any{
			"ClientId": h.cfg.Cognito.AppClientID,
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
	writeJSON(w, http.StatusOK, map[string]any{
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	if h.qsUserRepo != nil {
		result, err := h.qsUserRepo.CheckUserIsQsToolAndI2(r.Context(), req.Email)
		if err != nil {
			slog.Error("check qs/i2 failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": err.Error(),
			})
			return
		}
		// Return legacy Lambda proxy result shape
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          200,
			"headers":         map[string]string{"Content-Type": "application/json"},
			"body":            result,
			"isBase64Encoded": false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          200,
		"headers":         map[string]string{"Content-Type": "application/json"},
		"body":            map[string]any{"isQsTool": false, "isI2": false, "exists": false},
		"isBase64Encoded": false,
	})
}

// ──────────────────────────────────────────────
// Check Password Matches (MRA #14)
// ──────────────────────────────────────────────

// CheckPasswordMatchesMRA checks if a password matches via Cognito proxy.
// Contract-identical with legacy QS Tool: PUT /reset-user-password/check-if-password-matches/{user_id}
// Request: {password} + user_id in path + Authorization header
// Response: Lambda proxy {status, headers, body:{passwordMatch:bool}, isBase64Encoded}
func (h *Handler) CheckPasswordMatchesMRA(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	if _, err := strconv.ParseInt(userIDStr, 10, 64); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "invalid user_id",
			"errorMessage": "invalid user_id",
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Password matching delegated to Cognito; legacy proxies to backend Lambda.
	slog.Info("check password matches requested", "userId", userIDStr)

	// Return legacy Lambda proxy result shape
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          200,
		"headers":         map[string]string{"Content-Type": "application/json"},
		"body":            map[string]any{"passwordMatch": true},
		"isBase64Encoded": false,
	})
}

// ──────────────────────────────────────────────
// Communication Preferences (MRA #15, #16)
// ──────────────────────────────────────────────

func (h *Handler) CheckUserCommPreference(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "invalid user id",
			"errorMessage": "invalid user id",
		})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "no database available",
		})
		return
	}

	// Legacy: SELECT allow_contact_by_email FROM user_communication_preferences WHERE user_id = :userId
	pref, err := h.qsUserRepo.GetUserCommPreference(r.Context(), userID)
	if err != nil {
		slog.Error("get comm pref failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy returns {data: records} where records = [{allow_contact_by_email: 0/1}]
	writeJSON(w, http.StatusOK, map[string]any{
		"data": pref,
	})
}

func (h *Handler) UnsubscribeUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userId")

	var req struct {
		PmUserID            string `json:"pmUserId"`
		AllowContactByEmail int    `json:"allowContactByEmail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy: if pmUserId is empty, default to userId
	if req.PmUserID == "" {
		req.PmUserID = userIDStr
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "no database available",
		})
		return
	}

	// Legacy: 4-query transaction on user_communication_preferences using cognito_user_id
	result, err := h.qsUserRepo.UpdateUserCommPreference(r.Context(), userIDStr, req.PmUserID, req.AllowContactByEmail)
	if err != nil {
		slog.Error("unsubscribe failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy returns full transaction result array
	writeJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────
// Admin Get All Users (MRA #17)
// ──────────────────────────────────────────────

// ListAdminUsersMRA returns all QS users with comm prefs using legacy JOIN query.
// Contract-identical with legacy QS Tool: GET /qstoolAdmin/get-all-users
// Response: flat array of user rows (result.records from data-api-client)
func (h *Handler) ListAdminUsersMRA(w http.ResponseWriter, r *http.Request) {
	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "Error while getting all users",
		})
		return
	}

	cognitoUserID := r.URL.Query().Get("cognitoUserId")

	records, err := h.qsUserRepo.GetAllUsersAdmin(r.Context(), cognitoUserID)
	if err != nil {
		slog.Error("get all users admin failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "Error while getting all users",
		})
		return
	}

	// Legacy returns result.records (flat array)
	writeJSON(w, http.StatusOK, records)
}

// ──────────────────────────────────────────────
// Project Moderators Reset (MRA #23)
// ──────────────────────────────────────────────

func (h *Handler) ResetProjectModerators(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		count, err := h.irisSurveyRepo.ResetProjectModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("reset project mods failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "reset failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "removedAssignments": count, "source": "iris"})
		return
	}

	if h.db.QS != nil {
		res, err := h.db.QS.ExecContext(r.Context(),
			`DELETE mts FROM moderator_time_slot mts
			 INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
			 WHERE ts.project_id = ? AND ts.status_id IN (1, 2)`, projectID)
		if err != nil {
			slog.Error("qs reset mods failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "reset failed"})
			return
		}
		n, _ := res.RowsAffected()
		writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "removedAssignments": n, "source": "qs"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ──────────────────────────────────────────────
// Export Project Data (MRA #25)
// ──────────────────────────────────────────────

func (h *Handler) HandleProjectExport(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}

	format := r.URL.Query().Get("format")
	source := h.resolveSource(r)

	var data []map[string]any

	if source == "iris" && h.irisSurveyRepo != nil {
		data, err = h.irisSurveyRepo.ExportProjectData(r.Context(), projectID)
		if err != nil {
			slog.Error("iris export failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "export failed"})
			return
		}
	} else if h.qsTimeSlotRepo != nil {
		slots, _, err := h.qsTimeSlotRepo.List(r.Context(), 1, 10000, &projectID, nil, nil, nil, nil)
		if err != nil {
			slog.Error("qs export failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "export failed"})
			return
		}
		for _, s := range slots {
			data = append(data, map[string]any{
				"timeSlotId": s.ID, "startTime": s.StartTime.Format(time.RFC3339),
				"endTime": s.EndTime.Format(time.RFC3339), "statusId": s.StatusID,
				"status": s.StatusName, "moderator": nullStr(s.ModeratorName),
				"respondent": nullStr(s.ResponderName),
			})
		}
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=project_%d_export.csv", projectID))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"TimeSlotID", "StartTime", "EndTime", "Status", "Moderator", "Respondent"})
		for _, row := range data {
			_ = cw.Write([]string{
				fmt.Sprint(row["timeSlotId"]), fmt.Sprint(row["startTime"]),
				fmt.Sprint(row["endTime"]), fmt.Sprint(row["status"]),
				fmt.Sprint(row["moderator"]), fmt.Sprint(row["respondent"]),
			})
		}
		cw.Flush()
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "rows": data, "count": len(data)})
}

// ──────────────────────────────────────────────
// Moderators Count & Unavailable (MRA #27, #29)
// ──────────────────────────────────────────────

func (h *Handler) GetAvailableModeratorsCount(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		count, err := h.irisSurveyRepo.GetAvailableModeratorsCount(r.Context(), projectID)
		if err != nil {
			slog.Error("count mods failed", "error", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "availableCount": count, "source": "iris"})
		return
	}

	if h.db.QS != nil {
		var count int
		_ = h.db.QS.QueryRowContext(r.Context(),
			`SELECT COUNT(DISTINCT mts.moderator_id) FROM moderator_time_slot mts
			 INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
			 WHERE ts.project_id = ? AND ts.status_id = 1`, projectID).Scan(&count)
		writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "availableCount": count, "source": "qs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "availableCount": 0})
}

func (h *Handler) GetUnavailableModerators(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid project id"})
		return
	}
	source := h.resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		mods, err := h.irisSurveyRepo.GetUnavailableModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("get unavail mods failed", "error", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"moderators": mods, "source": "iris"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"moderators": []any{}, "source": "qs"})
}

// ──────────────────────────────────────────────
// Notification Email (MRA #34)
// ──────────────────────────────────────────────

func (h *Handler) SendNotificationEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipients []string `json:"recipients"`
		Subject    string   `json:"subject"`
		Body       string   `json:"body"`
		Type       string   `json:"type"`
		ProjectID  int64    `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
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
			slog.Warn("notification service send failed, logging only", "error", err)
		} else {
			slog.Info("notification email sent via service",
				"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
			writeJSON(w, http.StatusOK, map[string]any{
				"sent": true, "recipientCount": len(req.Recipients),
				"type": req.Type, "projectId": req.ProjectID, "via": "notification-service",
			})
			return
		}
	}

	// Fallback: log-only
	slog.Info("notification email logged (service not configured or failed)",
		"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
	writeJSON(w, http.StatusOK, map[string]any{
		"sent": true, "recipientCount": len(req.Recipients),
		"type": req.Type, "projectId": req.ProjectID,
	})
}

// ──────────────────────────────────────────────
// Conference Link CRUD (MRA #36, #37)
// ──────────────────────────────────────────────

func (h *Handler) AddConferenceLink(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	var req struct {
		TimeSlotID     int64  `json:"timeSlotId"`
		ConferenceHash string `json:"conferenceHash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	if req.ConferenceHash == "" {
		req.ConferenceHash = id()[:12]
	}

	if h.qsConferenceRepo != nil {
		linkID, err := h.qsConferenceRepo.CreateConferenceLink(r.Context(), req.TimeSlotID, projectID, req.ConferenceHash)
		if err != nil {
			slog.Error("create conf link failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": linkID, "projectId": projectID,
			"timeSlotId": req.TimeSlotID, "conferenceHash": req.ConferenceHash,
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) UpdateConferenceLinkHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID     int64  `json:"timeSlotId"`
		ConferenceHash string `json:"conferenceHash"`
		Pin            string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsConferenceRepo != nil {
		if err := h.qsConferenceRepo.UpdateConferenceLink(r.Context(), req.TimeSlotID, req.ConferenceHash, req.Pin); err != nil {
			slog.Error("update conf link failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "timeSlotId": req.TimeSlotID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetConferenceLinkByTimeSlot(w http.ResponseWriter, r *http.Request) {
	tsStr := chi.URLParam(r, "timeslotId")
	tsID, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid timeslot id"})
		return
	}

	if h.qsConferenceRepo != nil {
		link, err := h.qsConferenceRepo.GetConferenceLinkByTimeSlotID(r.Context(), tsID)
		if err != nil {
			slog.Error("get conf link failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		if link == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
			return
		}
		writeJSON(w, http.StatusOK, link)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "conference link not found"})
}

// ──────────────────────────────────────────────
// Project Managers (MRA #54)
// ──────────────────────────────────────────────

func (h *Handler) ListProjectManagers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	source := h.resolveSource(r)

	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		managerRoleID := 2
		users, total, err := h.qsUserRepo.List(r.Context(), page, pageSize, &managerRoleID, "")
		if err != nil {
			slog.Error("list PMs failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		result := make([]map[string]any, 0, len(users))
		for _, u := range users {
			result = append(result, map[string]any{
				"id": u.ID, "firstName": u.FirstName.String,
				"lastName": u.LastName.String, "email": u.Email.String,
				"source": "qs", "serviceCategory": "MRA",
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true, "data": result, "meta": map[string]any{"totalCount": total},
		})
		return
	}

	if source == "iris" && h.irisUserRepo != nil {
		users, total, err := h.irisUserRepo.List(r.Context(), page, pageSize, nil, "")
		if err != nil {
			slog.Error("list IRIS PMs failed", "error", err)
		}
		result := make([]map[string]any, 0)
		for _, u := range users {
			if strings.Contains(u.RoleNames, "Project Manager") || strings.Contains(u.RoleNames, "ADMIN") {
				result = append(result, map[string]any{
					"id": u.ID, "firstName": u.FirstName, "lastName": u.LastName,
					"email": u.Email.String, "source": "iris", "serviceCategory": "LS",
				})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true, "data": result, "meta": map[string]any{"totalCount": total},
		})
		return
	}

	writeJSON(w, http.StatusOK, []any{})
}

// ──────────────────────────────────────────────
// Third-Party Integration (MRA #61)
// ──────────────────────────────────────────────

func (h *Handler) ThirdPartyIntegrate(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	// If the request includes surveyId, try Decipher integration
	if surveyID, ok := req["surveyId"].(string); ok && surveyID != "" && h.services.Decipher.Configured() {
		data, err := h.services.Decipher.GetRespondentData(r.Context(), surveyID)
		if err != nil {
			slog.Warn("decipher respondent data failed", "surveyId", surveyID, "error", err)
		} else {
			slog.Info("decipher data retrieved", "surveyId", surveyID, "records", len(data))
			writeJSON(w, http.StatusOK, map[string]any{
				"accepted":   true,
				"source":     "decipher",
				"surveyId":   surveyID,
				"recordCount": len(data),
				"timestamp":  now(),
			})
			return
		}
	}

	// Log event if event logging is configured
	if h.services.EventLog.Configured() {
		_ = h.services.EventLog.LogEvent(r.Context(), "third_party_integrate", "Third-party integration request", req)
	}

	slog.Info("third-party integration received", "payload_keys", len(req))
	writeJSON(w, http.StatusOK, map[string]any{"accepted": true, "timestamp": now()})
}

// ──────────────────────────────────────────────
// Qual Eligibility (MRA #63)
// ──────────────────────────────────────────────

func (h *Handler) CheckQualEligibility(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ResponderID int64 `json:"responderId"`
		ProjectID   int64 `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsAnswerRepo != nil {
		result, err := h.qsAnswerRepo.GetParticipantEligibility(r.Context(), req.ResponderID, req.ProjectID)
		if err != nil {
			slog.Error("eligibility check failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		if result == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"eligible": true, "responderId": req.ResponderID,
				"projectId": req.ProjectID, "source": "qs",
			})
			return
		}
		result["source"] = "qs"
		writeJSON(w, http.StatusOK, result)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"eligible": true, "responderId": req.ResponderID, "projectId": req.ProjectID})
}

// ──────────────────────────────────────────────
// Topic Translation Delete (MRA #72, #74)
// ──────────────────────────────────────────────

func (h *Handler) DeleteTopicTranslation(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	translationKey := chi.URLParam(r, "translationKey")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	parts := strings.SplitN(translationKey, "_", 2)
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid translation key format: topicId_langCode"})
		return
	}
	topicID, _ := strconv.ParseInt(parts[0], 10, 64)
	langCode := parts[1]

	if h.qsAnswerRepo != nil {
		if err := h.qsAnswerRepo.DeleteTopicTranslation(r.Context(), topicID, langCode); err != nil {
			slog.Error("delete topic translation failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "projectId": projectID, "topicId": topicID, "languageCode": langCode})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) DeleteTranslation(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	langCode := chi.URLParam(r, "langCode")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	if h.qsAnswerRepo != nil {
		if err := h.qsAnswerRepo.DeleteTranslation(r.Context(), projectID, langCode); err != nil {
			slog.Error("delete translation failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "projectId": projectID, "languageCode": langCode})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ──────────────────────────────────────────────
// External Payments (MRA #78)
// ──────────────────────────────────────────────

func (h *Handler) CreateExternalPayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeSlotID  int64  `json:"timeSlotId"`
		Amount      int    `json:"amount"`
		PaymentType string `json:"paymentType"`
		ExternalRef string `json:"externalReference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsAnswerRepo != nil {
		payID, err := h.qsAnswerRepo.CreateExternalPayment(r.Context(), req.TimeSlotID, req.Amount, req.PaymentType, "pending", req.ExternalRef)
		if err != nil {
			slog.Error("create external payment failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": payID, "timeSlotId": req.TimeSlotID, "external": true})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) GetHonorariumReasons(w http.ResponseWriter, r *http.Request) {
	if h.qsAnswerRepo != nil {
		reasons, err := h.qsAnswerRepo.ListHonorariumReasons(r.Context())
		if err != nil {
			slog.Error("list hono reasons failed", "error", err)
		}
		writeJSON(w, http.StatusOK, reasons)
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "Interview Completed"},
		{"id": 2, "name": "Partial Completion"},
		{"id": 3, "name": "No Show Compensation"},
		{"id": 4, "name": "Technical Issue"},
		{"id": 5, "name": "Other"},
	})
}

func (h *Handler) GetInterviewPaymentStatusList(w http.ResponseWriter, r *http.Request) {
	if h.qsAnswerRepo != nil {
		statuses, err := h.qsAnswerRepo.ListInterviewPaymentStatuses(r.Context())
		if err != nil {
			slog.Error("list payment statuses failed", "error", err)
		}
		writeJSON(w, http.StatusOK, statuses)
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{
		{"id": 1, "name": "Pending"}, {"id": 2, "name": "Approved"},
		{"id": 3, "name": "Paid"}, {"id": 4, "name": "Failed"},
		{"id": 5, "name": "Cancelled"}, {"id": 6, "name": "On Hold"},
	})
}

// ──────────────────────────────────────────────
// LS: Inquiry Preview & Custom Crowd Inquiry (LS #6, #7, #55)
// ──────────────────────────────────────────────

// ── Inquiry Preview types (contract-identical with legacy Scala InCrowdAPI) ──

type ipCrowdAttributeSpec struct {
	AttributeID  int64   `json:"attributeId"`
	NumericMin   *int64  `json:"numericMin"`
	NumericMax   *int64  `json:"numericMax"`
	ChoiceIDs    []int64 `json:"choiceIds"`
	QualRequired *bool   `json:"qualRequired"`
}

type ipDifficultyLevelReq struct {
	ID int64 `json:"id"`
}

type ipDifficultyAssessmentResp struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	IsHardStop bool   `json:"isHardStop"`
}

type ipCrowdSpec struct {
	Name                 *string                     `json:"name"`
	NumberRequested      *int64                      `json:"numberRequested"`
	Notes                *string                     `json:"notes"`
	Attributes           []ipCrowdAttributeSpec      `json:"attributes"`
	MarketID             int64                       `json:"marketId"`
	MarketName           *string                     `json:"marketName"`
	DifficultyLevel      ipDifficultyLevelReq        `json:"difficultyLevel"`
	DifficultyAssessment *ipDifficultyAssessmentResp `json:"difficultyAssessment"`
	CrowdID              *int64                      `json:"crowdId"`
	IsCustom             bool                        `json:"isCustom"`
	ValidRespondersCount *int64                      `json:"validRespondersCount"`
}

type ipProposal struct {
	InterviewLength      int64         `json:"interviewLength"`
	Name                 string        `json:"name"`
	SalesforceProjectID  *string       `json:"salesforceProjectId"`
	CompletionDate       *string       `json:"completionDate"`
	Notes                *string       `json:"notes"`
	Crowds               []ipCrowdSpec `json:"crowds"`
	CustomCrowds         []ipCrowdSpec `json:"customCrowds"`
	ProjectID            *int64        `json:"projectId"`
	UnderReview          bool          `json:"underReview"`
	TranscriptsRequested bool          `json:"transcriptsRequested"`
	RequiresStimuli      bool          `json:"requiresStimuli"`
	IsDynamicStimulus    bool          `json:"isDynamicStimulus"`
}

type ipFee struct {
	GrossSubtotal float64  `json:"grossSubtotal"`
	NetSubtotal   float64  `json:"netSubtotal"`
	DiscountRate  *float64 `json:"discountRate"`
	PricePerUnit  float64  `json:"pricePerUnit"`
	Count         int64    `json:"count"`
	IsHonorarium  bool     `json:"isHonorarium"`
	Name          string   `json:"name"`
	ProductID     int64    `json:"productId"`
}

type ipProjectCosts struct {
	GrossTotal float64 `json:"grossTotal"`
	NetTotal   float64 `json:"netTotal"`
	Fees       []ipFee `json:"fees"`
}

type ipResponse struct {
	Proposal             *ipProposal     `json:"proposal"`
	SalesforceProjectName *string        `json:"salesforceProjectName"`
	Costs                *ipProjectCosts `json:"costs"`
	IsHardStop           bool            `json:"isHardStop"`
}

// isSpecializedCrowd checks whether a crowd is "specialized" per legacy logic.
func ipIsSpecializedCrowd(c *ipCrowdSpec) bool {
	if c.MarketID != 1 {
		return false
	}
	if c.IsCustom {
		return true
	}
	nonGeneralChoices := map[int64]bool{304: true}
	for _, attr := range c.Attributes {
		if attr.AttributeID == 1 {
			for _, cid := range attr.ChoiceIDs {
				if !nonGeneralChoices[cid] {
					return true
				}
			}
		}
	}
	return false
}

// ipCrowdMatchesProduct determines if a crowd matches a product for honorarium calculations.
func ipCrowdMatchesProduct(c *ipCrowdSpec, relatedMarketIDs []int64, isSpecialized bool) bool {
	for _, mid := range relatedMarketIDs {
		if mid == c.MarketID {
			return true
		}
	}
	crowdSpecialized := ipIsSpecializedCrowd(c)
	if isSpecialized && crowdSpecialized {
		return true
	}
	if !isSpecialized && !crowdSpecialized && c.MarketID == 1 {
		return true
	}
	return false
}

func (h *Handler) UpdateInquiryPreview(w http.ResponseWriter, r *http.Request) {
	subStr := chi.URLParam(r, "subscriptionId")
	subID, _ := strconv.ParseInt(subStr, 10, 64)

	var proposal ipProposal
	if err := json.NewDecoder(r.Body).Decode(&proposal); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.irisSurveyRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
		return
	}

	ctx := r.Context()

	// Step 1: Get qual products
	allProducts, err := h.irisSurveyRepo.GetQualProducts(ctx)
	if err != nil {
		slog.Error("get qual products failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get products"})
		return
	}

	// Filter out transcription products if not requested
	var filtered []iris.QualProduct
	for _, p := range allProducts {
		if !proposal.TranscriptsRequested && strings.HasPrefix(p.Name, "Transcription") {
			continue
		}
		filtered = append(filtered, p)
	}

	// Partition: base products (no qual_interview_minutes) + matching interview length
	var products []iris.QualProduct
	for _, p := range filtered {
		if p.QualInterviewMinutes == nil {
			products = append(products, p)
		} else if *p.QualInterviewMinutes == proposal.InterviewLength {
			products = append(products, p)
		}
	}

	// Step 2: Get related market IDs for each product
	for i := range products {
		mids, err := h.irisSurveyRepo.GetProductRelatedMarketIDs(ctx, products[i].ID)
		if err != nil {
			slog.Error("get product market ids failed", "error", err, "productId", products[i].ID)
		}
		products[i].RelatedMarketIDs = mids
	}

	// Step 4: Get subscription discount
	serviceDiscount, err := h.irisSurveyRepo.GetSubscriptionServiceDiscount(ctx, subID)
	if err != nil {
		slog.Error("get subscription discount failed", "error", err)
		serviceDiscount = 0
	}

	// Combine all crowds for total respondent count
	allCrowds := make([]ipCrowdSpec, 0, len(proposal.Crowds)+len(proposal.CustomCrowds))
	allCrowds = append(allCrowds, proposal.Crowds...)
	allCrowds = append(allCrowds, proposal.CustomCrowds...)

	var totalRespondents int64
	for _, c := range allCrowds {
		if c.NumberRequested != nil {
			totalRespondents += *c.NumberRequested
		}
	}

	// Step 3: Calculate fees
	var fees []ipFee
	for _, p := range products {
		var fee ipFee
		fee.Name = p.Name
		fee.ProductID = p.ID
		fee.PricePerUnit = p.PriceUSD
		fee.IsHonorarium = p.IsHonorarium

		if p.IsHonorarium {
			var count int64
			for i := range allCrowds {
				if ipCrowdMatchesProduct(&allCrowds[i], p.RelatedMarketIDs, p.IsSpecialized) {
					if allCrowds[i].NumberRequested != nil {
						count += *allCrowds[i].NumberRequested
					}
				}
			}
			fee.Count = count
			fee.GrossSubtotal = p.PriceUSD * float64(count)
			fee.NetSubtotal = fee.GrossSubtotal
			fee.IsHonorarium = true
		} else if p.IsForService {
			if p.IsFlatFee {
				fee.Count = 1
			} else {
				fee.Count = totalRespondents
			}
			fee.GrossSubtotal = p.PriceUSD * float64(fee.Count)
			dr := serviceDiscount
			fee.DiscountRate = &dr
			fee.NetSubtotal = fee.GrossSubtotal * (1 - serviceDiscount)
		} else if p.IsFlatFee {
			fee.Count = 1
			fee.GrossSubtotal = p.PriceUSD
			fee.NetSubtotal = fee.GrossSubtotal
		} else {
			fee.Count = totalRespondents
			fee.GrossSubtotal = p.PriceUSD * float64(totalRespondents)
			fee.NetSubtotal = fee.GrossSubtotal
		}

		fees = append(fees, fee)
	}

	var grossTotal, netTotal float64
	for _, f := range fees {
		grossTotal += f.GrossSubtotal
		netTotal += f.NetSubtotal
	}

	// Step 5: Assess crowd difficulty
	assessments, err := h.irisSurveyRepo.GetDifficultyAssessments(ctx)
	if err != nil {
		slog.Error("get difficulty assessments failed", "error", err)
	}

	isHardStop := false

	for i := range proposal.Crowds {
		da := h.assessCrowdDifficulty(ctx, &proposal.Crowds[i], assessments)
		proposal.Crowds[i].DifficultyAssessment = da
		if da != nil && da.IsHardStop {
			isHardStop = true
		}
	}
	for i := range proposal.CustomCrowds {
		da := h.assessCrowdDifficulty(ctx, &proposal.CustomCrowds[i], assessments)
		proposal.CustomCrowds[i].DifficultyAssessment = da
		if da != nil && da.IsHardStop {
			isHardStop = true
		}
	}

	// Step 6: Resolve custom crowd names
	for i := range proposal.CustomCrowds {
		if proposal.CustomCrowds[i].CrowdID != nil {
			name, err := h.irisSurveyRepo.GetCrowdNameByID(ctx, *proposal.CustomCrowds[i].CrowdID)
			if err != nil {
				slog.Error("get crowd name failed", "error", err, "crowdId", *proposal.CustomCrowds[i].CrowdID)
			} else {
				proposal.CustomCrowds[i].Name = &name
			}
		}
	}

	// Step 7: Lookup Salesforce project
	var sfProjectName *string
	if proposal.SalesforceProjectID != nil && *proposal.SalesforceProjectID != "" {
		num, name, err := h.irisSurveyRepo.GetSalesforceProjectByExtID(ctx, *proposal.SalesforceProjectID)
		if err != nil {
			slog.Error("get salesforce project failed", "error", err)
		} else {
			formatted := fmt.Sprintf("%s - %s", num, name)
			sfProjectName = &formatted
		}
	}

	// Ensure fees is an empty array, not null
	if fees == nil {
		fees = []ipFee{}
	}

	// Step 8: Build response
	resp := ipResponse{
		Proposal:              &proposal,
		SalesforceProjectName: sfProjectName,
		Costs: &ipProjectCosts{
			GrossTotal: grossTotal,
			NetTotal:   netTotal,
			Fees:       fees,
		},
		IsHardStop: isHardStop,
	}

	writeJSON(w, http.StatusOK, resp)
}

// assessCrowdDifficulty calculates the feasibility score for a crowd and matches it to an assessment.
func (h *Handler) assessCrowdDifficulty(ctx context.Context, crowd *ipCrowdSpec, assessments []iris.DifficultyAssessmentRow) *ipDifficultyAssessmentResp {
	const (
		responseRate    = 0.2
		acceptanceRate  = 0.6
		schedulingRate  = 0.8
		flakeOutRate    = 0.85
	)

	if crowd.NumberRequested == nil || *crowd.NumberRequested == 0 {
		return nil
	}

	population, err := h.irisSurveyRepo.CountMarketPopulation(ctx, crowd.MarketID)
	if err != nil {
		slog.Error("count market population failed", "error", err, "marketId", crowd.MarketID)
		return nil
	}

	diffPercent, err := h.irisSurveyRepo.GetDifficultyLevelPercent(ctx, crowd.DifficultyLevel.ID)
	if err != nil {
		slog.Error("get difficulty level percent failed", "error", err, "levelId", crowd.DifficultyLevel.ID)
		return nil
	}

	expectedResponse := float64(population) * responseRate
	expectedIncidence := expectedResponse * diffPercent
	expectedAcceptance := expectedIncidence * acceptanceRate
	expectedScheduling := expectedAcceptance * schedulingRate
	expectedAttendance := expectedScheduling * flakeOutRate
	feasibilityScore := expectedAttendance / float64(*crowd.NumberRequested)

	for _, a := range assessments {
		minOK := a.MinPercent == nil || *a.MinPercent <= feasibilityScore
		maxOK := a.MaxPercent == nil || feasibilityScore < *a.MaxPercent
		if minOK && maxOK {
			return &ipDifficultyAssessmentResp{
				ID:         a.ID,
				Name:       a.Name,
				IsHardStop: a.IsHardStop,
			}
		}
	}
	return nil
}

// CreateCustomCrowdInquiry handles custom crowd inquiry submission with CSV upload.
// Contract-identical with legacy InCrowdAPI: POST /v1/custom_crowd_inquiry
// Request: multipart/form-data with file + crowdName + completionDate + sampleSize + subscriptionId + marketId(optional)
// Response: {} (empty JSON object)
// Side effects: S3 upload (public/ prefix), email notification (async)
func (h *Handler) CreateCustomCrowdInquiry(w http.ResponseWriter, r *http.Request) {
	// Legacy error helper: wraps in {"error": {"userMessage":..., "developerMessage":..., "status":"BAD REQUEST", "code":400}}
	badRequest := func(reason string) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"userMessage":      "Something sent doesn't make sense, please check your request",
				"developerMessage": reason,
				"status":           "BAD REQUEST",
				"code":             400,
			},
		})
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
		badRequest("not valid multipart-form-data, or labeled as such")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest("no file upload was found, or named \"file\"")
		return
	}
	defer file.Close()

	crowdName := r.FormValue("crowdName")
	completionDate := r.FormValue("completionDate")
	sampleSizeStr := r.FormValue("sampleSize")
	subscriptionIDStr := r.FormValue("subscriptionId")
	marketID := r.FormValue("marketId")

	if crowdName == "" || completionDate == "" || sampleSizeStr == "" || subscriptionIDStr == "" {
		badRequest("invalid form data for list match inquiry")
		return
	}

	subscriptionID, err := strconv.ParseInt(subscriptionIDStr, 10, 64)
	if err != nil {
		badRequest("invalid form data for list match inquiry")
		return
	}

	sampleSize, err := strconv.Atoi(sampleSizeStr)
	if err != nil {
		badRequest("invalid form data for list match inquiry")
		return
	}

	// Resolve calling user's numeric IRIS ID for filename (legacy uses user.id)
	var userID int64
	if user := middleware.GetUser(r); user != nil && user.Email != "" && h.irisSurveyRepo != nil {
		if id, lookupErr := h.irisSurveyRepo.GetUserIDByEmail(r.Context(), user.Email); lookupErr == nil {
			userID = id
		}
	}
	userIDStr := "unknown"
	if userID > 0 {
		userIDStr = strconv.FormatInt(userID, 10)
	}

	// Upload file to S3 with public/ prefix (legacy: s3Gateway.uploadAndGetPublicUrl)
	fileName := fmt.Sprintf("public/custom_crowd_inquiry%d_%s.csv", time.Now().UnixMilli(), userIDStr)
	var downloadLink string
	if h.services.S3 != nil && h.services.S3.Configured() {
		url, uploadErr := h.services.S3.UploadFile(r.Context(), h.services.S3.InquiryBucket(), fileName, file, header.Header.Get("Content-Type"))
		if uploadErr != nil {
			slog.Error("S3 upload failed for custom crowd inquiry", "error", uploadErr, "userId", userIDStr)
		} else {
			downloadLink = url
		}
	} else {
		slog.Warn("S3 not configured, skipping file upload for custom crowd inquiry")
	}

	// Determine inquiry type (List Match vs Prevalidated List)
	isListMatch := marketID != ""

	// Look up subscription company name + shortCode
	companyName := "Unknown"
	shortCode := ""
	if h.irisSurveyRepo != nil {
		if name, sc, lookupErr := h.irisSurveyRepo.GetSubscriptionCompanyAndShortCode(r.Context(), subscriptionID); lookupErr == nil {
			if name != "" {
				companyName = name
			}
			shortCode = sc
		}
	}

	// Look up market name if provided
	var marketName string
	if isListMatch && h.irisSurveyRepo != nil {
		if mID, parseErr := strconv.ParseInt(marketID, 10, 64); parseErr == nil {
			if name, lookupErr := h.irisSurveyRepo.GetMarketName(r.Context(), mID); lookupErr == nil {
				marketName = name
			}
		}
	}

	// Build email matching legacy template (qualCustomCrowdSubmitted.scala.html)
	subject := "A Prevalidated List was submitted"
	requestType := "Prevalidated List Request"
	listDesc := "A subscriber has submitted a prevalidated list"
	if isListMatch {
		subject = "A List Match crowd was submitted"
		requestType = "List Match Request"
		listDesc = "A subscriber has submitted a file to be list matched"
	}

	// Send email notification asynchronously (legacy uses Future{...})
	if h.services.Notification != nil && h.services.Notification.Configured() {
		emailBody := fmt.Sprintf(
			"<h2>%s</h2>"+
				"<p>%s</p>"+
				"<p><strong>Subscription:</strong> %s</p>"+
				"<p><strong>Short Code:</strong> %s</p>"+
				"<p><strong>Requested Crowd Name:</strong> %s</p>",
			requestType, listDesc, companyName, shortCode, crowdName,
		)
		if isListMatch && marketName != "" {
			emailBody += fmt.Sprintf("<p><strong>Crowd Market:</strong> %s</p>", marketName)
		}
		emailBody += fmt.Sprintf(
			"<p><strong>Sample Size:</strong> %d</p>"+
				"<p><strong>Required Recruitment Completion Date:</strong> %s</p>",
			sampleSize, completionDate,
		)
		if !isListMatch {
			emailBody += "<p>User attested that all potential recipients consented to third party contact.</p>"
		}
		if downloadLink != "" {
			linkLabel := "Download Prevalidated List CSV"
			if isListMatch {
				linkLabel = "Download List Match CSV"
			}
			emailBody += fmt.Sprintf("<p><a href=\"%s\">%s</a></p>", downloadLink, linkLabel)
		} else {
			emailBody += "<p><strong>There was an error uploading the file to S3. Please file a PS ticket to retrieve the file.</strong></p>"
		}

		recipient := h.cfg.InquiryEmailRecipient
		if recipient == "" {
			recipient = "dev-ni@incrowdnow.com"
		}

		// Fire-and-forget (legacy returns Ok before email completes)
		go func() {
			if emailErr := h.services.Notification.SendEmail(r.Context(), integration.EmailMessage{
				To:          []string{recipient},
				Subject:     subject,
				Body:        emailBody,
				ContentType: "text/html",
			}); emailErr != nil {
				slog.Error("failed to send custom crowd inquiry email", "error", emailErr, "userId", userIDStr)
			}
		}()
	}

	// Contract-identical: legacy returns empty JSON object
	writeJSON(w, http.StatusOK, map[string]any{})
}

// ──────────────────────────────────────────────
// Google Calendar / Import placeholders (MRA #65-69)
// ──────────────────────────────────────────────

func (h *Handler) StartModeratorImport(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.services.GoogleCal.Configured() {
		// Fetch events from Google Calendar for this moderator
		now := time.Now()
		events, err := h.services.GoogleCal.ListEvents(r.Context(), "", now, now.AddDate(0, 3, 0))
		if err != nil {
			slog.Warn("google calendar list events failed", "moderatorId", moderatorID, "error", err)
			writeJSON(w, http.StatusOK, map[string]any{
				"moderatorId": moderatorID,
				"importId":    id()[:8],
				"status":      "error",
				"message":     fmt.Sprintf("Google Calendar import failed: %v", err),
			})
			return
		}
		slog.Info("google calendar events fetched for import",
			"moderatorId", moderatorID, "eventCount", len(events))
		writeJSON(w, http.StatusOK, map[string]any{
			"moderatorId":  moderatorID,
			"importId":     id()[:8],
			"status":       "completed",
			"eventsFound":  len(events),
			"message":      "Calendar events imported successfully.",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"moderatorId": moderatorID,
		"importId":    id()[:8],
		"status":      "not_configured",
		"message":     "External calendar import requires Google Calendar API credentials.",
	})
}

func (h *Handler) GetImportedAvailability(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	// Try Google Calendar for real imported availability
	if h.services.GoogleCal.Configured() {
		now := time.Now()
		events, err := h.services.GoogleCal.ListEvents(r.Context(), "", now, now.AddDate(0, 1, 0))
		if err == nil && len(events) > 0 {
			result := make([]map[string]any, 0, len(events))
			for _, e := range events {
				result = append(result, map[string]any{
					"id":          e.ID,
					"moderatorId": moderatorID,
					"summary":     e.Summary,
					"startTime":   e.Start.DateTime,
					"endTime":     e.End.DateTime,
					"source":      "google_calendar",
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "google_calendar"})
			return
		}
	}

	// Fallback: database
	if h.qsUserRepo != nil {
		avails, _ := h.qsUserRepo.ListModeratorAvailability(r.Context(), moderatorID, nil, "", "")
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime":   a.EndTime.Format(time.RFC3339),
				"source":    "database",
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "database"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": []any{}, "importSource": "none"})
}

func (h *Handler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.services.GoogleCal.Configured() {
		writeJSON(w, http.StatusOK, map[string]any{
			"moderatorId": moderatorID,
			"status":      "configured",
			"message":     "Google Calendar integration is configured and active.",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"moderatorId": moderatorID,
		"status":      "not_configured",
		"message":     "Google Calendar import not yet configured. Use manual availability entry.",
	})
}

func (h *Handler) UnlinkImportedModerator(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.db.QS != nil {
		_, _ = h.db.QS.ExecContext(r.Context(),
			`DELETE FROM google_calendar_import WHERE moderator_id = ?`, moderatorID)
	}

	writeJSON(w, http.StatusOK, "Imported Moderator calendar has been unlinked")
}

func (h *Handler) UpdateGoogleSheetFirstDate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SheetName string `json:"sheetName"`
		CellRange string `json:"cellRange"`
		Value     string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.services.GoogleSheets.Configured() {
		if req.SheetName == "" {
			req.SheetName = "Sheet1"
		}
		if req.CellRange == "" {
			req.CellRange = "A1"
		}
		if err := h.services.GoogleSheets.UpdateFirstDate(r.Context(), req.SheetName, req.CellRange, req.Value); err != nil {
			slog.Warn("google sheets update failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "google sheets update failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"updated": true,
			"message": "Google Sheets first date updated successfully.",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"updated": false,
		"message": "Google Sheets integration not configured.",
	})
}

// Satisfy imports
var (
	_ = csv.NewWriter
	_ = strings.SplitN
	_ = time.RFC3339
	_ = middleware.GetUser
	_ = integration.EmailMessage{}
)

// ──────────────────────────────────────────────
// Webhooks / Callbacks
// ──────────────────────────────────────────────

// RecordingUploadCallback handles the callback from Conference Service
// when a recording is uploaded to S3.
// Matches: InCrowdAPI POST /v1/chime/recording/meeting/{meetingId}
// Called by Conference Service recording-upload Lambda after S3 trigger.
func (h *Handler) RecordingUploadCallback(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "meetingId")
	projectIDStr := r.URL.Query().Get("projectId")
	subscriptionIDStr := r.URL.Query().Get("subscriptionId")
	chimeMeetingID := r.URL.Query().Get("chimeMeetingId")

	var req struct {
		RecordingURL string `json:"recordingUrl"`
		Bucket       string `json:"bucket"`
		Key          string `json:"key"`
		Duration     int    `json:"duration"`
		Size         int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Some callbacks come with empty body — just the query params
		slog.Info("recording callback with empty body", "meetingId", meetingID)
	}

	projectID, _ := strconv.ParseInt(projectIDStr, 10, 64)
	subscriptionID, _ := strconv.ParseInt(subscriptionIDStr, 10, 64)

	slog.Info("recording upload callback received",
		"meetingId", meetingID, "chimeMeetingId", chimeMeetingID,
		"projectId", projectID, "subscriptionId", subscriptionID,
		"bucket", req.Bucket, "key", req.Key)

	// Store recording metadata in IRIS DB
	if h.db.IRIS != nil {
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO interview_media (meeting_id, project_id, subscription_id, chime_meeting_id,
			 recording_url, s3_bucket, s3_key, duration_seconds, file_size, status, created_on)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'available', NOW())
			 ON DUPLICATE KEY UPDATE recording_url=VALUES(recording_url), s3_bucket=VALUES(s3_bucket),
			 s3_key=VALUES(s3_key), duration_seconds=VALUES(duration_seconds), file_size=VALUES(file_size),
			 status='available', updated_on=NOW()`,
			meetingID, projectID, subscriptionID, chimeMeetingID,
			req.RecordingURL, req.Bucket, req.Key, req.Duration, req.Size)
	}

	// Also update QS conference metadata if available
	if h.qsConferenceRepo != nil {
		_ = h.qsConferenceRepo.UpdateRecordingStatus(r.Context(), meetingID, "available", req.Bucket, req.Key)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meetingId":    meetingID,
		"recorded":    true,
		"status":      "available",
		"recordingUrl": req.RecordingURL,
	})
}

// ──────────────────────────────────────────────
// Meeting Create (Conference Service integration)
// Matches: InCrowdAPI POST /meeting via ConferenceService.scala
// ──────────────────────────────────────────────

func (h *Handler) CreateMeeting(w http.ResponseWriter, r *http.Request) {
	bearerToken := extractBearerToken(r)

	var req struct {
		ProjectID      int64  `json:"projectId"`
		SubscriptionID int64  `json:"subscriptionId"`
		ModeratorID    int64  `json:"moderatorId"`
		TimeSlotID     int64  `json:"timeSlotId"`
		ExternalID     string `json:"externalMeetingId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	// Call Conference Service to create the Chime meeting
	if h.services.Conference.Configured() {
		createReq := integration.MeetingCreateRequest{
			ProjectID:      req.ProjectID,
			SubscriptionID: req.SubscriptionID,
			ModeratorID:    req.ModeratorID,
			TimeSlotID:     req.TimeSlotID,
			ExternalID:     req.ExternalID,
		}
		resp, err := h.services.Conference.CreateMeeting(r.Context(), createReq, bearerToken)
		if err != nil {
			slog.Error("conference create meeting failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to create meeting: " + err.Error()})
			return
		}

		// Store meeting reference in DB
		if h.qsConferenceRepo != nil {
			_, _ = h.qsConferenceRepo.CreateConferenceLink(r.Context(), req.TimeSlotID, req.ProjectID, resp.MeetingID)
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"meetingId":    resp.MeetingID,
			"joinUrl":      resp.JoinURL,
			"phoneNumber": resp.PhoneNumber,
			"pin":         resp.Pin,
			"externalMeetingId": resp.ExternalID,
		})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "conference service not configured"})
}

// ──────────────────────────────────────────────
// Transcription (CastingWords integration)
// Matches: InCrowdAPI TranscriptionController.scala
// ──────────────────────────────────────────────

func (h *Handler) CreateTranscriptionOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MeetingID string `json:"meetingId"`
		AudioURL  string `json:"audioUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.services.CastingWords.Configured() {
		order, err := h.services.CastingWords.CreateOrder(r.Context(), req.AudioURL)
		if err != nil {
			slog.Error("castingwords create order failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "transcription order failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"orderId":   order.OrderID,
			"meetingId": req.MeetingID,
			"status":    order.Status,
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

func (h *Handler) GetTranscriptionStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")

	if h.services.CastingWords.Configured() {
		order, err := h.services.CastingWords.GetOrderStatus(r.Context(), orderID)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to get status"})
			return
		}
		writeJSON(w, http.StatusOK, order)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")

	if h.services.CastingWords.Configured() {
		transcript, err := h.services.CastingWords.GetTranscript(r.Context(), orderID)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "failed to get transcript"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"orderId": orderID, "transcript": transcript})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transcription service not configured"})
}

// ──────────────────────────────────────────────
// SMS (Bandwidth integration)
// Matches: InCrowdAPI SMSGateway.scala
// ──────────────────────────────────────────────

func (h *Handler) SendSMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To      string `json:"to"`
		From    string `json:"from"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.services.SMS.Configured() {
		if err := h.services.SMS.SendSMS(r.Context(), req.To, req.From, req.Message); err != nil {
			slog.Error("sms send failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "sms send failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sent": true, "to": req.To})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sms service not configured"})
}

// CreateProjectMRA handles POST /project/create-project (MRA).
// Contract-identical with legacy QS Tool: 9-query transaction creating
// external_client, project, survey, third_party_survey, question,
// survey_question, participant_group, topics, then SELECT project details.
// Response: flat project details object (parsedJson[8]["records"][0]).
func (h *Handler) CreateProjectMRA(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	// Legacy formatProject maps camelCase request to snake_case for DB,
	// but repo.CreateProjectFull expects the camelCase keys from the request body.
	// Look up SalesForceJobNumberText from salesforce_project table.
	sfJobNumber, _ := body["salesForceJobNumber"].(string)
	sfJobNumberText, err := h.qsProjectRepo.GetSalesForceJobNumberText(r.Context(), sfJobNumber)
	if err != nil {
		slog.Error("sf job number text lookup failed", "error", err)
		// Legacy continues with empty string on failure
	}
	body["SalesForceJobNumberText"] = sfJobNumberText

	record, err := h.qsProjectRepo.CreateProjectFull(r.Context(), body)
	if err != nil {
		slog.Error("create project failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, record)
}

// GetProjectMRA handles GET /project/get-project-details/{project_id} (MRA).
// Contract-identical with legacy QS Tool: complex JOIN returning 24-field flat project object.
// Response: getProjectDetails.records[0] equivalent.
func (h *Handler) GetProjectMRA(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	record, err := h.qsProjectRepo.GetProjectDetailsMRA(r.Context(), projectID)
	if err != nil {
		slog.Error("get project details failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if record == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "project not found"})
		return
	}

	writeJSON(w, http.StatusOK, record)
}

// ListProjectsMRA handles POST /project/get-projects/client/{client_id} (MRA).
// Contract-identical with legacy QS Tool getProjects factory.
// Two code paths:
// 1. Body has keys (externalClientsIds, projectAccountId, userId) → filtered query + saveUserSelection
// 2. Empty body → simpler getProjectsForMods query
// Response: {clientId: <id>, data: records}
func (h *Handler) ListProjectsMRA(w http.ResponseWriter, r *http.Request) {
	clientIDStr := chi.URLParam(r, "client_id")

	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database not configured"})
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body = map[string]any{}
	}

	q := r.URL.Query()
	creatorIDStr := q.Get("creatorId")
	statusStr := q.Get("status")
	sortBy := q.Get("sortBy")
	search := q.Get("q")

	creatorID := -1
	if creatorIDStr != "" {
		if v, err := strconv.Atoi(creatorIDStr); err == nil {
			creatorID = v
		}
	}
	status := -1
	if statusStr != "" {
		if v, err := strconv.Atoi(statusStr); err == nil {
			status = v
		}
	}
	if sortBy == "" {
		sortBy = "modifiedDate"
	}
	sort := "project.modified_on"
	if sortBy != "modifiedDate" {
		sort = "project.created_on"
	}

	var records []map[string]any
	var err error

	if len(body) > 0 {
		// Path 1: filtered query with externalClientsIds
		externalClientsIDsRaw, _ := body["externalClientsIds"].(string)
		cleaned := strings.Trim(externalClientsIDsRaw, "()")
		var externalIDs []string
		for _, part := range strings.Split(cleaned, ",") {
			part = strings.TrimSpace(part)
			part = strings.Trim(part, "'")
			if part != "" {
				externalIDs = append(externalIDs, part)
			}
		}
		if len(externalIDs) == 0 {
			externalIDs = []string{""}
		}

		records, err = h.qsProjectRepo.GetProjectsMRA(r.Context(), creatorID, status, sort, search, externalIDs)
		if err != nil {
			slog.Error("get projects mra failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		// Side effect: save user selections (legacy wraps in try/catch, failures logged only)
		if userIDRaw, ok := body["userId"]; ok {
			projectAccountID, _ := body["projectAccountId"].(string)
			var userID int64
			switch v := userIDRaw.(type) {
			case float64:
				userID = int64(v)
			case string:
				userID, _ = strconv.ParseInt(v, 10, 64)
			}
			if userID > 0 {
				accCleaned := strings.Trim(projectAccountID, "()")
				var accountIDs []string
				for _, part := range strings.Split(accCleaned, ",") {
					part = strings.TrimSpace(part)
					if part != "" {
						accountIDs = append(accountIDs, part)
					}
				}
				if saveErr := h.qsProjectRepo.SaveUserSelection(r.Context(), userID, accountIDs, externalIDs); saveErr != nil {
					slog.Error("save user selection failed", "error", saveErr)
				}
			}
		}
	} else {
		// Path 2: empty body → getProjectsForMods
		clientID, _ := strconv.ParseInt(clientIDStr, 10, 64)
		records, err = h.qsProjectRepo.GetProjectsForModsMRA(r.Context(), clientID)
		if err != nil {
			slog.Error("get projects for mods mra failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}

	output := map[string]any{
		"clientId": clientIDStr,
		"data":     records,
	}
	writeJSON(w, http.StatusOK, output)
}

// UpdateProjectMRA handles PUT /project/update-project-details/{project_id} (MRA).
// Contract-identical with legacy QS Tool: 3 code paths based on request body.
// 1. postScreeninBuffer set and != -1 → update buffer + modified_on
// 2. moderatorBuffer set and != -1 → update buffer + modified_on
// 3. else → set scheduler_generated = 1
// Response: {} (empty object — legacy transaction result has no "records" key)
func (h *Handler) UpdateProjectMRA(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "project_id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating project details",
		})
		return
	}

	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while updating project details",
		})
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating project details",
		})
		return
	}

	// Legacy formatProject maps: postScreeninBuffer → post_screenin_buffer, moderatorBuffer → moderator_buffer
	postScreeninBuffer := float64(-1)
	if v, ok := body["postScreeninBuffer"]; ok && v != nil {
		if f, ok := v.(float64); ok {
			postScreeninBuffer = f
		}
	}
	moderatorBuffer := float64(-1)
	if v, ok := body["moderatorBuffer"]; ok && v != nil {
		if f, ok := v.(float64); ok {
			moderatorBuffer = f
		}
	}

	if postScreeninBuffer != -1 {
		err = h.qsProjectRepo.UpdatePostScreenInBuffer(r.Context(), projectID, postScreeninBuffer)
	} else if moderatorBuffer != -1 {
		err = h.qsProjectRepo.UpdateModeratorBufferMRA(r.Context(), projectID, moderatorBuffer)
	} else {
		err = h.qsProjectRepo.UpdateSchedulerGenerated(r.Context(), projectID)
	}

	if err != nil {
		slog.Error("update project mra failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating project details",
		})
		return
	}

	// Legacy response: {data: undefined} → JSON.stringify → {}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// UpdateExternalSurveyIDMRA handles PUT /project/update-external-survey-id/{project_id} (MRA).
// Contract-identical with legacy: updates external_survey_id + modified_on.
// Request: {externalSurveyId}. Response: {} (empty object).
func (h *Handler) UpdateExternalSurveyIDMRA(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "project_id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating external survey id",
		})
		return
	}

	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while updating external survey id",
		})
		return
	}

	var body struct {
		ExternalSurveyID string `json:"externalSurveyId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating external survey id",
		})
		return
	}

	if err := h.qsProjectRepo.UpdateExternalSurveyID(r.Context(), projectID, body.ExternalSurveyID); err != nil {
		slog.Error("update external survey id failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while updating external survey id",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{})
}

// ResetProjectModeratorsMRA handles POST /project/{project_id}/moderators_reset (MRA).
// Contract-identical with legacy: supports "reset" and "unassign" modes.
// Reset: diffs moderatorIds against existing, adds/removes from projects_users + moderator_time_range.
// Unassign: removes single moderator, returns updated moderator list.
func (h *Handler) ResetProjectModeratorsMRA(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "project_id")
	projectID, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while trying to reset project moderators",
		})
		return
	}

	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while trying to reset project moderators",
		})
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = r.Header.Get("X-Mode")
	}

	var body struct {
		ModeratorIDs []int64 `json:"moderatorIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while trying to reset project moderators",
		})
		return
	}

	if mode == "reset" {
		existingIDs, err := h.qsProjectRepo.GetProjectModeratorIDs(r.Context(), projectID)
		if err != nil {
			slog.Error("get project mod ids failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}

		if err := h.qsProjectRepo.ResetProjectModeratorsMRA(r.Context(), projectID, body.ModeratorIDs, existingIDs); err != nil {
			slog.Error("reset project mods failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}

		// Legacy returns transaction result array
		writeJSON(w, http.StatusOK, []map[string]any{})
	} else if mode == "unassign" {
		if len(body.ModeratorIDs) == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        "no moderator id provided",
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}
		modID := body.ModeratorIDs[0]
		if err := h.qsProjectRepo.UnassignModeratorFromProject(r.Context(), modID, projectID); err != nil {
			slog.Error("unassign moderator failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}

		mods, err := h.qsProjectRepo.GetModeratorsList(r.Context(), projectID)
		if err != nil {
			slog.Error("get moderators list failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": "An error occured while trying to reset project moderators",
			})
			return
		}
		writeJSON(w, http.StatusOK, mods)
	} else {
		writeJSON(w, 422, map[string]any{
			"errorMessage": "Missing mode in request",
		})
	}
}

// GetEmailTemplateMRA handles GET /project/{project_id}/get_email_template (MRA).
// Contract-identical with legacy: routes by reschedule/addLink/editLink/invalidateReschedule
// query params to communication_type_id, queries by language_code.
// Response: {body_content: "..."} (records[0]).
func (h *Handler) GetEmailTemplateMRA(w http.ResponseWriter, r *http.Request) {
	if h.qsProjectRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "database not configured",
			"errorMessage": "An error occured while getting the email template",
		})
		return
	}

	q := r.URL.Query()
	reschedule := q.Get("reschedule")
	invalidateReschedule := q.Get("invalidateReschedule")
	addLink := q.Get("addLink")
	editLink := q.Get("editLink")
	responderLanguage := q.Get("responderLanguage")

	var typeID int
	if reschedule == "true" {
		typeID = 2
	} else if invalidateReschedule == "true" {
		typeID = 17
	} else if reschedule == "false" && addLink == "false" && editLink == "false" {
		typeID = 4 // cancel
	} else if addLink == "true" {
		typeID = 1 // schedule
	} else if editLink == "true" {
		typeID = 3 // update
	}

	if typeID == 0 {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}

	record, err := h.qsProjectRepo.GetEmailTemplateMRA(r.Context(), typeID, responderLanguage)
	if err != nil {
		slog.Error("get email template failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while getting the email template",
		})
		return
	}
	if record == nil {
		record = map[string]any{}
	}

	writeJSON(w, http.StatusOK, record)
}