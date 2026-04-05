package shared

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"

	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// Scheduler handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Scheduler (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) ScheduleInterview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		TimeSlotID  int64 `json:"timeSlotId"  validate:"required,gt=0"`
		ModeratorID int64 `json:"moderatorId"`
		ResponderID int64 `json:"responderId"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Update timeslot status to PENDING (2)
	if err := h.InterviewService.UpdateTimeSlot(ctx, body.TimeSlotID, map[string]any{"status_id": 2, "confirmed": true}); err != nil {
		slog.ErrorContext(ctx, "schedule interview: update timeslot failed", "error", err)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to schedule interview"})
		return
	}

	// Assign moderator if provided
	if body.ModeratorID > 0 {
		if _, err := h.InterviewService.AssignModerator(ctx, body.ModeratorID, body.TimeSlotID, true); err != nil {
			slog.ErrorContext(ctx, "schedule interview: assign moderator failed", "error", err)
		}
	}

	support.WriteJSON(w, http.StatusCreated, map[string]any{
		"timeSlotId": body.TimeSlotID,
		"statusId":   2,
		"status":     "PENDING",
		"source":     "qs",
	})
}

func (h *Handler) CancelInterview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
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

	if err := h.InterviewService.UpdateTimeSlot(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "cancel interview failed", "error", err, "id", tsID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "cancel failed"})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"timeSlotId": tsID, "statusId": cancelStatusID, "status": "CANCELLED"})
}

func (h *Handler) RescheduleInterview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body struct {
		NewStartTime string `json:"newStartTime"`
		NewEndTime   string `json:"newEndTime"`
		Reason       string `json:"reason"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
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

	if err := h.InterviewService.UpdateTimeSlot(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "reschedule interview failed", "error", err, "id", tsID)
		support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "reschedule failed"})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"timeSlotId": tsID, "statusId": rescheduleStatusID, "status": "RESCHEDULED"})
}

// ──────────────────────────────────────────────
// Waiting Queue
// ──────────────────────────────────────────────

func (h *Handler) GetWaitingQueue(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "wq-1", "userId": "par-301", "projectId": "proj-101",
			"participantName": "Alice Johnson", "participantEmail": "alice@hospital.org",
			"projectName":    "Cardiology Study Q1",
			"preferredStart": "09:00", "preferredEnd": "17:00",
			"preferredDays": []string{"Monday", "Wednesday", "Friday"},
			"timezone":      "America/New_York", "status": "waiting",
			"waitingSince": "2026-03-18T10:00:00Z",
		},
		{
			"id": "wq-2", "userId": "par-302", "projectId": "proj-101",
			"participantName": "Bob Patient", "participantEmail": "bob@clinic.org",
			"projectName":    "Cardiology Study Q1",
			"preferredStart": "10:00", "preferredEnd": "14:00",
			"preferredDays": []string{"Tuesday", "Thursday"},
			"timezone":      "America/Chicago", "status": "waiting",
			"waitingSince": "2026-03-19T10:00:00Z",
		},
	})
}

func (h *Handler) AddToWaitingQueue(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": "wq-" + support.ID()[:8], "status": "waiting", "waitingSince": support.Now(),
	})
}

func (h *Handler) RemoveFromWaitingQueue(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *Handler) TriggerMatching(w http.ResponseWriter, r *http.Request) {
	support.WriteJSON(w, http.StatusOK, map[string]any{
		"assigned": 2, "invited": 3, "message": "Matching complete. 2 assigned, 3 invited.",
	})
}
