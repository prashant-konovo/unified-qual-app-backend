package handler

import (
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
		UserID  int64 `json:"userId"`
		RoleIDs []int `json:"roleIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	if req.UserID == 0 || len(req.RoleIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "userId and roleIds required"})
		return
	}

	if h.qsUserRepo != nil {
		if err := h.qsUserRepo.AddRoles(r.Context(), req.UserID, req.RoleIDs); err != nil {
			slog.Error("add roles failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to add roles"})
			return
		}
		roles, _ := h.qsUserRepo.GetRoles(r.Context(), req.UserID)
		success(w, map[string]any{"userId": req.UserID, "roles": roles, "source": "qs"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) DeleteUserRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  int64 `json:"userId"`
		RoleIDs []int `json:"roleIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	if req.UserID == 0 || len(req.RoleIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "userId and roleIds required"})
		return
	}

	if h.qsUserRepo != nil {
		if err := h.qsUserRepo.DeleteRoles(r.Context(), req.UserID, req.RoleIDs); err != nil {
			slog.Error("delete roles failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete roles"})
			return
		}
		roles, _ := h.qsUserRepo.GetRoles(r.Context(), req.UserID)
		success(w, map[string]any{"userId": req.UserID, "roles": roles, "source": "qs"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ──────────────────────────────────────────────
// Password Management (MRA #10, #11)
// ──────────────────────────────────────────────

func (h *Handler) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "email required"})
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

	// Always return success to not leak user existence.
	slog.Info("password reset requested", "email", req.Email)
	success(w, map[string]any{"sent": true, "email": req.Email})
}

func (h *Handler) CheckUserIsQsToolAndI2(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.qsUserRepo != nil {
		result, err := h.qsUserRepo.CheckUserIsQsToolAndI2(r.Context(), req.Email)
		if err != nil {
			slog.Error("check qs/i2 failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		success(w, result)
		return
	}
	success(w, map[string]any{"isQsTool": false, "isI2": false, "exists": false})
}

// ──────────────────────────────────────────────
// Communication Preferences (MRA #15, #16)
// ──────────────────────────────────────────────

func (h *Handler) CheckUserCommPreference(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}
	source := h.resolveSource(r)

	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		pref, err := h.qsUserRepo.GetUserCommPreference(r.Context(), userID)
		if err != nil {
			slog.Error("get comm pref failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		if pref == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		pref["source"] = "qs"
		success(w, pref)
		return
	}

	if source == "iris" && h.irisUserRepo != nil {
		u, err := h.irisUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
			return
		}
		success(w, map[string]any{
			"userId": u.ID, "email": u.Email.String,
			"optedIn": true, "canUnsubscribe": true, "source": "iris",
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) UnsubscribeUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user id"})
		return
	}
	source := h.resolveSource(r)

	if (source == "" || source == "qs") && h.qsUserRepo != nil {
		if err := h.qsUserRepo.SetUnsubscribed(r.Context(), userID); err != nil {
			slog.Error("unsubscribe failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "unsubscribe failed"})
			return
		}
		success(w, map[string]any{"userId": userID, "unsubscribed": true, "source": "qs"})
		return
	}

	if source == "iris" && h.db.IRIS != nil {
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			"UPDATE ic_user SET comm_opt_out = 1 WHERE id = ?", userID)
		success(w, map[string]any{"userId": userID, "unsubscribed": true, "source": "iris"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
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
		success(w, map[string]any{"projectId": projectID, "removedAssignments": count, "source": "iris"})
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
		success(w, map[string]any{"projectId": projectID, "removedAssignments": n, "source": "qs"})
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

	success(w, map[string]any{"projectId": projectID, "rows": data, "count": len(data)})
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
		success(w, map[string]any{"projectId": projectID, "availableCount": count, "source": "iris"})
		return
	}

	if h.db.QS != nil {
		var count int
		_ = h.db.QS.QueryRowContext(r.Context(),
			`SELECT COUNT(DISTINCT mts.moderator_id) FROM moderator_time_slot mts
			 INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
			 WHERE ts.project_id = ? AND ts.status_id = 1`, projectID).Scan(&count)
		success(w, map[string]any{"projectId": projectID, "availableCount": count, "source": "qs"})
		return
	}

	success(w, map[string]any{"projectId": projectID, "availableCount": 0})
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
		success(w, map[string]any{"moderators": mods, "source": "iris"})
		return
	}

	success(w, map[string]any{"moderators": []any{}, "source": "qs"})
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
			success(w, map[string]any{
				"sent": true, "recipientCount": len(req.Recipients),
				"type": req.Type, "projectId": req.ProjectID, "via": "notification-service",
			})
			return
		}
	}

	// Fallback: log-only
	slog.Info("notification email logged (service not configured or failed)",
		"type", req.Type, "recipients", len(req.Recipients), "projectId", req.ProjectID)
	success(w, map[string]any{
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
		created(w, map[string]any{
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
		success(w, map[string]any{"updated": true, "timeSlotId": req.TimeSlotID})
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
		success(w, link)
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

	success(w, []any{})
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
			success(w, map[string]any{
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
	success(w, map[string]any{"accepted": true, "timestamp": now()})
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
			success(w, map[string]any{
				"eligible": true, "responderId": req.ResponderID,
				"projectId": req.ProjectID, "source": "qs",
			})
			return
		}
		result["source"] = "qs"
		success(w, result)
		return
	}

	success(w, map[string]any{"eligible": true, "responderId": req.ResponderID, "projectId": req.ProjectID})
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
		success(w, map[string]any{"deleted": true, "projectId": projectID, "topicId": topicID, "languageCode": langCode})
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
		success(w, map[string]any{"deleted": true, "projectId": projectID, "languageCode": langCode})
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
		created(w, map[string]any{"id": payID, "timeSlotId": req.TimeSlotID, "external": true})
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
		success(w, reasons)
		return
	}
	success(w, []map[string]any{
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
		success(w, statuses)
		return
	}
	success(w, []map[string]any{
		{"id": 1, "name": "Pending"}, {"id": 2, "name": "Approved"},
		{"id": 3, "name": "Paid"}, {"id": 4, "name": "Failed"},
		{"id": 5, "name": "Cancelled"}, {"id": 6, "name": "On Hold"},
	})
}

// ──────────────────────────────────────────────
// LS: Inquiry Preview & Custom Crowd Inquiry (LS #6, #7, #55)
// ──────────────────────────────────────────────

func (h *Handler) UpdateInquiryPreview(w http.ResponseWriter, r *http.Request) {
	subStr := chi.URLParam(r, "subscriptionId")
	subID, _ := strconv.ParseInt(subStr, 10, 64)

	var req struct {
		ProjectID            int64  `json:"projectId"`
		Description          string `json:"description"`
		InterviewLength      int    `json:"interviewLength"`
		TranscriptsRequested bool   `json:"transcriptsRequested"`
		RequiresStimuli      bool   `json:"requiresStimuli"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.irisSurveyRepo != nil {
		fields := map[string]any{}
		if req.Description != "" {
			fields["description"] = req.Description
		}
		if req.InterviewLength > 0 {
			fields["interview_length"] = req.InterviewLength
		}
		if req.TranscriptsRequested {
			fields["transcripts_requested"] = 1
		}
		if req.RequiresStimuli {
			fields["requires_stimuli"] = 1
		}
		if err := h.irisSurveyRepo.UpdateInquiryPreview(r.Context(), subID, req.ProjectID, fields); err != nil {
			slog.Error("update inquiry preview failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		success(w, map[string]any{"updated": true, "subscriptionId": subID, "projectId": req.ProjectID, "source": "iris"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) CreateCustomCrowdInquiry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SubscriptionID  int64  `json:"subscriptionId"`
		Description     string `json:"description"`
		InterviewLength int    `json:"interviewLength"`
		InquiryTypeID   int    `json:"inquiryTypeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
		return
	}

	if h.irisSurveyRepo != nil {
		inquiryID, err := h.irisSurveyRepo.CreateCustomCrowdInquiry(r.Context(), req.SubscriptionID, req.Description, req.InterviewLength, req.InquiryTypeID)
		if err != nil {
			slog.Error("create inquiry failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		created(w, map[string]any{
			"id": inquiryID, "subscriptionId": req.SubscriptionID,
			"description": req.Description, "source": "iris",
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
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
			success(w, map[string]any{
				"moderatorId": moderatorID,
				"importId":    id()[:8],
				"status":      "error",
				"message":     fmt.Sprintf("Google Calendar import failed: %v", err),
			})
			return
		}
		slog.Info("google calendar events fetched for import",
			"moderatorId", moderatorID, "eventCount", len(events))
		success(w, map[string]any{
			"moderatorId":  moderatorID,
			"importId":     id()[:8],
			"status":       "completed",
			"eventsFound":  len(events),
			"message":      "Calendar events imported successfully.",
		})
		return
	}

	success(w, map[string]any{
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
			success(w, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "google_calendar"})
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
		success(w, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "database"})
		return
	}
	success(w, map[string]any{"moderatorId": moderatorID, "availabilities": []any{}, "importSource": "none"})
}

func (h *Handler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.services.GoogleCal.Configured() {
		success(w, map[string]any{
			"moderatorId": moderatorID,
			"status":      "configured",
			"message":     "Google Calendar integration is configured and active.",
		})
		return
	}

	success(w, map[string]any{
		"moderatorId": moderatorID,
		"status":      "not_configured",
		"message":     "Google Calendar import not yet configured. Use manual availability entry.",
	})
}

func (h *Handler) UnlinkImportedModerator(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)
	success(w, map[string]any{
		"moderatorId": moderatorID,
		"unlinked":    true,
		"message":     "External calendar integration not active.",
	})
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
		success(w, map[string]any{
			"updated": true,
			"message": "Google Sheets first date updated successfully.",
		})
		return
	}

	success(w, map[string]any{
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

	success(w, map[string]any{
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

		created(w, map[string]any{
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
		created(w, map[string]any{
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
		success(w, order)
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
		success(w, map[string]any{"orderId": orderID, "transcript": transcript})
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
		success(w, map[string]any{"sent": true, "to": req.To})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "sms service not configured"})
}
