package ls

import (
	"github.com/InCrowd/unified-qual-api/internal/dto"
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
func (h *Handler) GetLocalesReal(w http.ResponseWriter, r *http.Request) {
	if h.TranslationService.AnswerRepoAvailable() {
		locales, err := h.TranslationService.ListLocales(r.Context())
		if err != nil {
			slog.Error("list locales failed", "error", err)
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		result := make([]map[string]any, 0, len(locales))
		for _, l := range locales {
			result = append(result, map[string]any{
				"id": l.ID, "languageCode": l.LanguageCode, "languageName": l.LanguageName,
			})
		}
		dto.WriteJSON(w, http.StatusOK, result)
		return
	}
	dto.WriteJSON(w, http.StatusOK, []any{})
}

// ──────────────────────────────────────────────
// Topic Translation Delete (MRA #72, #74)
// ──────────────────────────────────────────────

func (h *Handler) DeleteTopicTranslation(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	translationKey := chi.URLParam(r, "translationKey")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	parts := strings.SplitN(translationKey, "_", 2)
	if len(parts) < 2 {
		dto.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid translation key format: topicId_langCode"})
		return
	}
	topicID, _ := strconv.ParseInt(parts[0], 10, 64)
	langCode := parts[1]

	if h.TranslationService.AnswerRepoAvailable() {
		if err := h.TranslationService.DeleteTopicTranslation(r.Context(), topicID, langCode); err != nil {
			slog.Error("delete topic translation failed", "error", err)
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		dto.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true, "projectId": projectID, "topicId": topicID, "languageCode": langCode})
		return
	}
	dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}

func (h *Handler) DeleteTranslation(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	langCode := chi.URLParam(r, "langCode")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	if h.TranslationService.AnswerRepoAvailable() {
		if err := h.TranslationService.DeleteTranslation(r.Context(), projectID, langCode); err != nil {
			slog.Error("delete translation failed", "error", err)
			dto.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		dto.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true, "projectId": projectID, "languageCode": langCode})
		return
	}
	dto.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}
