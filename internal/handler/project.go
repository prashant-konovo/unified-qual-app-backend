package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ProjectHandler handles project CRUD endpoints.
type ProjectHandler struct{ *Deps }

// ──────────────────────────────────────────────
// Projects — Real dual-DB queries (IRIS + QS)
// ──────────────────────────────────────────────

// irisStatusName maps IRIS project_status_id to name.
var irisStatusName = map[int]string{
	1: "Inquiry", 2: "Defining", 3: "In Progress", 4: "Complete", 6: "Paused", 7: "Finalizing",
}

func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	log := slog.With("handler", "ListProjects")

	pg := validate.ParsePagination(r, 20, 100)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	source := r.URL.Query().Get("source") // "iris", "qs", or "" (both)

	// Support serviceCategory filter (LS→iris, MRA→qs)
	if sc := strings.ToUpper(r.URL.Query().Get("serviceCategory")); sc != "" {
		switch sc {
		case "LS":
			source = "iris"
		case "MRA":
			source = "qs"
		}
	}

	var statusID *int
	if s := r.URL.Query().Get("statusId"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			statusID = &v
		}
	}

	var result []map[string]any

	// IRIS projects
	if source == "" || source == "iris" {
		if h.irisProjectRepo != nil {
			irisProjects, irisTotal, err := h.irisProjectRepo.List(r.Context(), pg.Page, pg.PageSize, statusID, search)
			if err != nil {
				log.Error("iris project list failed", "error", err)
			} else {
				for _, p := range irisProjects {
					result = append(result, map[string]any{
						"id":                  p.ID,
						"name":                p.Name,
						"description":         nullStr(p.Description),
						"subscriptionId":      p.SubscriptionID,
						"subscriptionCompany": nullStr(p.SubscriptionCompany),
						"statusId":            p.ProjectStatusID,
						"status":              p.ProjectStatusName,
						"projectTypeId":       p.ProjectTypeID,
						"salesforceProjectId": nullStr(p.SalesforceProjectID),
						"isArchived":          p.IsArchived,
						"createdAt":           p.CreatedOn.Format(time.RFC3339),
						"modifiedAt":          nullTime(p.ModifiedOn),
						"source":              "iris",
						"serviceCategory":     "LS",
					})
				}
				_ = irisTotal
			}
		}
	}

	// QS projects
	if source == "" || source == "qs" {
		if h.qsProjectRepo != nil {
			qsProjects, qsTotal, err := h.qsProjectRepo.List(r.Context(), pg.Page, pg.PageSize, statusID, search)
			if err != nil {
				log.Error("qs project list failed", "error", err)
			} else {
				for _, p := range qsProjects {
					result = append(result, map[string]any{
						"id":                  p.ID,
						"name":                p.Name,
						"salesforceJobNumber": nullStr(p.SalesforceJobNumber),
						"clientId":            nullInt64(p.ClientID),
						"clientCompany":       nullStr(p.ClientCompany),
						"sampleSize":          nullInt64(p.SampleSize),
						"interviewLength":     nullInt64(p.InterviewLength),
						"statusId":            p.ProjectStatusID,
						"status":              p.ProjectStatusName,
						"scheduledCount":      p.ScheduledCount,
						"completedCount":      p.CompletedCount,
						"createdAt":           p.CreatedOn.Format(time.RFC3339),
						"modifiedAt":          nullTime(p.ModifiedOn),
						"source":              "qs",
						"serviceCategory":     "MRA",
					})
				}
				_ = qsTotal
			}
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    result,
		"meta": map[string]any{
			"page":       pg.Page,
			"pageSize":   pg.PageSize,
			"totalCount": len(result),
		},
	})
}

type createProjectRequest struct {
	Name                string  `json:"name"                validate:"required"`
	Description         string  `json:"description"`
	SubscriptionID      int64   `json:"subscriptionId"`
	SalesforceProjectID string  `json:"salesforceProjectId"`
	SalesforceJobNumber string  `json:"salesforceJobNumber"`
	SampleSize          int64   `json:"sampleSize"`
	InterviewLength     int64   `json:"interviewLength"`
	ClientID            int64   `json:"clientId"`
	PostScreeninBuffer  float64 `json:"postScreeninBuffer"`
	ModeratorBuffer     float64 `json:"moderatorBuffer"`
	Source              string  `json:"source"` // "iris" or "qs"
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	if req.Source == "" {
		req.Source = "qs" // default to QS
	}

	user := middleware.GetUser(r)

	if req.Source == "iris" && h.irisProjectRepo != nil {
		p := &iris.Project{
			Name:                req.Name,
			Description:         toNullStr(req.Description),
			SubscriptionID:      req.SubscriptionID,
			SalesforceProjectID: toNullStr(req.SalesforceProjectID),
			ProjectStatusID:     2, // Defining
		}
		id, err := h.irisProjectRepo.Create(r.Context(), p)
		if err != nil {
			slog.Error("iris project create failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create project"})
			return
		}
		writeJSON(w, http.StatusCreated, dto.MutationResult{ID: id, Source: "iris", ServiceCategory: "LS"})
		return
	}

	if h.qsProjectRepo != nil {
		p := &qs.Project{
			Name:                req.Name,
			SalesforceJobNumber: toNullStr(req.SalesforceJobNumber),
			SampleSize:          toNullInt64(req.SampleSize),
			InterviewLength:     toNullInt64(req.InterviewLength),
			ClientID:            toNullInt64(req.ClientID),
			PostScreeninBuffer:  toNullStr(fmt.Sprintf("%.2f", req.PostScreeninBuffer)),
			ModeratorBuffer:     toNullStr(fmt.Sprintf("%.2f", req.ModeratorBuffer)),
		}
		_ = user // TODO: set CreatedBy from user lookup
		id, err := h.qsProjectRepo.Create(r.Context(), p)
		if err != nil {
			slog.Error("qs project create failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create project"})
			return
		}
		writeJSON(w, http.StatusCreated, dto.MutationResult{ID: id, Source: "qs", ServiceCategory: "MRA"})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "no database available"})
}

// GetProject returns a project by ID.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id
// Response: flat project adminJson object
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	source := r.URL.Query().Get("source")

	// Try QS first (or if source=qs)
	if (source == "" || source == "qs") && h.qsProjectRepo != nil {
		p, err := h.qsProjectRepo.GetByID(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project get failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "database error"})
			return
		}
		if p != nil {
			scheduled, completed, _ := h.qsProjectRepo.TimeSlotCounts(r.Context(), projectID)
			topics, _ := h.qsProjectRepo.GetTopics(r.Context(), projectID)
			topicNames := make([]string, 0, len(topics))
			for _, t := range topics {
				topicNames = append(topicNames, t.TopicName)
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"id":                  p.ID,
				"name":                p.Name,
				"externalSurveyId":    nullStr(p.ExternalSurveyID),
				"salesforceJobNumber": nullStr(p.SalesforceJobNumber),
				"clientId":            nullInt64(p.ClientID),
				"sampleSize":          nullInt64(p.SampleSize),
				"interviewLength":     nullInt64(p.InterviewLength),
				"statusId":            p.ProjectStatusID,
				"schedulerGenerated":  p.SchedulerGenerated,
				"postScreeninBuffer":  nullStr(p.PostScreeninBuffer),
				"moderatorBuffer":     nullStr(p.ModeratorBuffer),
				"topics":              topicNames,
				"scheduledCount":      scheduled,
				"completedCount":      completed,
				"createdAt":           p.CreatedOn.Format(time.RFC3339),
				"modifiedAt":          nullTime(p.ModifiedOn),
				"source":              "qs",
				"serviceCategory":     "MRA",
			})
			return
		}
	}

	// Try IRIS (or if source=iris)
	if (source == "" || source == "iris") && h.irisProjectRepo != nil {
		p, err := h.irisProjectRepo.GetByID(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project get failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "database error"})
			return
		}
		if p != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":                  p.ID,
				"name":                p.Name,
				"description":         nullStr(p.Description),
				"subscriptionId":      p.SubscriptionID,
				"statusId":            p.ProjectStatusID,
				"status":              irisStatusName[p.ProjectStatusID],
				"projectTypeId":       p.ProjectTypeID,
				"salesforceProjectId": nullStr(p.SalesforceProjectID),
				"isPrivate":           p.IsPrivate,
				"isArchived":          p.IsArchived,
				"createdAt":           p.CreatedOn.Format(time.RFC3339),
				"modifiedAt":          nullTime(p.ModifiedOn),
				"source":              "iris",
				"serviceCategory":     "LS",
			})
			return
		}
	}

	writeJSON(w, http.StatusNotFound, dto.ErrorBody{Error: "project not found"})
}

type updateProjectRequest struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	StatusID            *int   `json:"statusId"`
	SalesforceProjectID string `json:"salesforceProjectId"`
	SampleSize          *int64 `json:"sampleSize"`
	InterviewLength     *int64 `json:"interviewLength"`
	IsArchived          *bool  `json:"isArchived"`
	Source              string `json:"source"`
}

func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	var req updateProjectRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	fields := map[string]any{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.StatusID != nil {
		fields["project_status_id"] = *req.StatusID
	}

	if req.Source == "iris" && h.irisProjectRepo != nil {
		if req.Description != "" {
			fields["description"] = req.Description
		}
		if req.SalesforceProjectID != "" {
			fields["salesforce_project_id"] = req.SalesforceProjectID
		}
		if req.IsArchived != nil {
			fields["is_archived"] = *req.IsArchived
		}
		if err := h.irisProjectRepo.Update(r.Context(), projectID, fields); err != nil {
			slog.Error("iris project update failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Updated: true, Source: "iris"})
		return
	}

	// Default to QS
	if h.qsProjectRepo != nil {
		if req.SampleSize != nil {
			fields["sample_size"] = *req.SampleSize
		}
		if req.InterviewLength != nil {
			fields["interview_length"] = *req.InterviewLength
		}
		if err := h.qsProjectRepo.Update(r.Context(), projectID, fields); err != nil {
			slog.Error("qs project update failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "update failed"})
			return
		}
		writeJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Updated: true, Source: "qs"})
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "no database available"})
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	// Soft-delete: archive instead of hard delete
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	source := r.URL.Query().Get("source")

	if source == "qs" && h.qsProjectRepo != nil {
		// QS: set project_status_id = 5 (Canceled)
		if err := h.qsProjectRepo.Update(r.Context(), projectID, map[string]any{"project_status_id": 5}); err != nil {
			slog.Error("qs project archive failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "archive failed"})
			return
		}
		writeJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Archived: true, Source: "qs"})
		return
	}

	if h.irisProjectRepo != nil {
		if err := h.irisProjectRepo.Update(r.Context(), projectID, map[string]any{"is_archived": true}); err != nil {
			slog.Error("iris project archive failed", "id", projectID, "error", err)
			writeJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "archive failed"})
			return
		}
	}
	writeJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Archived: true, Source: "iris"})
}
