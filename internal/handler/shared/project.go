package shared

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/handler/httputil"
	"github.com/InCrowd/unified-qual-api/internal/service"
)

// ProjectHandler handles project CRUD endpoints.
type ProjectHandler struct{ *service.Deps }

func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	pg := httputil.ParsePagination(r, 20, 100)
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
	httputil.WriteJSON(w, http.StatusOK, dto.NewPaginated(result.Projects, pg.Page, pg.PageSize, result.Total))
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProjectRequest
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	result, err := h.ProjectService.CreateProject(r.Context(), req)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create project"})
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, dto.MutationResult{
		ID: result.ID, Source: result.Source, ServiceCategory: result.ServiceCategory,
	})
}

func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	result, err := h.ProjectService.GetProject(r.Context(), projectID, r.URL.Query().Get("source"))
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "database error"})
		return
	}
	if !result.Found {
		httputil.WriteJSON(w, http.StatusNotFound, dto.ErrorBody{Error: "project not found"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result.Project)
}

func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	var req dto.UpdateProjectRequest
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}

	result, err := h.ProjectService.UpdateProject(r.Context(), projectID, req)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "update failed"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}

	result, err := h.ProjectService.DeleteProject(r.Context(), projectID, r.URL.Query().Get("source"))
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "archive failed"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}
