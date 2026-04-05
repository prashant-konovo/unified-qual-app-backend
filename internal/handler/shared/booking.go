package shared

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"


	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// Booking handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Bookings (QS: timeslots with linked respondents)
// ──────────────────────────────────────────────

func (h *Handler) ListBookings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.BookingService.Available() {
		dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var projectID *int64
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		v, _ := strconv.ParseInt(pid, 10, 64)
		if v > 0 {
			projectID = &v
		}
	}

	// Bookings are timeslots that have a linked respondent (non-OPEN status)
	slots, total, err := h.BookingService.ListTimeSlots(ctx, page, pageSize, projectID, nil, nil, nil, nil)
	if err != nil {
		slog.ErrorContext(ctx, "list bookings failed", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list bookings"})
		return
	}

	// Filter to only include slots that have a respondent
	result := make([]map[string]any, 0)
	for _, s := range slots {
		if !s.ResponderID.Valid {
			continue
		}
		item := map[string]any{
			"id":              s.ID,
			"projectId":       s.ProjectID,
			"projectName":     s.ProjectName,
			"startTime":       s.StartTime.Format(time.RFC3339),
			"endTime":         s.EndTime.Format(time.RFC3339),
			"duration":        s.Duration,
			"statusId":        s.StatusID,
			"status":          s.StatusName,
			"responderId":     s.ResponderID.Int64,
			"responderName":   s.ResponderName.String,
			"source":          "qs",
			"serviceCategory": "MRA",
			"modifiedOn":      s.ModifiedOn.Format(time.RFC3339),
		}
		if s.ModeratorID.Valid {
			item["moderatorId"] = s.ModeratorID.Int64
			item["moderatorName"] = s.ModeratorName.String
		}
		if s.ConferenceHash.Valid {
			item["conferenceHash"] = s.ConferenceHash.String
		}
		result = append(result, item)
	}

	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": total,
		},
	})
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	// Booking creation is handled via ScheduleInterview which links respondent to timeslot
	dto.WriteJSON(w, http.StatusCreated, map[string]any{"message": "use POST /interviews/schedule to create bookings"})
}

func (h *Handler) GetBookingsByUser(w http.ResponseWriter, r *http.Request) {
	// This returns timeslots linked to a specific respondent
	if !h.BookingService.Available() {
		dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	// userId here would be a responder ID in the QS context
	userID := chi.URLParam(r, "userId")
	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    []map[string]any{},
		"message": fmt.Sprintf("bookings for user %s — respondent-level lookup pending", userID),
	})
}

func (h *Handler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	// Booking update is a timeslot status change
	ctx := r.Context()
	if !h.BookingService.Available() {
		dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		dto.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body dto.UpdateBookingRequest
	if errs := dto.DecodeAndValidate(r, &body); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	fields := map[string]any{}
	if body.StatusID != nil {
		fields["status_id"] = *body.StatusID
	}

	if err := h.BookingService.UpdateTimeSlot(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "update booking failed", "error", err, "id", tsID)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
		return
	}

	dto.WriteJSON(w, http.StatusOK, map[string]any{"id": tsID, "updated": true, "source": "qs"})
}

func (h *Handler) UpdateBookingReward(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.BookingService.Available() {
		dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	bookingID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		dto.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req dto.UpdateBookingRewardRequest
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}
	if req.RewardStatus == "" {
		req.RewardStatus = "not_credited"
	}
	if err := h.BookingService.UpsertReward(ctx, bookingID, req.RewardPoints, req.RewardStatus); err != nil {
		slog.Error("update booking reward", "error", err)
		dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to update reward"})
		return
	}
	dto.WriteJSON(w, http.StatusOK, map[string]any{
		"id":           bookingID,
		"rewardPoints": req.RewardPoints,
		"rewardStatus": req.RewardStatus,
		"updated":      true,
	})
}
