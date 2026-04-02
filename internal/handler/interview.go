package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// InterviewHandler handles timeslot and interview scheduling endpoints.
type InterviewHandler struct{ *Deps }

// ──────────────────────────────────────────────
// Timeslots
// ──────────────────────────────────────────────

func (h *InterviewHandler) ListTimeslots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
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

	slots, total, err := h.qsTimeSlotRepo.List(ctx, page, pageSize, projectID, statusID, moderatorID, fromTime, toTime)
	if err != nil {
		slog.ErrorContext(ctx, "list timeslots failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to list timeslots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		item := map[string]any{
			"id":                     s.ID,
			"projectId":              s.ProjectID,
			"projectName":            s.ProjectName,
			"startTime":              s.StartTime.Format(time.RFC3339),
			"endTime":                s.EndTime.Format(time.RFC3339),
			"duration":               s.Duration,
			"statusId":               s.StatusID,
			"status":                 s.StatusName,
			"confirmed":              s.Confirmed,
			"isInvalid":              s.IsInvalid,
			"isInvalidatedInterview": s.IsInvalidatedInterview,
			"source":                 "qs",
			"serviceCategory":        "MRA",
			"modifiedOn":             s.ModifiedOn.Format(time.RFC3339),
		}
		if s.ModeratorID.Valid {
			item["moderatorId"] = s.ModeratorID.Int64
			item["moderatorName"] = s.ModeratorName.String
			item["isHost"] = s.IsHost.Valid && s.IsHost.Bool
		}
		if s.ResponderID.Valid {
			item["responderId"] = s.ResponderID.Int64
			item["responderName"] = s.ResponderName.String
		}
		if s.ConferenceHash.Valid {
			item["conferenceHash"] = s.ConferenceHash.String
		}
		if s.InvalidationReasonCode.Valid {
			item["invalidationReasonCode"] = s.InvalidationReasonCode.String
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalCount": total,
		},
	})
}

func (h *InterviewHandler) CreateTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		ProjectID   int64  `json:"projectId"  validate:"required,gt=0"`
		StartTime   string `json:"startTime"  validate:"required"`
		EndTime     string `json:"endTime"    validate:"required"`
		Duration    int    `json:"duration"`
		ModeratorID int64  `json:"moderatorId"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	st, err := time.Parse(time.RFC3339, body.StartTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid startTime format, use RFC3339"})
		return
	}
	et, err := time.Parse(time.RFC3339, body.EndTime)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid endTime format, use RFC3339"})
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

	tsID, err := h.qsTimeSlotRepo.Create(ctx, ts)
	if err != nil {
		slog.ErrorContext(ctx, "create timeslot failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create timeslot"})
		return
	}

	// Assign moderator if provided
	if body.ModeratorID > 0 {
		if _, err := h.qsTimeSlotRepo.AssignModerator(ctx, body.ModeratorID, tsID, true); err != nil {
			slog.ErrorContext(ctx, "assign moderator failed", "error", err, "timeSlotId", tsID, "moderatorId", body.ModeratorID)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": tsID, "projectId": body.ProjectID, "statusId": 1, "source": "qs"})
}

func (h *InterviewHandler) GetTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	ts, err := h.qsTimeSlotRepo.GetByID(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
		return
	}
	if ts == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "timeslot not found"})
		return
	}

	result := map[string]any{
		"id":                     ts.ID,
		"projectId":              ts.ProjectID,
		"startTime":              ts.StartTime.Format(time.RFC3339),
		"endTime":                ts.EndTime.Format(time.RFC3339),
		"confirmed":              ts.Confirmed,
		"statusId":               ts.StatusID,
		"duration":               ts.Duration,
		"isInvalid":              ts.IsInvalid,
		"isInvalidatedInterview": ts.IsInvalidatedInterview,
		"isPreviousNoShow":       ts.IsPreviousNoShow,
		"source":                 "qs",
		"serviceCategory":        "MRA",
		"modifiedOn":             ts.ModifiedOn.Format(time.RFC3339),
	}
	if ts.ConferenceHash.Valid {
		result["conferenceHash"] = ts.ConferenceHash.String
	}
	if ts.ParticipantHash.Valid {
		result["participantHash"] = ts.ParticipantHash.String
	}
	if ts.InvalidationReasonCode.Valid {
		result["invalidationReasonCode"] = ts.InvalidationReasonCode.String
	}
	if ts.InvalidationReasonText.Valid {
		result["invalidationReasonText"] = ts.InvalidationReasonText.String
	}

	// Get assigned moderators
	mods, err := h.qsTimeSlotRepo.GetModerators(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot moderators failed", "error", err)
	}
	if mods != nil {
		modList := make([]map[string]any, 0, len(mods))
		for _, m := range mods {
			modList = append(modList, map[string]any{
				"moderatorId": m.ModeratorID,
				"isHost":      m.IsHost,
			})
		}
		result["moderators"] = modList
	}

	// Get linked respondent
	resp, err := h.qsTimeSlotRepo.GetRespondent(ctx, tsID)
	if err != nil {
		slog.ErrorContext(ctx, "get timeslot respondent failed", "error", err)
	}
	if resp != nil {
		result["respondent"] = map[string]any{
			"id":        resp.ID,
			"firstName": resp.FirstName,
			"lastName":  resp.LastName,
			"timeZone":  resp.TimeZone.String,
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *InterviewHandler) UpdateTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body struct {
		StatusID  *int   `json:"statusId"`
		Confirmed *bool  `json:"confirmed"`
		IsInvalid *int   `json:"isInvalid"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
		Duration  *int   `json:"duration"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
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

	if err := h.qsTimeSlotRepo.Update(ctx, tsID, fields); err != nil {
		slog.ErrorContext(ctx, "update timeslot failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": tsID, "updated": true, "source": "qs"})
}

func (h *InterviewHandler) DeleteTimeslot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	tsID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if err := h.qsTimeSlotRepo.Delete(ctx, tsID); err != nil {
		slog.ErrorContext(ctx, "delete timeslot failed", "error", err, "id", tsID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": tsID})
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
	writeJSON(w, http.StatusOK, slots)
}

func (h *InterviewHandler) GetAvailableSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
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

	slots, _, err := h.qsTimeSlotRepo.List(ctx, 1, 50, projectID, &openStatus, nil, nil, nil)
	if err != nil {
		slog.ErrorContext(ctx, "get available slots failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get available slots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		item := map[string]any{
			"id":        s.ID,
			"projectId": s.ProjectID,
			"startTime": s.StartTime.Format(time.RFC3339),
			"endTime":   s.EndTime.Format(time.RFC3339),
			"duration":  s.Duration,
		}
		if s.ModeratorID.Valid {
			item["moderatorId"] = s.ModeratorID.Int64
			item["moderatorName"] = s.ModeratorName.String
		}
		result = append(result, item)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *InterviewHandler) GetAISuggestions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"projectId": r.URL.Query().Get("projectId"),
		"suggestions": []map[string]any{
			{"suggestedStart": "2026-04-01T09:00:00Z", "suggestedEnd": "2026-04-01T09:30:00Z", "participantCount": 3},
			{"suggestedStart": "2026-04-01T14:00:00Z", "suggestedEnd": "2026-04-01T14:30:00Z", "participantCount": 5},
		},
	})
}
