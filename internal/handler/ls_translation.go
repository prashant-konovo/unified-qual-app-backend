package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// LS Translation handlers
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Translations extended
// ──────────────────────────────────────────────

// GetLocalesReal returns locales from the QS DB.
func (h *LSHandler) GetLocalesReal(w http.ResponseWriter, r *http.Request) {
	if h.qsAnswerRepo != nil {
		locales, err := h.qsAnswerRepo.ListLocales(r.Context())
		if err != nil {
			slog.Error("list locales failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(locales))
		for _, l := range locales {
			result = append(result, map[string]any{
				"id": l.ID, "languageCode": l.LanguageCode, "languageName": l.LanguageName,
			})
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}

// ──────────────────────────────────────────────
// Topic Translation Delete (MRA #72, #74)
// ──────────────────────────────────────────────

func (h *LSHandler) DeleteTopicTranslation(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	translationKey := chi.URLParam(r, "translationKey")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	parts := strings.SplitN(translationKey, "_", 2)
	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid translation key format: topicId_langCode"})
		return
	}
	topicID, _ := strconv.ParseInt(parts[0], 10, 64)
	langCode := parts[1]

	if h.qsAnswerRepo != nil {
		if err := h.qsAnswerRepo.DeleteTopicTranslation(r.Context(), topicID, langCode); err != nil {
			slog.Error("delete topic translation failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "projectId": projectID, "topicId": topicID, "languageCode": langCode})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *LSHandler) DeleteTranslation(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	langCode := chi.URLParam(r, "langCode")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	if h.qsAnswerRepo != nil {
		if err := h.qsAnswerRepo.DeleteTranslation(r.Context(), projectID, langCode); err != nil {
			slog.Error("delete translation failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "projectId": projectID, "languageCode": langCode})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}
