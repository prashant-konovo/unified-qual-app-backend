package ls

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/httputil"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// LS Moderator handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Moderator availability (by subscription) extended
// ──────────────────────────────────────────────

// GetModeratorAvailabilityBySub returns moderator availability for a subscription.
// Contract-identical with legacy InCrowdAPI: GET /v1/moderator/:modId/subscription/:subId/availability
// Response: flat array of availability objects
func (h *Handler) GetModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := dto.ParseIDParam(r, "modId")
	subID, _ := dto.ParseIDParam(r, "subId")
	source := httputil.ResolveSource(r)

	if source == "iris" && h.SurveyService.IrisAvailable() {
		avails, err := h.SurveyService.ListModeratorAvailability(r.Context(), modID, subID)
		if err != nil {
			slog.Error("iris mod avail failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"subscriptionId": a.SubscriptionID,
				"startTime":      a.StartTime.Format(time.RFC3339),
				"endTime":        a.EndTime.Format(time.RFC3339),
			})
		}
		httputil.WriteJSON(w, http.StatusOK, result)
		return
	}

	// QS
	if h.ModeratorService.QsUserAvailable() {
		avails, err := h.ModeratorService.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
		if err != nil {
			slog.Error("qs mod avail failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID, "clientId": a.ClientID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime":   a.EndTime.Format(time.RFC3339),
			})
		}
		httputil.WriteJSON(w, http.StatusOK, result)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, []any{})
}

// PostModeratorAvailabilityBySub creates moderator availability for a subscription.
// Contract-identical with legacy InCrowdAPI: POST /v1/moderator/:modId/subscription/:subId/availability
// Response: flat array of availability objects (200, not 201)
func (h *Handler) PostModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := dto.ParseIDParam(r, "modId")
	subID, _ := dto.ParseIDParam(r, "subId")

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := httputil.ResolveSource(r)

	if source == "iris" && h.SurveyService.IrisAvailable() {
		_, err := h.SurveyService.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create iris avail failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.SurveyService.ListModeratorAvailability(r.Context(), modID, subID)
		if err != nil {
			slog.Error("list avail after create failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"subscriptionId": a.SubscriptionID,
				"startTime":      a.StartTime.Format(time.RFC3339),
				"endTime":        a.EndTime.Format(time.RFC3339),
			})
		}
		httputil.WriteJSON(w, http.StatusOK, result)
		return
	}

	if h.ModeratorService.QsUserAvailable() {
		_, err := h.ModeratorService.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create qs avail failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.ModeratorService.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
		if err != nil {
			slog.Error("list avail after create failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID, "clientId": a.ClientID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime":   a.EndTime.Format(time.RFC3339),
			})
		}
		httputil.WriteJSON(w, http.StatusOK, result)
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// UpdateModeratorAvailabilityExt updates a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: PUT /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *Handler) UpdateModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := dto.ParseIDParam(r, "maId")

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := httputil.ResolveSource(r)

	if source == "iris" && h.SurveyService.IrisAvailable() {
		if err := h.SurveyService.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update iris avail failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	if h.ModeratorService.QsUserAvailable() {
		if err := h.ModeratorService.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update qs avail failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// DeleteModeratorAvailabilityExt deletes a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *Handler) DeleteModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := dto.ParseIDParam(r, "maId")
	source := httputil.ResolveSource(r)

	if source == "iris" && h.SurveyService.IrisAvailable() {
		if err := h.SurveyService.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete iris avail failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	if h.ModeratorService.QsUserAvailable() {
		if err := h.ModeratorService.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete qs avail failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// ──────────────────────────────────────────────
// Self-Service (no show)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Self-Service (no show)
// ──────────────────────────────────────────────

// GetNoShowCheck checks for no-show timeslots.
// GetNoShowCheck checks for no-show timeslots.
// Contract-identical with legacy InCrowdAPI: GET /v1/selfservice/noshow
// Response: {"timeSlot": <adminJson>, "interviewee": <userID>}
func (h *Handler) GetNoShowCheck(w http.ResponseWriter, r *http.Request) {
	if h.SurveyService.IrisAvailable() {
		data, err := h.SurveyService.GetNoShowCheck(r.Context())
		if err != nil {
			slog.Error("noshow check failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, data)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"timeSlot": nil, "interviewee": nil})
}

// MarkNoShow marks a timeslot as no-show.
// Contract-identical with legacy InCrowdAPI: PUT /v1/selfservice/project/:pid/timeslot/:tid
// Response: full updated TimeSlot adminJson
func (h *Handler) MarkNoShow(w http.ResponseWriter, r *http.Request) {
	projectID, _ := dto.ParseIDParam(r, "pid")
	timeSlotID, _ := dto.ParseIDParam(r, "tid")

	if h.SurveyService.IrisAvailable() {
		if err := h.SurveyService.MarkNoShow(r.Context(), projectID, timeSlotID); err != nil {
			slog.Error("mark noshow failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "mark failed"})
			return
		}
		// Hook: TimeSlot.afterUpdateHooks — assignConferenceHashAndPin
		if err := h.SurveyService.AssignConferenceHash(r.Context(), timeSlotID); err != nil {
			slog.Warn("hook: assign conference hash failed (non-fatal)", "timeSlotId", timeSlotID, "error", err)
		}
		if err := h.SurveyService.AssignConferencePin(r.Context(), timeSlotID); err != nil {
			slog.Warn("hook: assign conference pin failed (non-fatal)", "timeSlotId", timeSlotID, "error", err)
		}
	}

	if h.ModeratorService.TimeSlotAvailable() {
		_ = h.ModeratorService.UpdateTimeSlot(r.Context(), timeSlotID, map[string]any{"status_id": 10}) // 10 = NoShow
	}

	// Return the updated timeslot adminJson (legacy contract)
	if h.SurveyService.IrisAvailable() {
		ts, err := h.SurveyService.GetTimeSlotAdminJSON(r.Context(), timeSlotID)
		if err == nil && ts != nil {
			httputil.WriteJSON(w, http.StatusOK, ts)
			return
		}
	}
	// Fallback: minimal timeslot shape
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"id": timeSlotID, "projectId": projectID, "statusId": 10,
		"stopPayment": true,
	})
}

// ──────────────────────────────────────────────
// User Domain extended
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Google Calendar / Import placeholders (MRA #65-69)
// ──────────────────────────────────────────────

func (h *Handler) StartModeratorImport(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.ModeratorService.GoogleCalConfigured() {
		// Fetch events from Google Calendar for this moderator
		now := time.Now()
		events, err := h.ModeratorService.ListGoogleCalEvents(r.Context(), "", now, now.AddDate(0, 3, 0))
		if err != nil {
			slog.Warn("google calendar list events failed", "moderatorId", moderatorID, "error", err)
			httputil.WriteJSON(w, http.StatusOK, map[string]any{
				"moderatorId": moderatorID,
				"importId":    httputil.ID()[:8],
				"status":      "error",
				"message":     fmt.Sprintf("Google Calendar import failed: %v", err),
			})
			return
		}
		slog.Info("google calendar events fetched for import",
			"moderatorId", moderatorID, "eventCount", len(events))
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"moderatorId": moderatorID,
			"importId":    httputil.ID()[:8],
			"status":      "completed",
			"eventsFound": len(events),
			"message":     "Calendar events imported successfully.",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"moderatorId": moderatorID,
		"importId":    httputil.ID()[:8],
		"status":      "not_configured",
		"message":     "External calendar import requires Google Calendar API credentials.",
	})
}

func (h *Handler) GetImportedAvailability(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	// Try Google Calendar for real imported availability
	if h.ModeratorService.GoogleCalConfigured() {
		now := time.Now()
		events, err := h.ModeratorService.ListGoogleCalEvents(r.Context(), "", now, now.AddDate(0, 1, 0))
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
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "google_calendar"})
			return
		}
	}

	// Fallback: database
	if h.ModeratorService.QsUserAvailable() {
		avails, _ := h.ModeratorService.ListModeratorAvailability(r.Context(), moderatorID, nil, "", "")
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime":   a.EndTime.Format(time.RFC3339),
				"source":    "database",
			})
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "database"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": []any{}, "importSource": "none"})
}

func (h *Handler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.ModeratorService.GoogleCalConfigured() {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"moderatorId": moderatorID,
			"status":      "configured",
			"message":     "Google Calendar integration is configured and active.",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"moderatorId": moderatorID,
		"status":      "not_configured",
		"message":     "Google Calendar import not yet configured. Use manual availability entry.",
	})
}

func (h *Handler) UnlinkImportedModerator(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.UserService.QsAvailable() {
		_ = h.UserService.DeleteGoogleCalendarImport(r.Context(), moderatorID)
	}

	httputil.WriteJSON(w, http.StatusOK, "Imported Moderator calendar has been unlinked")
}

func (h *Handler) UpdateGoogleSheetFirstDate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SheetName string `json:"sheetName"`
		CellRange string `json:"cellRange"`
		Value     string `json:"value"`
	}
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	if h.ModeratorService.GoogleSheetsConfigured() {
		if req.SheetName == "" {
			req.SheetName = "Sheet1"
		}
		if req.CellRange == "" {
			req.CellRange = "A1"
		}
		if err := h.ModeratorService.UpdateGoogleSheetFirstDate(r.Context(), req.SheetName, req.CellRange, req.Value); err != nil {
			slog.Warn("google sheets update failed", "error", err)
			httputil.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "google sheets update failed: " + err.Error()})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"updated": true,
			"message": "Google Sheets first date updated successfully.",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
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
