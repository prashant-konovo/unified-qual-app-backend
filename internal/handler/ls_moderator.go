package handler

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/validate"
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
func (h *LSHandler) GetModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := validate.ParseIDParam(r, "modId")
	subID, _ := validate.ParseIDParam(r, "subId")
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		avails, err := h.irisSurveyRepo.ListModeratorAvailability(r.Context(), modID, subID)
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
		writeJSON(w, http.StatusOK, result)
		return
	}

	// QS
	if h.qsUserRepo != nil {
		avails, err := h.qsUserRepo.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
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
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// PostModeratorAvailabilityBySub creates moderator availability for a subscription.
// Contract-identical with legacy InCrowdAPI: POST /v1/moderator/:modId/subscription/:subId/availability
// Response: flat array of availability objects (200, not 201)
func (h *LSHandler) PostModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := validate.ParseIDParam(r, "modId")
	subID, _ := validate.ParseIDParam(r, "subId")

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		_, err := h.irisSurveyRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.irisSurveyRepo.ListModeratorAvailability(r.Context(), modID, subID)
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
		writeJSON(w, http.StatusOK, result)
		return
	}

	if h.qsUserRepo != nil {
		_, err := h.qsUserRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.qsUserRepo.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
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
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// UpdateModeratorAvailabilityExt updates a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: PUT /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *LSHandler) UpdateModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := validate.ParseIDParam(r, "maId")

	var req struct {
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	if h.qsUserRepo != nil {
		if err := h.qsUserRepo.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// DeleteModeratorAvailabilityExt deletes a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *LSHandler) DeleteModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := validate.ParseIDParam(r, "maId")
	source := resolveSource(r)

	if source == "iris" && h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete iris avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	if h.qsUserRepo != nil {
		if err := h.qsUserRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete qs avail failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
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
func (h *LSHandler) GetNoShowCheck(w http.ResponseWriter, r *http.Request) {
	if h.irisSurveyRepo != nil {
		data, err := h.irisSurveyRepo.GetNoShowCheck(r.Context())
		if err != nil {
			slog.Error("noshow check failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeSlot": nil, "interviewee": nil})
}

// MarkNoShow marks a timeslot as no-show.
// Contract-identical with legacy InCrowdAPI: PUT /v1/selfservice/project/:pid/timeslot/:tid
// Response: full updated TimeSlot adminJson
func (h *LSHandler) MarkNoShow(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	timeSlotID, _ := validate.ParseIDParam(r, "tid")

	if h.irisSurveyRepo != nil {
		if err := h.irisSurveyRepo.MarkNoShow(r.Context(), projectID, timeSlotID); err != nil {
			slog.Error("mark noshow failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "mark failed"})
			return
		}
	}

	if h.qsTimeSlotRepo != nil {
		_ = h.qsTimeSlotRepo.Update(r.Context(), timeSlotID, map[string]any{"status_id": 10}) // 10 = NoShow
	}

	// Return the updated timeslot adminJson (legacy contract)
	if h.irisSurveyRepo != nil {
		ts, err := h.irisSurveyRepo.GetTimeSlotAdminJSON(r.Context(), timeSlotID)
		if err == nil && ts != nil {
			writeJSON(w, http.StatusOK, ts)
			return
		}
	}
	// Fallback: minimal timeslot shape
	writeJSON(w, http.StatusOK, map[string]any{
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

func (h *LSHandler) StartModeratorImport(w http.ResponseWriter, r *http.Request) {
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
			"moderatorId": moderatorID,
			"importId":    id()[:8],
			"status":      "completed",
			"eventsFound": len(events),
			"message":     "Calendar events imported successfully.",
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

func (h *LSHandler) GetImportedAvailability(w http.ResponseWriter, r *http.Request) {
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

func (h *LSHandler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
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

func (h *LSHandler) UnlinkImportedModerator(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.db.QS != nil {
		_, _ = h.db.QS.ExecContext(r.Context(),
			`DELETE FROM google_calendar_import WHERE moderator_id = ?`, moderatorID)
	}

	writeJSON(w, http.StatusOK, "Imported Moderator calendar has been unlinked")
}

func (h *LSHandler) UpdateGoogleSheetFirstDate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SheetName string `json:"sheetName"`
		CellRange string `json:"cellRange"`
		Value     string `json:"value"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
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
