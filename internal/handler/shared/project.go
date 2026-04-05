package shared

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ProjectHandler handles project CRUD endpoints.
type ProjectHandler struct{ *service.Deps }

func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	pg := utilities.ParsePagination(r, 20, 100)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	source := service.ResolveSource(
		r.URL.Query().Get("source"),
		strings.ToUpper(r.URL.Query().Get("serviceCategory")),
	)

	var statusID *int
	if s := r.URL.Query().Get("statusId"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			statusID = &v
		}
	}

	result := h.ProjectService.ListProjects(r.Context(), pg.Page, pg.PageSize, source, search, statusID)
	utilities.WriteJSON(w, http.StatusOK, utilities.NewPaginated(result.Projects, pg.Page, pg.PageSize, result.Total))
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProjectRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	result, err := h.ProjectService.CreateProject(r.Context(), req)
	if err != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "failed to create project"})
		return
	}
	utilities.WriteJSON(w, http.StatusCreated, utilities.MutationResult{
		ID: result.ID, Source: result.Source, ServiceCategory: result.ServiceCategory,
	})
}

func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: err.Error()})
		return
	}

	result, err := h.ProjectService.GetProject(r.Context(), projectID, r.URL.Query().Get("source"))
	if err != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "database error"})
		return
	}
	if !result.Found {
		utilities.WriteJSON(w, http.StatusNotFound, utilities.ErrorBody{Error: "project not found"})
		return
	}
	utilities.WriteJSON(w, http.StatusOK, result.Project)
}

func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: err.Error()})
		return
	}

	var req dto.UpdateProjectRequest
	if errs := utilities.DecodeAndValidate(r, &req); errs != nil {
		utilities.WriteError(w, errs)
		return
	}

	result, err := h.ProjectService.UpdateProject(r.Context(), projectID, req)
	if err != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "update failed"})
		return
	}
	utilities.WriteJSON(w, http.StatusOK, result)
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "id")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, utilities.ErrorBody{Error: err.Error()})
		return
	}

	result, err := h.ProjectService.DeleteProject(r.Context(), projectID, r.URL.Query().Get("source"))
	if err != nil {
		utilities.WriteJSON(w, http.StatusInternalServerError, utilities.ErrorBody{Error: "archive failed"})
		return
	}
	utilities.WriteJSON(w, http.StatusOK, result)
}
