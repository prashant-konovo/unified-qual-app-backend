package mra

import (
	"log/slog"
	"net/http"

	"github.com/InCrowd/unified-qual-api/internal/handler/core"

	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// MRA Translation handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// MRA #70 — GetTopicsByProjectMRA
// GET /v1/mra/project/get_topics_by_project/{project_id}
// Legacy: get_topics_by_project.js
// ──────────────────────────────────────────────

func (h *Handler) GetTopicsByProjectMRA(w http.ResponseWriter, r *http.Request) {
	if h.QsProjectRepo == nil {
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	projectID, err := validate.ParseIDParam(r, "project_id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	ctx := r.Context()

	topics, err := h.QsProjectRepo.GetTopicsByProjectMRA(ctx, projectID)
	if err != nil {
		slog.Error("get topics by project failed", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	// Get project status once — legacy checks per-topic but status is the same
	projectStatusID, err := h.QsProjectRepo.GetProjectStatusByIdMRA(ctx, projectID)
	if err != nil {
		slog.Error("get project status failed", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err})
		return
	}

	// Get scheduled languages once (only if project status == 2)
	var scheduledLanguages []string
	if projectStatusID == 2 {
		scheduledLanguages, err = h.QsProjectRepo.GetRespondersLanguagesByProjectIdMRA(ctx, projectID)
		if err != nil {
			slog.Error("get responders languages failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err})
			return
		}
	}
	scheduledSet := make(map[string]bool, len(scheduledLanguages))
	for _, lang := range scheduledLanguages {
		scheduledSet[lang] = true
	}

	topicsInformation := make([]any, 0, len(topics))
	canNotBeEdited := make([]any, 0, len(topics))

	for _, t := range topics {
		langCode, _ := t["langCode_countryCode"].(string)
		topicName, _ := t["topic_name"].(string)
		topicsInformation = append(topicsInformation, []any{langCode, topicName})
		canNotBeEdited = append(canNotBeEdited, []any{langCode, scheduledSet[langCode]})
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"topicsInformation": topicsInformation,
		"canNotBeEdited":    canNotBeEdited,
	})
}

// ──────────────────────────────────────────────
// MRA #71 — UpdateTopicTranslationMRA
// POST /v1/mra/update-topic-translation/{project_id}
// Legacy: update-topic-transaltion.js
// ──────────────────────────────────────────────

func (h *Handler) UpdateTopicTranslationMRA(w http.ResponseWriter, r *http.Request) {
	if h.QsProjectRepo == nil {
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	projectID, err := validate.ParseIDParam(r, "project_id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	var req struct {
		TopicInfo [][]any `json:"topicInfo"`
		UserID    any     `json:"userId"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	ctx := r.Context()

	for _, info := range req.TopicInfo {
		if len(info) < 2 {
			continue
		}
		languageCode, _ := info[0].(string)
		topicName, _ := info[1].(string)

		languageID, ok := langCodeToID[languageCode]
		if !ok {
			continue
		}

		exists, err := h.QsProjectRepo.GetTopicsByProjectIdAndLanguageIdMRA(ctx, projectID, languageID)
		if err != nil {
			slog.Error("check topic exists failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": err.Error(),
			})
			return
		}

		if exists {
			err = h.QsProjectRepo.UpdateTopicByProjectAndLanguageMRA(ctx, topicName, languageID, projectID, req.UserID)
		} else {
			err = h.QsProjectRepo.AddTopicByProjectAndLanguageMRA(ctx, topicName, languageID, projectID, req.UserID)
		}
		if err != nil {
			slog.Error("upsert topic translation failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": err.Error(),
			})
			return
		}
	}

	core.WriteJSON(w, http.StatusOK, "Done")
}

// ──────────────────────────────────────────────
// MRA #72 — DeleteTopicTranslationMRA
// DELETE /v1/mra/translations/delete-topic-translation/{project_id}/{translation_to_delete}
// Legacy: delete-topic-transaltion.js
// ──────────────────────────────────────────────

func (h *Handler) DeleteTopicTranslationMRA(w http.ResponseWriter, r *http.Request) {
	if h.QsProjectRepo == nil {
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "repository not available"})
		return
	}

	projectID, err := validate.ParseIDParam(r, "project_id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	translationToDelete := chi.URLParam(r, "transaltion_to_delete")

	ctx := r.Context()

	// Check if scheduled interviews exist for this language
	projectStatusID, err := h.QsProjectRepo.GetProjectStatusByIdMRA(ctx, projectID)
	if err != nil {
		slog.Error("get project status failed", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	if projectStatusID == 2 {
		scheduledLanguages, err := h.QsProjectRepo.GetRespondersLanguagesByProjectIdMRA(ctx, projectID)
		if err != nil {
			slog.Error("get responders languages failed", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error":        err.Error(),
				"errorMessage": err.Error(),
			})
			return
		}
		for _, lang := range scheduledLanguages {
			if lang == translationToDelete {
				core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
					"errorMessage": "Scheduled Interviews exists with the language trying to be deleted",
				})
				return
			}
		}
	}

	languageID, ok := langCodeToID[translationToDelete]
	if !ok {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown language code"})
		return
	}

	if err := h.QsProjectRepo.DeleteTopicByProjectAndLanguageMRA(ctx, projectID, languageID); err != nil {
		slog.Error("delete topic translation failed", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"errorMessage": err.Error(),
		})
		return
	}

	// Legacy returns Data API transaction commit result
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"transactionStatus": "Transaction Committed",
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #73: GET /translations/get-all-localisations
// Legacy: getAllLanguageLocalisationsFactory — returns 3 arrays in one object
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) GetAllLocalisationsMRA(w http.ResponseWriter, r *http.Request) {
	allLangs, err := h.QsProjectRepo.GetAllLanguageLocalisationsMRA(r.Context())
	if err != nil {
		slog.Error("get all localisations mra: all", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	langData, err := h.QsProjectRepo.GetDataFromLanguageLocalisationsMRA(r.Context())
	if err != nil {
		slog.Error("get all localisations mra: data", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	countriesData, err := h.QsProjectRepo.GetCountriesWithLocalisationsMRA(r.Context())
	if err != nil {
		slog.Error("get all localisations mra: countries", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	core.WriteJSON(w, http.StatusOK, map[string]any{
		"getAllLanguageLocalisations":     allLangs,
		"getDataFromLanguageLocalisation": langData,
		"getCountriesWithLocalisations":   countriesData,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// MRA #74: DELETE /translations/delete-translation/{project_id}/{transaltion_to_delete}
// Legacy: deleteTranslationFactory — checks scheduled interviews, deletes project_meeting_translation
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) DeleteTranslationMRA(w http.ResponseWriter, r *http.Request) {
	projectID, err := validate.ParseIDParam(r, "project_id")
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	translationToDelete := chi.URLParam(r, "transaltion_to_delete")

	// Check for scheduled interviews
	projectStatusID, err := h.QsProjectRepo.GetProjectStatusByIdMRA(r.Context(), projectID)
	if err != nil {
		slog.Error("delete translation mra: get project status", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	if projectStatusID == 2 {
		respondersLanguages, err := h.QsProjectRepo.GetRespondersLanguagesByProjectIdMRA(r.Context(), projectID)
		if err != nil {
			slog.Error("delete translation mra: get responders languages", "error", err)
			core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
			return
		}
		for _, lang := range respondersLanguages {
			if lang == translationToDelete {
				core.WriteJSON(w, http.StatusInternalServerError, map[string]any{
					"errorMessage": "Scheduled Interviews exists with the language trying to be deleted",
				})
				return
			}
		}
	}

	languageID := langCodeToID[translationToDelete]
	if languageID == 0 {
		core.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid language code"})
		return
	}

	result, err := h.QsProjectRepo.DeleteMeetingInformationTranslationMRA(r.Context(), projectID, languageID)
	if err != nil {
		slog.Error("delete translation mra", "error", err)
		core.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "errorMessage": err.Error()})
		return
	}

	core.WriteJSON(w, http.StatusOK, result)
}
