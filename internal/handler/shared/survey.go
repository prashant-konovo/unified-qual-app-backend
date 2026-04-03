package shared

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/handler/core"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// SurveyHandler handles survey CRUD endpoints.
type SurveyHandler struct{ *core.Deps }

// ──────────────────────────────────────────────
// Surveys
// ──────────────────────────────────────────────

func (h *SurveyHandler) ListSurveys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsSurveyRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	search := r.URL.Query().Get("search")
	rows, err := h.QsSurveyRepo.List(ctx, search)
	if err != nil {
		slog.Error("list surveys", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to list surveys"})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.SurveyFromRow(&row))
	}
	core.WriteJSON(w, http.StatusOK, out)
}

type createSurveyRequest struct {
	Title     string          `json:"title"`
	ProjectID *int64          `json:"projectId,omitempty"`
	Status    string          `json:"status"`
	Questions json.RawMessage `json:"questions"`
	Rules     json.RawMessage `json:"rules"`
}

func (h *SurveyHandler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsSurveyRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	var req createSurveyRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	if req.Title == "" {
		req.Title = "Untitled Survey"
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Questions == nil {
		req.Questions = json.RawMessage("[]")
	}
	if req.Rules == nil {
		req.Rules = json.RawMessage("[]")
	}
	newID, err := h.QsSurveyRepo.Create(ctx, req.ProjectID, req.Title, req.Status, req.Questions, req.Rules)
	if err != nil {
		slog.Error("create survey", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create survey"})
		return
	}
	row, err := h.QsSurveyRepo.GetByID(ctx, newID)
	if err != nil || row == nil {
		core.WriteJSON(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("%d", newID)})
		return
	}
	core.WriteJSON(w, http.StatusCreated, dto.SurveyFromRow(row))
}

func (h *SurveyHandler) UpdateSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsSurveyRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}
	var req createSurveyRequest
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}
	if req.Questions == nil {
		req.Questions = json.RawMessage("[]")
	}
	if req.Rules == nil {
		req.Rules = json.RawMessage("[]")
	}
	if err := h.QsSurveyRepo.Update(ctx, surveyID, req.Title, req.Status, req.Questions, req.Rules); err != nil {
		slog.Error("update survey", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to update survey"})
		return
	}
	row, err := h.QsSurveyRepo.GetByID(ctx, surveyID)
	if err != nil || row == nil {
		core.WriteJSON(w, http.StatusOK, map[string]any{"id": fmt.Sprintf("%d", surveyID), "updatedAt": core.Now()})
		return
	}
	core.WriteJSON(w, http.StatusOK, dto.SurveyFromRow(row))
}

func (h *SurveyHandler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsSurveyRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	surveyID, err := validate.ParseIDParam(r, "id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}
	if err := h.QsSurveyRepo.Delete(ctx, surveyID); err != nil {
		slog.Error("delete survey", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to delete survey"})
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *SurveyHandler) GetPublicSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.QsSurveyRepo == nil {
		core.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	surveyID, err := validate.ParseIDParam(r, "surveyId")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}
	row, err := h.QsSurveyRepo.GetByID(ctx, surveyID)
	if err != nil {
		slog.Error("get public survey", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to get survey"})
		return
	}
	if row == nil {
		core.WriteJSON(w, http.StatusNotFound, dto.ErrorBody{Error: "survey not found"})
		return
	}
	core.WriteJSON(w, http.StatusOK, dto.SurveyFromRow(row))
}

// ──────────────────────────────────────────────
// Survey Responses
// ──────────────────────────────────────────────

func (h *SurveyHandler) GetSurveyResponses(w http.ResponseWriter, r *http.Request) {
	core.WriteJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified",
			"submittedAt": "2026-03-15T10:00:00Z",
		},
	})
}

func (h *SurveyHandler) SubmitSurveyResponse(w http.ResponseWriter, r *http.Request) {
	core.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": "resp-" + core.ID()[:8], "status": "qualified", "submittedAt": core.Now(),
	})
}

func (h *SurveyHandler) SubmitParticipantSurvey(w http.ResponseWriter, r *http.Request) {
	core.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": "resp-" + core.ID()[:8], "status": "qualified", "submittedAt": core.Now(),
	})
}

func (h *SurveyHandler) GetParticipantSurveyResponse(w http.ResponseWriter, r *http.Request) {
	core.WriteJSON(w, http.StatusOK, []map[string]any{
		{"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified"},
	})
}
