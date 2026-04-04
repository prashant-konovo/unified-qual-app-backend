package shared

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/handler/support"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/validate"
)

// ProjectHandler handles project CRUD endpoints.
type ProjectHandler struct{ *support.Deps }

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
		if h.IrisProjectRepo != nil {
			irisProjects, irisTotal, err := h.IrisProjectRepo.List(r.Context(), pg.Page, pg.PageSize, statusID, search)
			if err != nil {
				log.Error("iris project list failed", "error", err)
			} else {
				for _, p := range irisProjects {
					result = append(result, dto.ProjectFromIRISList(p))
				}
				_ = irisTotal
			}
		}
	}

	// QS projects
	if source == "" || source == "qs" {
		if h.QsProjectRepo != nil {
			qsProjects, qsTotal, err := h.QsProjectRepo.List(r.Context(), pg.Page, pg.PageSize, statusID, search)
			if err != nil {
				log.Error("qs project list failed", "error", err)
			} else {
				for _, p := range qsProjects {
					result = append(result, dto.ProjectFromQSList(p))
				}
				_ = qsTotal
			}
		}
	}

	if result == nil {
		result = []map[string]any{}
	}
	support.WriteJSON(w, http.StatusOK, dto.NewPaginated(result, pg.Page, pg.PageSize, len(result)))
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProjectRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	if req.Source == "" {
		req.Source = "qs" // default to QS
	}

	user := middleware.GetUser(r)

	if req.Source == "iris" && h.IrisProjectRepo != nil {
		p := &iris.Project{
			Name:                req.Name,
			Description:         dto.ToNullStr(req.Description),
			SubscriptionID:      req.SubscriptionID,
			SalesforceProjectID: dto.ToNullStr(req.SalesforceProjectID),
			ProjectStatusID:     2, // Defining
		}
		id, err := h.IrisProjectRepo.Create(r.Context(), p)
		if err != nil {
			slog.Error("iris project create failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create project"})
			return
		}
		support.WriteJSON(w, http.StatusCreated, dto.MutationResult{ID: id, Source: "iris", ServiceCategory: "LS"})
		return
	}

	if h.QsProjectRepo != nil {
		p := &qs.Project{
			Name:                req.Name,
			SalesforceJobNumber: dto.ToNullStr(req.SalesforceJobNumber),
			SampleSize:          dto.ToNullInt64(req.SampleSize),
			InterviewLength:     dto.ToNullInt64(req.InterviewLength),
			ClientID:            dto.ToNullInt64(req.ClientID),
			PostScreeninBuffer:  dto.ToNullStr(fmt.Sprintf("%.2f", req.PostScreeninBuffer)),
			ModeratorBuffer:     dto.ToNullStr(fmt.Sprintf("%.2f", req.ModeratorBuffer)),
		}
		_ = user // TODO: set CreatedBy from user lookup
		id, err := h.QsProjectRepo.Create(r.Context(), p)
		if err != nil {
			slog.Error("qs project create failed", "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create project"})
			return
		}
		support.WriteJSON(w, http.StatusCreated, dto.MutationResult{ID: id, Source: "qs", ServiceCategory: "MRA"})
		return
	}

	support.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "no database available"})
}

// GetProject returns a project by ID.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:id
// Response: flat project adminJson object
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	source := r.URL.Query().Get("source")

	// Try QS first (or if source=qs)
	if (source == "" || source == "qs") && h.QsProjectRepo != nil {
		p, err := h.QsProjectRepo.GetByID(r.Context(), projectID)
		if err != nil {
			slog.Error("qs project get failed", "id", projectID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "database error"})
			return
		}
		if p != nil {
			scheduled, completed, _ := h.QsProjectRepo.TimeSlotCounts(r.Context(), projectID)
			topics, _ := h.QsProjectRepo.GetTopics(r.Context(), projectID)
			topicNames := make([]string, 0, len(topics))
			for _, t := range topics {
				topicNames = append(topicNames, t.TopicName)
			}
			support.WriteJSON(w, http.StatusOK, dto.ProjectDetailFromQS(p, scheduled, completed, topicNames))
			return
		}
	}

	// Try IRIS (or if source=iris)
	if (source == "" || source == "iris") && h.IrisProjectRepo != nil {
		p, err := h.IrisProjectRepo.GetByID(r.Context(), projectID)
		if err != nil {
			slog.Error("iris project get failed", "id", projectID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "database error"})
			return
		}
		if p != nil {
			support.WriteJSON(w, http.StatusOK, dto.ProjectDetailFromIRIS(p, irisStatusName[p.ProjectStatusID]))
			return
		}
	}

	support.WriteJSON(w, http.StatusNotFound, dto.ErrorBody{Error: "project not found"})
}

func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	var req dto.UpdateProjectRequest
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

	if req.Source == "iris" && h.IrisProjectRepo != nil {
		if req.Description != "" {
			fields["description"] = req.Description
		}
		if req.SalesforceProjectID != "" {
			fields["salesforce_project_id"] = req.SalesforceProjectID
		}
		if req.IsArchived != nil {
			fields["is_archived"] = *req.IsArchived
		}
		if err := h.IrisProjectRepo.Update(r.Context(), projectID, fields); err != nil {
			slog.Error("iris project update failed", "id", projectID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "update failed"})
			return
		}
		support.WriteJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Updated: true, Source: "iris"})
		return
	}

	// Default to QS
	if h.QsProjectRepo != nil {
		if req.SampleSize != nil {
			fields["sample_size"] = *req.SampleSize
		}
		if req.InterviewLength != nil {
			fields["interview_length"] = *req.InterviewLength
		}
		if err := h.QsProjectRepo.Update(r.Context(), projectID, fields); err != nil {
			slog.Error("qs project update failed", "id", projectID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "update failed"})
			return
		}
		support.WriteJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Updated: true, Source: "qs"})
		return
	}

	support.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "no database available"})
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	// Soft-delete: archive instead of hard delete
	projectID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		support.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	source := r.URL.Query().Get("source")

	if source == "qs" && h.QsProjectRepo != nil {
		// QS: set project_status_id = 5 (Canceled)
		if err := h.QsProjectRepo.Update(r.Context(), projectID, map[string]any{"project_status_id": 5}); err != nil {
			slog.Error("qs project archive failed", "id", projectID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "archive failed"})
			return
		}
		support.WriteJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Archived: true, Source: "qs"})
		return
	}

	if h.IrisProjectRepo != nil {
		if err := h.IrisProjectRepo.Update(r.Context(), projectID, map[string]any{"is_archived": true}); err != nil {
			slog.Error("iris project archive failed", "id", projectID, "error", err)
			support.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "archive failed"})
			return
		}
	}
	support.WriteJSON(w, http.StatusOK, dto.MutationResult{ID: projectID, Archived: true, Source: "iris"})
}
