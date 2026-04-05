package shared

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/service"
	"github.com/InCrowd/unified-qual-api/internal/handler/httputil"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/go-chi/chi/v5"
)

// SurveyHandler handles survey CRUD endpoints.
type SurveyHandler struct{ *service.Deps }

// ──────────────────────────────────────────────
// Surveys
// ──────────────────────────────────────────────

func (h *SurveyHandler) ListSurveys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.SurveyService.QsAvailable() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	search := r.URL.Query().Get("search")
	rows, err := h.SurveyService.List(ctx, search)
	if err != nil {
		slog.Error("list surveys", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to list surveys"})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.SurveyFromRow(&row))
	}
	httputil.WriteJSON(w, http.StatusOK, out)
}


func (h *SurveyHandler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.SurveyService.QsAvailable() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	var req dto.CreateSurveyRequest
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
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
	newID, err := h.SurveyService.Create(ctx, req.ProjectID, req.Title, req.Status, req.Questions, req.Rules)
	if err != nil {
		slog.Error("create survey", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to create survey"})
		return
	}
	row, err := h.SurveyService.GetByID(ctx, newID)
	if err != nil || row == nil {
		httputil.WriteJSON(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("%d", newID)})
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, dto.SurveyFromRow(row))
}

func (h *SurveyHandler) UpdateSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.SurveyService.QsAvailable() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	surveyID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}
	var req dto.CreateSurveyRequest
	if errs := dto.DecodeAndValidate(r, &req); errs != nil {
		dto.WriteError(w, errs)
		return
	}
	if req.Questions == nil {
		req.Questions = json.RawMessage("[]")
	}
	if req.Rules == nil {
		req.Rules = json.RawMessage("[]")
	}
	if err := h.SurveyService.Update(ctx, surveyID, req.Title, req.Status, req.Questions, req.Rules); err != nil {
		slog.Error("update survey", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to update survey"})
		return
	}
	row, err := h.SurveyService.GetByID(ctx, surveyID)
	if err != nil || row == nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": fmt.Sprintf("%d", surveyID), "updatedAt": httputil.Now()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dto.SurveyFromRow(row))
}

func (h *SurveyHandler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.SurveyService.QsAvailable() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	surveyID, err := dto.ParseIDParam(r, "id")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}
	if err := h.SurveyService.Delete(ctx, surveyID); err != nil {
		slog.Error("delete survey", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to delete survey"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *SurveyHandler) GetPublicSurvey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.SurveyService.QsAvailable() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, dto.ErrorBody{Error: "QS database unavailable"})
		return
	}
	surveyID, err := dto.ParseIDParam(r, "surveyId")
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, dto.ErrorBody{Error: err.Error()})
		return
	}
	row, err := h.SurveyService.GetByID(ctx, surveyID)
	if err != nil {
		slog.Error("get public survey", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, dto.ErrorBody{Error: "failed to get survey"})
		return
	}
	if row == nil {
		httputil.WriteJSON(w, http.StatusNotFound, dto.ErrorBody{Error: "survey not found"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, dto.SurveyFromRow(row))
}

// ──────────────────────────────────────────────
// Survey Responses
// ──────────────────────────────────────────────

func (h *SurveyHandler) GetSurveyResponses(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, []map[string]any{
		{
			"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified",
			"submittedAt": "2026-03-15T10:00:00Z",
		},
	})
}

func (h *SurveyHandler) SubmitSurveyResponse(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": "resp-" + httputil.ID()[:8], "status": "qualified", "submittedAt": httputil.Now(),
	})
}

func (h *SurveyHandler) SubmitParticipantSurvey(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": "resp-" + httputil.ID()[:8], "status": "qualified", "submittedAt": httputil.Now(),
	})
}

func (h *SurveyHandler) GetParticipantSurveyResponse(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, []map[string]any{
		{"id": "resp-1", "userId": chi.URLParam(r, "userId"),
			"projectId": "proj-101", "status": "qualified"},
	})
}
