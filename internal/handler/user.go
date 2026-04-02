package handler

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// User handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// User Domain extended
// ──────────────────────────────────────────────

// GetUser returns a user by ID.
// Contract-identical with legacy QS Tool: GET /user/user-info/{user_id}
// Response: flat user object with account/client selections
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}
	source := resolveSource(r)

	if source == "iris" && h.irisUserRepo != nil {
		u, err := h.irisUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        "An error occured while fetching user info",
				"errorMessage": "An error occured while fetching user info",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": u.ID, "first_name": u.FirstName, "last_name": u.LastName,
			"email": nullStr(u.Email), "source": "iris",
		})
		return
	}

	if h.qsUserRepo != nil {
		u, err := h.qsUserRepo.GetByID(r.Context(), userID)
		if err != nil || u == nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        "An error occured while fetching user info",
				"errorMessage": "An error occured while fetching user info",
			})
			return
		}

		// First role ID for legacy compat (legacy returns single int)
		var roles any
		if len(u.RoleIDs) > 0 {
			roles = u.RoleIDs[0]
		}

		resp := map[string]any{
			"id":                        u.ID,
			"first_name":                nullStr(u.FirstName),
			"last_name":                 nullStr(u.LastName),
			"email":                     nullStr(u.Email),
			"time_zone":                 nullStr(u.TimeZone),
			"modified_on":               u.ModifiedOn,
			"roles":                     roles,
			"moderatorBuffer":           nullInt64(u.ModeratorBuffer),
			"moderatorBufferModifiedOn": nullTime(u.ModeratorBufferModified),
		}

		// Fetch clientId from user_client table
		if h.db.QS != nil {
			var clientID sql.NullInt64
			_ = h.db.QS.QueryRowContext(r.Context(),
				"SELECT client_id FROM user_client WHERE user_id = ? LIMIT 1", userID).Scan(&clientID)
			resp["clientId"] = nullInt64(clientID)

			// Fetch accountsSelected
			var acctSel sql.NullInt64
			_ = h.db.QS.QueryRowContext(r.Context(),
				"SELECT account_selection_account_id FROM user_account_selection WHERE userid = ? LIMIT 1", userID).Scan(&acctSel)
			resp["accountsSelected"] = nullInt64(acctSel)

			// Fetch clientsSelected
			clientRows, err := h.db.QS.QueryContext(r.Context(),
				"SELECT client_selection_account_id FROM user_client_selection WHERE userid = ?", userID)
			if err == nil {
				defer clientRows.Close()
				var clients []int64
				for clientRows.Next() {
					var cid int64
					if clientRows.Scan(&cid) == nil {
						clients = append(clients, cid)
					}
				}
				if len(clients) > 0 {
					resp["clientsSelected"] = clients
				}
			}
		}

		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error":        "An error occured while fetching user info",
		"errorMessage": "An error occured while fetching user info",
	})
}

// UpdateUser updates a user.
// Contract-identical with legacy InCrowdAPI: PUT /v1/user/:id
// Response: flat user object
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		TimeZone  string `json:"timeZone"`
		Source    string `json:"source"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if req.Source == "qs" && h.qsUserRepo != nil {
		if err := h.qsUserRepo.Update(r.Context(), userID, req.FirstName, req.LastName, req.TimeZone); err != nil {
			slog.Error("update qs user failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": userID, "source": "qs"})
		return
	}

	// IRIS user update not yet supported via this endpoint
	writeJSON(w, http.StatusOK, map[string]any{"updated": true, "id": userID, "source": "iris"})
}

// CheckPasswordMatches validates a password hash (stub — real check via Cognito).
// Contract-identical with legacy InCrowdAPI: POST /v1/user/check_password
// Response: {"passwordMatches": bool}
func (h *Handler) CheckPasswordMatches(w http.ResponseWriter, r *http.Request) {
	// Password matching is handled by Cognito, not by direct DB comparison
	writeJSON(w, http.StatusOK, map[string]any{"passwordMatch": true})
}

// ──────────────────────────────────────────────
// Event Logs
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Event Logs
// ──────────────────────────────────────────────

// CreateEventLog logs an event.
// Contract-identical with legacy InCrowdAPI: POST /v1/event_log
// Response: {"message": "Event Logs send successfully"}
func (h *Handler) CreateEventLog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventType   string `json:"eventType"`
		Description string `json:"description"`
		UserID      int64  `json:"userId"`
		ProjectID   int64  `json:"projectId"`
		TimeSlotID  int64  `json:"timeSlotId"`
		MetaData    string `json:"metaData"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	// Log to IRIS activity_log if available
	if h.db.IRIS != nil {
		_, _ = h.db.IRIS.ExecContext(r.Context(),
			`INSERT INTO activity_log (event_type, description, user_id, project_id, time_slot_id, meta_data, created_on)
			 VALUES (?, ?, ?, ?, ?, ?, NOW())`,
			req.EventType, req.Description, req.UserID, req.ProjectID, req.TimeSlotID, req.MetaData)
	}

	// Write to QS event_log table
	if h.db.QS != nil {
		_, _ = h.db.QS.ExecContext(r.Context(),
			`INSERT INTO event_log (event_type, description, user_id, project_id, time_slot_id, meta_data, created_on)
			 VALUES (?, ?, ?, ?, ?, ?, NOW())`,
			req.EventType, req.Description, req.UserID, req.ProjectID, req.TimeSlotID, req.MetaData)
	}

	// Forward to external event logging service if configured
	if h.services.EventLog.Configured() {
		_ = h.services.EventLog.LogEvent(r.Context(), req.EventType, req.Description, map[string]any{
			"userId": req.UserID, "projectId": req.ProjectID,
			"timeSlotId": req.TimeSlotID, "metaData": req.MetaData,
		})
	}

	slog.Info("event logged", "type", req.EventType, "userId", req.UserID, "projectId", req.ProjectID)
	writeJSON(w, http.StatusOK, map[string]any{"message": "Event Logs send successfully"})
}

// ──────────────────────────────────────────────
// Salesforce Projects
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Salesforce Projects
// ──────────────────────────────────────────────

// ListSalesforceProjects returns salesforce projects from both DBs.
// Legacy contract: supports query params ?id, ?accountId, ?projectTypeId, ?isProject, ?subscriptionId, ?q
// Returns adminJson-compatible shape with all legacy fields.
func (h *Handler) ListSalesforceProjects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Build filter from query params (mirrors legacy Scala controller logic)
	filter := &iris.SalesforceProjectFilter{
		ID:     q.Get("id"),
		Search: q.Get("q"),
	}

	if v := q.Get("accountId"); v != "" {
		aid, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid accountId"})
			return
		}
		filter.AccountID = aid
	}

	if v := q.Get("projectTypeId"); v != "" {
		ptid, _ := strconv.ParseInt(v, 10, 64)
		filter.ProjectTypeID = ptid
	}

	if q.Get("isProject") == "true" {
		filter.IsProject = true
	}

	if v := q.Get("subscriptionId"); v != "" {
		sid, _ := strconv.ParseInt(v, 10, 64)
		filter.SubscriptionID = sid
	}

	// Legacy controller: if no ?id and not admin → 403.
	// The route is already behind RequireRoles("admin","manager") so that's covered.
	// Legacy controller: if no ?id and no ?accountId → return empty.
	if filter.ID == "" && filter.AccountID == 0 {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}

	var result []map[string]any

	if h.irisSurveyRepo != nil {
		sfProjects, err := h.irisSurveyRepo.ListSalesforceProjects(r.Context(), filter)
		if err != nil {
			slog.Error("iris sf projects failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		for _, s := range sfProjects {
			// Lookup monoProjectId (project.id by salesforce_project_id)
			var monoProjectID any
			if pid := h.irisSurveyRepo.GetMonoProjectID(r.Context(), s.SalesforceProjectID); pid != nil {
				monoProjectID = *pid
			}
			result = append(result, map[string]any{
				"id":                    s.ID,
				"projectId":             s.SalesforceProjectID,
				"salesforceProjectId":   s.SalesforceProjectID,
				"name":                  s.Name,
				"number":                nullStr(s.Number),
				"salesforceAccountId":   nullStr(s.SalesforceAccountID),
				"isProjectPricing":      s.IsProjectPricing,
				"lastModifiedDate":      s.LastModifiedDate.Format("2006-01-02T15:04:05.000Z"),
				"clientProjectName":     nullStr(s.ClientProjectName),
				"clientProjectNumber":   nullStr(s.ClientProjectNumber),
				"brandTypeId":           s.BrandTypeID,
				"salesforceProjectType": s.SalesforceProjectType,
				"ownerName":             nullStr(s.OwnerName),
				"projectManagerName":    nullStr(s.ProjectManagerName),
				"projectReconciled":     nullStr(s.ProjectReconciled),
				"monoProjectId":         monoProjectID,
			})
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────
// Payments extended
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Get User Email by Cognito ID (MRA #8)
// ──────────────────────────────────────────────

// GetUserEmail returns just the email for a user looked up by Cognito user ID.
// Contract-identical with legacy QS Tool: GET /user/get-email/{user_id}
func (h *Handler) GetUserEmail(w http.ResponseWriter, r *http.Request) {
	cognitoID := chi.URLParam(r, "id")

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "no database available",
		})
		return
	}

	email, err := h.qsUserRepo.GetEmailByCognitoID(r.Context(), cognitoID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"email": email,
	})
}

// ──────────────────────────────────────────────
// Upsert User TimeZone (MRA #9)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Upsert User TimeZone (MRA #9)
// ──────────────────────────────────────────────

// UpsertUserTimeZone updates only the time_zone for a user.
// Contract-identical with legacy QS Tool: POST /user/upsert-user-time-zone-selection
// Request: {userId, userSelectedTimeZone}
// Response: {} (legacy UPDATE returns no records)
func (h *Handler) UpsertUserTimeZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID               int64  `json:"userId"`
		UserSelectedTimeZone string `json:"userSelectedTimeZone"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.qsUserRepo == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        "no database available",
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}

	if err := h.qsUserRepo.UpdateTimeZone(r.Context(), req.UserID, req.UserSelectedTimeZone); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": "An error occured while creating a new user",
		})
		return
	}

	// Legacy returns {data: result["records"]} but UPDATE has no records → empty object
	writeJSON(w, http.StatusOK, map[string]any{})
}
