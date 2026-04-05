package shared

import (
	"log/slog"
	"net/http"
	"strconv"

	qualapi "github.com/InCrowd/unified-qual-api"

	"github.com/InCrowd/unified-qual-api/internal/validate"
	"github.com/go-chi/chi/v5"
)

// ──────────────────────────────────────────────
// Translation handlers (shared)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Translations (LLD endpoints)
// ──────────────────────────────────────────────

func (h *Handler) GetLocales(w http.ResponseWriter, r *http.Request) {
	qualapi.WriteJSON(w, http.StatusOK, map[string]any{
		"locales": []map[string]string{
			{"code": "en_us", "name": "English (US)"},
			{"code": "es_es", "name": "Spanish"},
			{"code": "fr_fr", "name": "French"},
			{"code": "fr_ca", "name": "French (Canada)"},
			{"code": "de_de", "name": "German"},
			{"code": "it_it", "name": "Italian"},
			{"code": "pt_pt", "name": "Portuguese"},
		},
	})
}

func (h *Handler) UpdateTopicTranslations(w http.ResponseWriter, r *http.Request) {
	pidStr := chi.URLParam(r, "projectId")
	projectID, _ := strconv.ParseInt(pidStr, 10, 64)

	var req struct {
		Translations []struct {
			TopicID        int64  `json:"topicId"`
			LanguageCode   string `json:"languageCode"`
			TranslatedName string `json:"translatedName"`
		} `json:"translations"`
	}
	if errs := validate.DecodeAndValidate(r, &req); errs != nil {
		validate.WriteError(w, errs)
		return
	}

	if h.QsAnswerRepo != nil {
		for _, t := range req.Translations {
			if err := h.QsAnswerRepo.UpdateTopicTranslation(r.Context(), projectID, t.TopicID, t.LanguageCode, t.TranslatedName); err != nil {
				slog.Error("update translation failed", "topicId", t.TopicID, "error", err)
			}
		}
		qualapi.WriteJSON(w, http.StatusOK, map[string]any{"projectId": projectID, "updatedCount": len(req.Translations)})
		return
	}
	qualapi.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no database available"})
}
