package shared

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/service"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// InterviewHandler handles timeslot and interview scheduling endpoints.
type InterviewHandler struct{ *service.Deps }

// ──────────────────────────────────────────────
// Timeslots
// ──────────────────────────────────────────────

func (h *InterviewHandler) ListTimeslots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		utilities.WriteJSON(w, http.StatusServiceUnavailable, utilities.ErrorBody{Error: "QS database unavailable"})
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
	var statusID *int
	if sid := r.URL.Query().Get("statusId"); sid != "" {
		v, _ := strconv.Atoi(sid)
		if v > 0 {
			statusID = &v
		}
	}
	var moderatorID *int64
	if mid := r.URL.Query().Get("moderatorId"); mid != "" {
		v, _ := strconv.ParseInt(mid, 10, 64)
		if v > 0 {
			moderatorID = &v
		}
	}
	var fromTime, toTime *time.Time
	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.Parse("2006-01-02", f); err == nil {
			fromTime = &t
			// when date filters are present, increase default pageSize to cover a full week
			if pageSize == 20 {
				pageSize = 500
			}
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			eod := parsed.Add(24*time.Hour - time.Second)
			toTime = &eod
		}
	}

	slots, total, err := h.InterviewService.ListTimeSlots(ctx, page, pageSize, projectID, statusID, moderatorID, fromTime, toTime)
	if err != nil {
		slog.ErrorContext(ctx, "list timeslots failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "failed to list timeslots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		result = append(result, dto.TimeslotFromListRow(s))
	}

	utilities.WriteJSON(w, http.StatusOK, utilities.NewPaginated(result, page, pageSize, total))
}

func (h *InterviewHandler) CreateTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		utilities.WriteJSON(w, http.StatusServiceUnavailable, utilities.ErrorBody{Error: "QS database unavailable"})
		return
	}

	var body dto.CreateTimeslotRequest
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	st, err := time.Parse(time.RFC3339, body.StartTime)
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: "invalid startTime format, use RFC3339"})
		return
	}
	et, err := time.Parse(time.RFC3339, body.EndTime)
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: "invalid endTime format, use RFC3339"})
		return
	}
	if body.Duration <= 0 {
		body.Duration = 15
	}

	ts := &qs.TimeSlot{
		ProjectID: body.ProjectID,
		StartTime: st,
		EndTime:   et,
		Confirmed: true,
		StatusID:  1, // OPEN
		Duration:  body.Duration,
	}

	tsID, err := h.InterviewService.Create(ctx, ts)
	if err != nil {
		slog.ErrorContext(ctx, "create timeslot failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "failed to create timeslot"})
		return
	}

	// Assign moderator if provided
	if body.ModeratorID > 0 {
		if _, err := h.InterviewService.AssignModerator(ctx, body.ModeratorID, tsID, true); err != nil {
			slog.ErrorContext(ctx, "assign moderator failed", "error", err, "timeSlotId", tsID, "moderatorId", body.ModeratorID)
		}
	}

	utilities.WriteJSON(w, http.StatusCreated, utilities.MutationResult{ID: tsID, Source: "qs"})
}

func (h *InterviewHandler) GetTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		utilities.WriteJSON(w, http.StatusServiceUnavailable, utilities.ErrorBody{Error: "QS database unavailable"})
		return
	}

	tsID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: err.Error()})
		return
	}

	ts, err := h.InterviewService.GetByID(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot failed", "error", err, "id", tsID)
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "database error"})
		return
	}
	if ts == nil {
		utilities.WriteJSON(w, http.StatusNotFound, utilities.ErrorBody{Error: "timeslot not found"})
		return
	}

	result := dto.TimeslotFromDetail(ts)

	// Get assigned moderators
	mods, err := h.InterviewService.GetModerators(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot moderators failed", "error", err)
	}
	if mods != nil {
		modList := make([]map[string]any, 0, len(mods))
		for _, m := range mods {
			modList = append(modList, dto.TimeslotModerator(m))
		}
		result["moderators"] = modList
	}

	// Get linked respondent
	resp, err := h.InterviewService.GetRespondent(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot respondent failed", "error", err)
	}
	if resp != nil {
		result["respondent"] = dto.TimeslotRespondent(resp)
	}

	utilities.WriteJSON(w, http.StatusOK, result)
}

func (h *InterviewHandler) UpdateTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		utilities.WriteJSON(w, http.StatusServiceUnavailable, utilities.ErrorBody{Error: "QS database unavailable"})
		return
	}

	tsID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: err.Error()})
		return
	}

	var body dto.UpdateTimeslotRequest
	if errs := utilities.DecodeAndValidate(r, &body); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	fields := map[string]any{}
	if body.StatusID != nil {
		fields["status_id"] = *body.StatusID
	}
	if body.Confirmed != nil {
		fields["confirmed"] = *body.Confirmed
	}
	if body.IsInvalid != nil {
		fields["is_invalid"] = *body.IsInvalid
	}
	if body.Duration != nil {
		fields["duration"] = *body.Duration
	}
	if body.StartTime != "" {
		if st, err := time.Parse(time.RFC3339, body.StartTime); err == nil {
			fields["start_time"] = st
		}
	}
	if body.EndTime != "" {
		if et, err := time.Parse(time.RFC3339, body.EndTime); err == nil {
			fields["end_time"] = et
		}
	}

	if err := h.InterviewService.UpdateTimeSlot(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "update timeslot failed", "error", err, "id", tsID)
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "update failed"})
		return
	}

	utilities.WriteJSON(w, http.StatusOK, utilities.MutationResult{ID: tsID, Updated: true, Source: "qs"})
}

func (h *InterviewHandler) DeleteTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		utilities.WriteJSON(w, http.StatusServiceUnavailable, utilities.ErrorBody{Error: "QS database unavailable"})
		return
	}

	tsID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: err.Error()})
		return
	}

	if err := h.InterviewService.DeleteTimeSlot(ctx, tsID); err != nil {
		slog.ErrorContext(ctx, "delete timeslot failed", "error", err, "id", tsID)
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "delete failed"})
		return
	}

	utilities.WriteJSON(w, http.StatusOK, utilities.MutationResult{ID: tsID, Deleted: true, Source: "qs"})
}

// ──────────────────────────────────────────────
// Interview Slots
// ──────────────────────────────────────────────

func (h *InterviewHandler) GenerateSlots(w http.ResponseWriter, r *http.Request) {
	// Slot generation remains a stub — requires complex scheduling algorithm
	// matching moderator availability, project duration, buffer times, etc.
	slots := []map[string]any{
		{"id": "slot-1", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T09:00:00Z", "end": "2026-04-01T09:30:00Z", "capacity": 1},
		{"id": "slot-2", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T09:30:00Z", "end": "2026-04-01T10:00:00Z", "capacity": 1},
		{"id": "slot-3", "projectId": "proj-101", "moderatorId": "mod-201",
			"start": "2026-04-01T10:00:00Z", "end": "2026-04-01T10:30:00Z", "capacity": 1},
	}
	utilities.WriteJSON(w, http.StatusOK, slots)
}

func (h *InterviewHandler) GetAvailableSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.InterviewService.TimeSlotAvailable() {
		utilities.WriteJSON(w, http.StatusServiceUnavailable, utilities.ErrorBody{Error: "QS database unavailable"})
		return
	}

	// Return OPEN timeslots (status_id=1)
	openStatus := 1
	var projectID *int64
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		v, _ := strconv.ParseInt(pid, 10, 64)
		if v > 0 {
			projectID = &v
		}
	}

	slots, _, err := h.InterviewService.ListTimeSlots(ctx, 1, 50, projectID, &openStatus, nil, nil, nil)
	if err != nil {
		slog.ErrorContext(ctx, "get available slots failed", "error", err)
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "failed to get available slots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		result = append(result, dto.SlotFromListRowSimple(s))
	}
	utilities.WriteJSON(w, http.StatusOK, result)
}

func (h *InterviewHandler) GetAISuggestions(w http.ResponseWriter, r *http.Request) {
	utilities.WriteJSON(w, http.StatusOK, map[string]any{
		"projectId": r.URL.Query().Get("projectId"),
		"suggestions": []map[string]any{
			{"suggestedStart": "2026-04-01T09:00:00Z", "suggestedEnd": "2026-04-01T09:30:00Z", "participantCount": 3},
			{"suggestedStart": "2026-04-01T14:00:00Z", "suggestedEnd": "2026-04-01T14:30:00Z", "participantCount": 5},
		},
	})
}
