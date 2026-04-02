package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// Moderator handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Moderators (real dual-DB)
// ──────────────────────────────────────────────

func (h *Handler) ListModerators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var combined []map[string]any

	// QS moderators (role_id = 1)
	if h.qsUserRepo != nil {
		mods, err := h.qsUserRepo.GetModerators(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "list QS moderators failed", "error", err)
		} else {
			for _, m := range mods {
				roles := []string{}
				for _, rid := range parseRoleCSV(m.RoleIDs) {
					roles = append(roles, qsRoleName(rid))
				}
				combined = append(combined, map[string]any{
					"id":              m.ID,
					"name":            strings.TrimSpace(m.FirstName.String + " " + m.LastName.String),
					"email":           m.Email.String,
					"role":            "moderator",
					"roles":           roles,
					"status":          "active",
					"source":          "qs",
					"serviceCategory": "MRA",
					"timezone":        m.TimeZone.String,
					"updatedAt":       m.ModifiedOn.Format(time.RFC3339),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, combined)
}

func (h *Handler) CreateModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"    validate:"required,email"`
		TimeZone  string `json:"timeZone"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	if req.TimeZone == "" {
		req.TimeZone = "America/New_York"
	}
	// Role 1 = Moderator
	uid, err := h.qsUserRepo.Create(ctx, req.FirstName, req.LastName, req.Email, req.TimeZone, []int{1})
	if err != nil {
		slog.Error("create moderator", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create moderator"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        uid,
		"firstName": req.FirstName,
		"lastName":  req.LastName,
		"email":     req.Email,
		"role":      "moderator",
		"status":    "active",
		"createdAt": now(),
	})
}

func (h *Handler) GetModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		source = "qs" // default to QS for moderators
	}

	switch source {
	case "qs":
		if h.qsUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
			return
		}
		u, err := h.qsUserRepo.GetByID(ctx, nid)
		if err != nil {
			slog.ErrorContext(ctx, "get QS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "moderator not found"})
			return
		}
		roles := []string{}
		for _, rid := range u.RoleIDs {
			roles = append(roles, qsRoleName(rid))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":              u.ID,
			"firstName":       u.FirstName.String,
			"lastName":        u.LastName.String,
			"email":           u.Email.String,
			"roles":           roles,
			"timezone":        u.TimeZone.String,
			"moderatorBuffer": u.ModeratorBuffer.Int64,
			"termsAccepted":   u.TermsAccepted == 1,
			"source":          "qs",
			"serviceCategory": "MRA",
			"modifiedOn":      u.ModifiedOn.Format(time.RFC3339),
		})
	case "iris":
		if h.irisUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "IRIS database unavailable"})
			return
		}
		u, err := h.irisUserRepo.GetByID(ctx, nid)
		if err != nil {
			slog.ErrorContext(ctx, "get IRIS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "moderator not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":              u.ID,
			"firstName":       u.FirstName,
			"lastName":        u.LastName,
			"email":           u.Email.String,
			"roles":           u.RoleNames,
			"timezone":        u.TimeZone.String,
			"source":          "iris",
			"serviceCategory": "LS",
			"lastLogin":       u.LastLogin.Time.Format(time.RFC3339),
			"registeredAt":    u.RegistrationDate.Format(time.RFC3339),
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid source, use qs or iris"})
	}
}

func (h *Handler) UpdateModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var body struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		TimeZone  string `json:"timezone"`
		Source    string `json:"source"`
	}
	if errs := validate.DecodeAndValidate(r, &body); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if body.Source == "" {
		body.Source = "qs"
	}

	switch body.Source {
	case "qs":
		if h.qsUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
			return
		}
		if err := h.qsUserRepo.Update(ctx, nid, body.FirstName, body.LastName, body.TimeZone); err != nil {
			slog.ErrorContext(ctx, "update QS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
	case "iris":
		if h.irisUserRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "IRIS database unavailable"})
			return
		}
		if err := h.irisUserRepo.Update(ctx, nid, body.FirstName, body.LastName, body.TimeZone); err != nil {
			slog.ErrorContext(ctx, "update IRIS moderator failed", "error", err, "id", nid)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid source"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": nid})
}

func (h *Handler) DeleteModerator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"error":        "QS database unavailable",
			"errorMessage": "An error occured while removing the user",
		})
		return
	}
	modID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing the user",
		})
		return
	}
	if err := h.qsUserRepo.SoftDelete(ctx, modID); err != nil {
		slog.Error("delete moderator", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while removing the user",
		})
		return
	}
	// Legacy returns data-api-client UPDATE result shape
	writeJSON(w, http.StatusOK, map[string]any{"numberOfRecordsUpdated": 1})
}

func (h *Handler) BulkUploadModerators(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"created": 0, "failed": 0, "errors": []any{}, "message": "bulk upload not yet implemented",
	})
}

func (h *Handler) GetModeratorTimeslots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.qsTimeSlotRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	modID, err := validate.ParseIDParam(r, "moderatorId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	slots, total, err := h.qsTimeSlotRepo.ListByModerator(ctx, modID, page, pageSize)
	if err != nil {
		slog.ErrorContext(ctx, "get moderator timeslots failed", "error", err, "moderatorId", modID)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to get moderator timeslots"})
		return
	}

	result := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		item := map[string]any{
			"id":          s.ID,
			"projectId":   s.ProjectID,
			"projectName": s.ProjectName,
			"startTime":   s.StartTime.Format(time.RFC3339),
			"endTime":     s.EndTime.Format(time.RFC3339),
			"duration":    s.Duration,
			"statusId":    s.StatusID,
			"status":      s.StatusName,
			"confirmed":   s.Confirmed,
			"source":      "qs",
		}
		if s.ResponderID.Valid {
			item["responderId"] = s.ResponderID.Int64
			item["responderName"] = s.ResponderName.String
		}
		result = append(result, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta":    map[string]any{"page": page, "pageSize": pageSize, "totalCount": total},
	})
}

// ──────────────────────────────────────────────
// Moderator Availability (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var clientID *int64
	if cid := r.URL.Query().Get("clientId"); cid != "" {
		v, err := strconv.ParseInt(cid, 10, 64)
		if err == nil {
			clientID = &v
		}
	}
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	avails, err := h.qsUserRepo.ListModeratorAvailability(ctx, nid, clientID, startDate, endDate)
	if err != nil {
		slog.ErrorContext(ctx, "list moderator availability failed", "error", err, "moderatorId", nid)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to fetch availability"})
		return
	}

	items := make([]map[string]any, 0, len(avails))
	for _, a := range avails {
		items = append(items, map[string]any{
			"id":          a.ID,
			"moderatorId": a.ModeratorID,
			"clientId":    a.ClientID,
			"startTime":   a.StartTime.Format(time.RFC3339),
			"endTime":     a.EndTime.Format(time.RFC3339),
			"isImported":  false,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"availabilities": items})
}

func (h *Handler) PostModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	var body struct {
		ClientID  int64  `json:"clientId"`
		StartTime string `json:"startTime"`
		EndTime   string `json:"endTime"`
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

	avail, err := h.qsUserRepo.CreateModeratorAvailability(ctx, nid, body.ClientID, st, et)
	if err != nil {
		slog.ErrorContext(ctx, "create moderator availability failed", "error", err, "moderatorId", nid)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to create availability"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          avail.ID,
		"moderatorId": avail.ModeratorID,
		"startTime":   avail.StartTime.Format(time.RFC3339),
		"endTime":     avail.EndTime.Format(time.RFC3339),
	})
}

func (h *Handler) DeleteModeratorAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid, err := validate.ParseIDParam(r, "availabilityId")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "QS database unavailable"})
		return
	}

	if err := h.qsUserRepo.DeleteModeratorAvailability(ctx, nid); err != nil {
		slog.ErrorContext(ctx, "delete moderator availability failed", "error", err, "availabilityId", nid)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to delete availability"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "availabilityId": nid})
}

func (h *Handler) GetTimeslotModeratorOptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"availableModerators": []map[string]any{
			{"id": "mod-201", "firstName": "Jane", "lastName": "Smith",
				"hasConflict": false, "availabilityId": 801},
			{"id": "mod-202", "firstName": "Bob", "lastName": "Wilson",
				"hasConflict": true, "conflictReason": "Overlapping interview at 14:00-14:30"},
		},
	})
}

// ──────────────────────────────────────────────
// Conference (LLD endpoints)
// ──────────────────────────────────────────────
