package ls

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"
	"github.com/InCrowd/unified-qual-api/internal/dto"

	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ──────────────────────────────────────────────
// LS Project handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Project Sub-resources
// ──────────────────────────────────────────────

// GetProjectSurveys returns surveys for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id/surveys
// Response: {"surveys": [...], "limit": N, "offset": N, "totalCount": N}
func (h *Handler) GetProjectSurveys(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := support.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		surveys, err := h.IrisSurveyRepo.ListSurveysForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project surveys failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(surveys))
		for _, s := range surveys {
			result = append(result, map[string]any{
				"id": s.ID, "namePublic": s.NamePublic, "status": s.Status,
				"surveyTypeId": s.SurveyTypeID, "completionsNeeded": s.CompletionsNeeded,
				"createdOn": s.CreatedOn.Format(time.RFC3339), "source": "iris",
			})
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"surveys": result, "limit": len(result), "offset": 0, "totalCount": len(result),
		})
		return
	}

	// QS: list native surveys by project
	if h.QsAnswerRepo != nil {
		surveys, err := h.QsAnswerRepo.ListNativeSurveysByProject(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project surveys failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(surveys))
		for _, s := range surveys {
			result = append(result, map[string]any{
				"id": s.ID, "projectId": s.ProjectID,
				"createdOn": s.CreatedOn.Format(time.RFC3339), "source": "qs",
			})
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"surveys": result, "limit": len(result), "offset": 0, "totalCount": len(result),
		})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{
		"surveys": []any{}, "limit": 0, "offset": 0, "totalCount": 0,
	})
}

// GetProjectTimeSlots returns timeslots for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id/time_slots
// Response: {"timeSlots": [...]}
func (h *Handler) GetProjectTimeSlots(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.QsTimeSlotRepo != nil {
		slots, _, err := h.QsTimeSlotRepo.List(r.Context(), 1, 500, &projectID, nil, nil, nil, nil)
		if err != nil {
			slog.Error("project timeslots failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(slots))
		for _, s := range slots {
			result = append(result, map[string]any{
				"id": s.ID, "projectId": s.ProjectID, "startTime": s.StartTime.Format(time.RFC3339),
				"endTime": s.EndTime.Format(time.RFC3339), "duration": s.Duration,
				"statusId": s.StatusID, "statusName": s.StatusName,
			})
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"timeSlots": result,
		})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"timeSlots": []any{}})
}

// GetProjectUsers returns users assigned to a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id/users
// Response: {"users": [...], "offset": N, "limit": N, "totalCount": N}
func (h *Handler) GetProjectUsers(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := support.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		users, err := h.IrisSurveyRepo.ListUserProjects(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project users failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"users": users, "offset": 0, "limit": len(users), "totalCount": len(users),
		})
		return
	}

	if h.QsAnswerRepo != nil {
		users, err := h.QsAnswerRepo.ListProjectsUsers(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project users failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"users": users, "offset": 0, "limit": len(users), "totalCount": len(users),
		})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{
		"users": []any{}, "offset": 0, "limit": 0, "totalCount": 0,
	})
}

// GetProjectObservers returns observers for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:pid/observers
// Response: {"observers": [...]}
func (h *Handler) GetProjectObservers(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.IrisSurveyRepo != nil {
		observers, err := h.IrisSurveyRepo.ListObserversForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("get observers failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(observers))
		for _, o := range observers {
			result = append(result, map[string]any{
				"id": o.ID, "projectId": o.ProjectID, "email": o.Email,
				"timeSlotId": dto.NullInt64(o.TimeSlotID),
			})
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"observers": result})
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"observers": []any{}})
}

// GetProjectQualReschedBody returns the reschedule email template body.
// Legacy contract: {"html": "<rendered email HTML>"}
func (h *Handler) GetProjectQualReschedBody(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.IrisSurveyRepo != nil {
		body, err := h.IrisSurveyRepo.GetQualRescheduleBody(r.Context(), projectID)
		if err != nil {
			slog.Error("get qual resched body failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"html": body})
		return
	}

	// QS: get template by name
	if h.QsAnswerRepo != nil {
		t, _ := h.QsAnswerRepo.GetCommunicationTemplate(r.Context(), "reschedule")
		if t != nil {
			support.WriteJSON(w, http.StatusOK, map[string]any{"html": t.Body})
			return
		}
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"html": ""})
}

// GetProjectAvailability returns moderator availability for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:pid/availability
// Response: flat array of availability objects
func (h *Handler) GetProjectAvailability(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.IrisSurveyRepo != nil && support.ResolveSource(r) == "iris" {
		avails, err := h.IrisSurveyRepo.GetProjectAvailability(r.Context(), projectID)
		if err != nil {
			slog.Error("project availability failed", "error", err)
		}
		result := make([]map[string]any, 0, len(avails))
		for _, a := range avails {
			result = append(result, map[string]any{
				"id": a.ID, "moderatorId": a.ModeratorID,
				"startTime": a.StartTime.Format(time.RFC3339),
				"endTime":   a.EndTime.Format(time.RFC3339),
			})
		}
		support.WriteJSON(w, http.StatusOK, result)
		return
	}

	// QS: get moderator availability for this project's moderators
	if h.QsUserRepo != nil && h.QsProjectRepo != nil {
		proj, _ := h.QsProjectRepo.GetByID(r.Context(), projectID)
		clientID := int64(0)
		if proj != nil && proj.ClientID.Valid {
			clientID = proj.ClientID.Int64
		}
		avails, err := h.QsUserRepo.ListModeratorAvailability(r.Context(), 0, &clientID, "", "")
		if err != nil {
			slog.Error("get qs project avail failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, avails)
		return
	}
	support.WriteJSON(w, http.StatusOK, []any{})
}

// GetProjectSchedulerModerators returns moderators for the project scheduler.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:pid/scheduler/moderators
// Response: {"moderatorInfo": {...}}
func (h *Handler) GetProjectSchedulerModerators(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.IrisSurveyRepo != nil && support.ResolveSource(r) == "iris" {
		mods, err := h.IrisSurveyRepo.GetSchedulerModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("scheduler mods failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"moderatorInfo": mods})
		return
	}

	// QS: get moderators from timeslots for this project
	if h.QsTimeSlotRepo != nil {
		tsRows, _, _ := h.QsTimeSlotRepo.ListByProject(r.Context(), projectID, 1, 1000)
		modMap := map[int64]map[string]any{}
		for _, ts := range tsRows {
			mods, _ := h.QsTimeSlotRepo.GetModerators(r.Context(), ts.ID)
			for _, m := range mods {
				if _, ok := modMap[m.ModeratorID]; !ok {
					modMap[m.ModeratorID] = map[string]any{
						"id": m.ModeratorID, "isHost": m.IsHost,
					}
				}
			}
		}
		result := make([]map[string]any, 0, len(modMap))
		for _, v := range modMap {
			result = append(result, v)
		}
		support.WriteJSON(w, http.StatusOK, result)
		return
	}
	support.WriteJSON(w, http.StatusOK, []map[string]any{})
}

// GetProjectDashboard returns combined availability and timeslot data for dashboard.
// GetProjectDashboard returns dashboard data for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/dashboard/availability_and_time_slots
// Response: {"scheduled": N, "completed": N, "moderatorInfo": {"<modId>": {id, firstName, lastName, interviewCount}}}
func (h *Handler) GetProjectDashboard(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "pid")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if h.IrisSurveyRepo != nil {
		data, err := h.IrisSurveyRepo.GetProjectDashboardInfo(r.Context(), projectID)
		if err != nil {
			slog.Error("project dashboard failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		support.WriteJSON(w, http.StatusOK, data)
		return
	}
	support.WriteJSON(w, http.StatusOK, map[string]any{"scheduled": 0, "completed": 0, "moderatorInfo": map[string]any{}})
}

func (h *Handler) ResetProjectModerators(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "projectId")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := support.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		count, err := h.IrisSurveyRepo.ResetProjectModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("reset project mods failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "reset failed"})
			return
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "removedAssignments": count, "source": "iris"})
		return
	}

	if h.DB.QS != nil {
		res, err := h.DB.QS.ExecContext(r.Context(),
			`DELETE mts FROM moderator_time_slot mts
			 INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
			 WHERE ts.project_id = ? AND ts.status_id IN (1, 2)`, projectID)
		if err != nil {
			slog.Error("qs reset mods failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "reset failed"})
			return
		}
		n, _ := res.RowsAffected()
		support.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "removedAssignments": n, "source": "qs"})
		return
	}
	support.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

// ──────────────────────────────────────────────
// Export Project Data (MRA #25)
// ──────────────────────────────────────────────

func (h *Handler) HandleProjectExport(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "projectId")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	format := r.URL.Query().Get("format")
	source := support.ResolveSource(r)

	var data []map[string]any

	if source == "iris" && h.IrisSurveyRepo != nil {
		data, err = h.IrisSurveyRepo.ExportProjectData(r.Context(), projectID)
		if err != nil {
			slog.Error("iris export failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "export failed"})
			return
		}
	} else if h.QsTimeSlotRepo != nil {
		slots, _, err := h.QsTimeSlotRepo.List(r.Context(), 1, 10000, &projectID, nil, nil, nil, nil)
		if err != nil {
			slog.Error("qs export failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "export failed"})
			return
		}
		for _, s := range slots {
			data = append(data, map[string]any{
				"timeSlotId": s.ID, "startTime": s.StartTime.Format(time.RFC3339),
				"endTime": s.EndTime.Format(time.RFC3339), "statusId": s.StatusID,
				"status": s.StatusName, "moderator": dto.NullStr(s.ModeratorName),
				"respondent": dto.NullStr(s.ResponderName),
			})
		}
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=project_%d_export.csv", projectID))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"TimeSlotID", "StartTime", "EndTime", "Status", "Moderator", "Respondent"})
		for _, row := range data {
			_ = cw.Write([]string{
				fmt.Sprint(row["timeSlotId"]), fmt.Sprint(row["startTime"]),
				fmt.Sprint(row["endTime"]), fmt.Sprint(row["status"]),
				fmt.Sprint(row["moderator"]), fmt.Sprint(row["respondent"]),
			})
		}
		cw.Flush()
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "rows": data, "count": len(data)})
}

// ──────────────────────────────────────────────
// Moderators Count & Unavailable (MRA #27, #29)
// ──────────────────────────────────────────────

func (h *Handler) GetAvailableModeratorsCount(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "projectId")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := support.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		count, err := h.IrisSurveyRepo.GetAvailableModeratorsCount(r.Context(), projectID)
		if err != nil {
			slog.Error("count mods failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "availableCount": count, "source": "iris"})
		return
	}

	if h.DB.QS != nil {
		var count int
		_ = h.DB.QS.QueryRowContext(r.Context(),
			`SELECT COUNT(DISTINCT mts.moderator_id) FROM moderator_time_slot mts
			 INNER JOIN time_slot ts ON ts.id = mts.time_slot_id
			 WHERE ts.project_id = ? AND ts.status_id = 1`, projectID).Scan(&count)
		support.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "availableCount": count, "source": "qs"})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "availableCount": 0})
}

func (h *Handler) GetUnavailableModerators(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "projectId")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	source := support.ResolveSource(r)

	if source == "iris" && h.IrisSurveyRepo != nil {
		mods, err := h.IrisSurveyRepo.GetUnavailableModerators(r.Context(), projectID)
		if err != nil {
			slog.Error("get unavail mods failed", "error", err)
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{"moderators": mods, "source": "iris"})
		return
	}

	support.WriteJSON(w, http.StatusOK, map[string]any{"moderators": []any{}, "source": "qs"})
}

// ──────────────────────────────────────────────
// Project Managers (MRA #54)
// ──────────────────────────────────────────────

func (h *Handler) ListProjectManagers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := support.ParsePagination(r)
	source := support.ResolveSource(r)

	if (source == "" || source == "qs") && h.QsUserRepo != nil {
		managerRoleID := 2
		users, total, err := h.QsUserRepo.List(r.Context(), page, pageSize, &managerRoleID, "")
		if err != nil {
			slog.Error("list PMs failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
			return
		}
		result := make([]map[string]any, 0, len(users))
		for _, u := range users {
			result = append(result, map[string]any{
				"id": u.ID, "firstName": u.FirstName.String,
				"lastName": u.LastName.String, "email": u.Email.String,
				"source": "qs", "serviceCategory": "MRA",
			})
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"success": true, "data": result, "meta": map[string]any{"totalCount": total},
		})
		return
	}

	if source == "iris" && h.IrisUserRepo != nil {
		users, total, err := h.IrisUserRepo.List(r.Context(), page, pageSize, nil, "")
		if err != nil {
			slog.Error("list IRIS PMs failed", "error", err)
		}
		result := make([]map[string]any, 0)
		for _, u := range users {
			if strings.Contains(u.RoleNames, "Project Manager") || strings.Contains(u.RoleNames, "ADMIN") {
				result = append(result, map[string]any{
					"id": u.ID, "firstName": u.FirstName, "lastName": u.LastName,
					"email": u.Email.String, "source": "iris", "serviceCategory": "LS",
				})
			}
		}
		support.WriteJSON(w, http.StatusOK, map[string]any{
			"success": true, "data": result, "meta": map[string]any{"totalCount": total},
		})
		return
	}

	support.WriteJSON(w, http.StatusOK, []any{})
}
