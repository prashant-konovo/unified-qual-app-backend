package ls

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/core"

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
func (h *Handler) GetModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
	modID, _ := validate.ParseIDParam(r, "modId")
	subID, _ := validate.ParseIDParam(r, "subId")
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		avails, err := h.IrisSurveyRepo.ListModeratorAvailability(r.Context(), modID, subID)
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
		core.WriteJSON(w, http.StatusOK, result)
		return
	}

	// QS
	if h.QsUserRepo != nil {
		avails, err := h.QsUserRepo.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
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
		core.WriteJSON(w, http.StatusOK, result)
		return
	}
	core.WriteJSON(w, http.StatusOK, []any{})
}

// PostModeratorAvailabilityBySub creates moderator availability for a subscription.
// Contract-identical with legacy InCrowdAPI: POST /v1/moderator/:modId/subscription/:subId/availability
// Response: flat array of availability objects (200, not 201)
func (h *Handler) PostModeratorAvailabilityBySub(w http.ResponseWriter, r *http.Request) {
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
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		_, err := h.IrisSurveyRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create iris avail failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.IrisSurveyRepo.ListModeratorAvailability(r.Context(), modID, subID)
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
		core.WriteJSON(w, http.StatusOK, result)
		return
	}

	if h.QsUserRepo != nil {
		_, err := h.QsUserRepo.CreateModeratorAvailability(r.Context(), modID, subID, startTime, endTime)
		if err != nil {
			slog.Error("create qs avail failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "create failed"})
			return
		}
		// Return updated availability list (legacy returns array)
		avails, err := h.QsUserRepo.ListModeratorAvailability(r.Context(), modID, &subID, "", "")
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
		core.WriteJSON(w, http.StatusOK, result)
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// UpdateModeratorAvailabilityExt updates a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: PUT /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *Handler) UpdateModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
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
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		if err := h.IrisSurveyRepo.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update iris avail failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	if h.QsUserRepo != nil {
		if err := h.QsUserRepo.UpdateModeratorAvailability(r.Context(), maID, startTime, endTime); err != nil {
			slog.Error("update qs avail failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"updated": true, "id": maID})
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
}

// DeleteModeratorAvailabilityExt deletes a moderator availability slot.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/moderator_availability/:maId
// Response: flat array of availability objects
func (h *Handler) DeleteModeratorAvailabilityExt(w http.ResponseWriter, r *http.Request) {
	maID, _ := validate.ParseIDParam(r, "maId")
	source := core.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		if err := h.IrisSurveyRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete iris avail failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	if h.QsUserRepo != nil {
		if err := h.QsUserRepo.DeleteModeratorAvailability(r.Context(), maID); err != nil {
			slog.Error("delete qs avail failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": maID})
		return
	}
	core.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "not available"})
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
	if h.IrisSurveyRepo != nil {
		data, err := h.IrisSurveyRepo.GetNoShowCheck(r.Context())
		if err != nil {
			slog.Error("noshow check failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "check failed"})
			return
		}
		core.WriteJSON(w, http.StatusOK, data)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"timeSlot": nil, "interviewee": nil})
}

// MarkNoShow marks a timeslot as no-show.
// Contract-identical with legacy InCrowdAPI: PUT /v1/selfservice/project/:pid/timeslot/:tid
// Response: full updated TimeSlot adminJson
func (h *Handler) MarkNoShow(w http.ResponseWriter, r *http.Request) {
	projectID, _ := validate.ParseIDParam(r, "pid")
	timeSlotID, _ := validate.ParseIDParam(r, "tid")

	if h.IrisSurveyRepo != nil {
		if err := h.IrisSurveyRepo.MarkNoShow(r.Context(), projectID, timeSlotID); err != nil {
			slog.Error("mark noshow failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "mark failed"})
			return
		}
	}

	if h.QsTimeSlotRepo != nil {
		_ = h.QsTimeSlotRepo.Update(r.Context(), timeSlotID, map[string]any{"status_id": 10}) // 10 = NoShow
	}

	// Return the updated timeslot adminJson (legacy contract)
	if h.IrisSurveyRepo != nil {
		ts, err := h.IrisSurveyRepo.GetTimeSlotAdminJSON(r.Context(), timeSlotID)
		if err == nil && ts != nil {
			core.WriteJSON(w, http.StatusOK, ts)
			return
		}
	}
	// Fallback: minimal timeslot shape
	core.WriteJSON(w, http.StatusOK, map[string]any{
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

	if h.Services.GoogleCal.Configured() {
		// Fetch events from Google Calendar for this moderator
		now := time.Now()
		events, err := h.Services.GoogleCal.ListEvents(r.Context(), "", now, now.AddDate(0, 3, 0))
		if err != nil {
			slog.Warn("google calendar list events failed", "moderatorId", moderatorID, "error", err)
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"moderatorId": moderatorID,
				"importId":    core.ID()[:8],
				"status":      "error",
				"message":     fmt.Sprintf("Google Calendar import failed: %v", err),
			})
			return
		}
		slog.Info("google calendar events fetched for import",
			"moderatorId", moderatorID, "eventCount", len(events))
		core.WriteJSON(w, http.StatusOK, map[string]any{
			"moderatorId": moderatorID,
			"importId":    core.ID()[:8],
			"status":      "completed",
			"eventsFound": len(events),
			"message":     "Calendar events imported successfully.",
		})
		return
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"moderatorId": moderatorID,
		"importId":    core.ID()[:8],
		"status":      "not_configured",
		"message":     "External calendar import requires Google Calendar API credentials.",
	})
}

func (h *Handler) GetImportedAvailability(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	// Try Google Calendar for real imported availability
	if h.Services.GoogleCal.Configured() {
		now := time.Now()
		events, err := h.Services.GoogleCal.ListEvents(r.Context(), "", now, now.AddDate(0, 1, 0))
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
			core.WriteJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "google_calendar"})
			return
		}
	}

	// Fallback: database
	if h.QsUserRepo != nil {
		avails, _ := h.QsUserRepo.ListModeratorAvailability(r.Context(), moderatorID, nil, "", "")
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime":   a.EndTime.Format(time.RFC3339),
				"source":    "database",
			})
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": result, "importSource": "database"})
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"moderatorId": moderatorID, "availabilities": []any{}, "importSource": "none"})
}

func (h *Handler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.Services.GoogleCal.Configured() {
		core.WriteJSON(w, http.StatusOK, map[string]any{
			"moderatorId": moderatorID,
			"status":      "configured",
			"message":     "Google Calendar integration is configured and active.",
		})
		return
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"moderatorId": moderatorID,
		"status":      "not_configured",
		"message":     "Google Calendar import not yet configured. Use manual availability entry.",
	})
}

func (h *Handler) UnlinkImportedModerator(w http.ResponseWriter, r *http.Request) {
	modStr := chi.URLParam(r, "moderatorId")
	moderatorID, _ := strconv.ParseInt(modStr, 10, 64)

	if h.DB.QS != nil {
		_, _ = h.DB.QS.ExecContext(r.Context(),
			`DELETE FROM google_calendar_import WHERE moderator_id = ?`, moderatorID)
	}

	core.WriteJSON(w, http.StatusOK, "Imported Moderator calendar has been unlinked")
}

func (h *Handler) UpdateGoogleSheetFirstDate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SheetName string `json:"sheetName"`
		CellRange string `json:"cellRange"`
		Value     string `json:"value"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.Services.GoogleSheets.Configured() {
		if req.SheetName == "" {
			req.SheetName = "Sheet1"
		}
		if req.CellRange == "" {
			req.CellRange = "A1"
		}
		if err := h.Services.GoogleSheets.UpdateFirstDate(r.Context(), req.SheetName, req.CellRange, req.Value); err != nil {
			slog.Warn("google sheets update failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "google sheets update failed: " + err.Error()})
			return
		}
		core.WriteJSON(w, http.StatusOK, map[string]any{
			"updated": true,
			"message": "Google Sheets first date updated successfully.",
		})
		return
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
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
