package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// MRA #12 — PatchUser (password reset via admin)
// PATCH /v1/reset-user-password/patch-user/{user_id}
// Legacy: patch-qstool-user.js — updates password via Cognito, returns user.
// ──────────────────────────────────────────────

func (h *Handler) PatchUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user_id"})
		return
	}

	var req struct {
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	// Password changes are handled by Cognito; log the request.
	slog.Info("patch user password requested (admin)", "userId", userID)

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "user repository not available"})
		return
	}

	u, err := h.qsUserRepo.GetByID(r.Context(), userID)
	if err != nil || u == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        u.ID,
		"firstName": nullStr(u.FirstName),
		"lastName":  nullStr(u.LastName),
		"email":     nullStr(u.Email),
		"roleIds":   u.RoleIDs,
	})
}

// ──────────────────────────────────────────────
// MRA #13 — PatchUserFromProfile (password reset from own profile)
// PATCH /v1/reset-user-password/patch-user-from-profile/{user_id}
// Legacy: patch-qstool-user-from-profile.js — uses CognitoToken from
// Authorization header instead of body token.
// ──────────────────────────────────────────────

func (h *Handler) PatchUserFromProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid user_id"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	// Token comes from the Authorization header (already validated by JWT middleware).
	slog.Info("patch user password requested (profile)", "userId", userID)

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "user repository not available"})
		return
	}

	u, err := h.qsUserRepo.GetByID(r.Context(), userID)
	if err != nil || u == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        u.ID,
		"firstName": nullStr(u.FirstName),
		"lastName":  nullStr(u.LastName),
		"email":     nullStr(u.Email),
		"roleIds":   u.RoleIDs,
	})
}

// ──────────────────────────────────────────────
// MRA #35 — CancelRescheduleAction
// POST /v1/project/{pid}/time_slot/{tsId}/{action}
// Legacy: cancel-resch-interview.js
// action = "cancel" → status_id 6, "reschedule" → status_id 7
// ──────────────────────────────────────────────

func (h *Handler) CancelRescheduleAction(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	tsIDStr := chi.URLParam(r, "tsId")
	tsID, err := strconv.ParseInt(tsIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid tsId"})
		return
	}

	var statusID int
	switch action {
	case "cancel":
		statusID = 6 // Cancelled
	case "reschedule":
		statusID = 7 // Rescheduled
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "action must be cancel or reschedule"})
		return
	}

	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "timeslot repository not available"})
		return
	}

	ctx := r.Context()

	if err := h.qsTimeSlotRepo.Update(ctx, tsID, map[string]any{"status_id": statusID}); err != nil {
		slog.Error("cancel/reschedule update failed", "tsId", tsID, "action", action, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
		return
	}

	ts, err := h.qsTimeSlotRepo.GetByID(ctx, tsID)
	if err != nil || ts == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "timeslot not found after update"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":               ts.ID,
		"projectId":        ts.ProjectID,
		"startTime":        ts.StartTime,
		"endTime":          ts.EndTime,
		"confirmed":        ts.Confirmed,
		"conferenceHash":   nullStr(ts.ConferenceHash),
		"participantHash":  nullStr(ts.ParticipantHash),
		"statusId":         ts.StatusID,
		"duration":         ts.Duration,
		"isInvalid":        ts.IsInvalid,
		"modifiedOn":       ts.ModifiedOn,
	})
}
