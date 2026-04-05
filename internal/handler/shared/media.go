package shared

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/utilities"
)

// ──────────────────────────────────────────────
// Media handlers (shared)
// ──────────────────────────────────────────────

// GetProjectMedia returns interview media for a project.
// GetProjectMedia returns paginated interview media for a project.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media
// Response: {"media": [...], "limit": N, "offset": N, "count": N}
func (h *Handler) GetProjectMedia(w http.ResponseWriter, r *http.Request) {
	projectID, err := utilities.ParseIDParam(r, "pid")
	if err != nil {
		utilities.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 25
	}

	if h.MediaService.Available() {
		media, err := h.MediaService.ListMediaForProject(r.Context(), projectID)
		if err != nil {
			slog.Error("list media failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		totalCount := len(media)

		// Apply pagination
		if offset > len(media) {
			offset = len(media)
		}
		end := offset + limit
		if end > len(media) {
			end = len(media)
		}
		paged := media[offset:end]

		result := make([]map[string]any, 0, len(paged))
		for _, m := range paged {
			entry := dto.MediaToJSON(m)
			// Add computed fields matching legacy response
			entry["basisPDF"] = fmt.Sprintf("/v1/project/%d/interview_media/%d/media.pdf", m.ProjectID, m.ID)
			pages := make([]map[string]any, 0, m.PageCount)
			for i := 0; i < m.PageCount; i++ {
				pages = append(pages, map[string]any{
					"page": fmt.Sprintf("/v1/project/%d/interview_media/%d/pages/%d/img.png", m.ProjectID, m.ID, i),
				})
			}
			entry["pages"] = pages
			result = append(result, entry)
		}
		utilities.WriteJSON(w, http.StatusOK, map[string]any{
			"media":  result,
			"limit":  limit,
			"offset": offset,
			"count":  totalCount,
		})
		return
	}
	utilities.WriteJSON(w, http.StatusOK, map[string]any{"media": []any{}, "limit": limit, "offset": offset, "count": 0})
}

// GetProjectMediaDetail returns a single media item with computed page URLs.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media/:mediaId
// Response: full InterviewMedia JSON with basisPDF and pages array
func (h *Handler) GetProjectMediaDetail(w http.ResponseWriter, r *http.Request) {
	projectID, _ := utilities.ParseIDParam(r, "pid")
	mediaID, _ := utilities.ParseIDParam(r, "mediaId")

	if h.MediaService.Available() {
		m, err := h.MediaService.GetMediaByID(r.Context(), projectID, mediaID)
		if err != nil {
			slog.Error("get media failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "database error"})
			return
		}
		if m == nil {
			utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
			return
		}
		entry := dto.MediaToJSON(*m)
		entry["basisPDF"] = fmt.Sprintf("/v1/project/%d/interview_media/%d/media.pdf", m.ProjectID, m.ID)
		pages := make([]map[string]any, 0, m.PageCount)
		for i := 0; i < m.PageCount; i++ {
			pages = append(pages, map[string]any{
				"page": fmt.Sprintf("/v1/project/%d/interview_media/%d/pages/%d/img.png", m.ProjectID, m.ID, i),
			})
		}
		entry["pages"] = pages
		utilities.WriteJSON(w, http.StatusOK, entry)
		return
	}
	utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
}

// DownloadMediaPDF streams a media PDF from S3.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media/:mediaId/media.pdf
func (h *Handler) DownloadMediaPDF(w http.ResponseWriter, r *http.Request) {
	projectID, _ := utilities.ParseIDParam(r, "pid")
	mediaID, _ := utilities.ParseIDParam(r, "mediaId")

	if !h.MediaService.Available() || !h.MediaService.S3Configured() {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	m, err := h.MediaService.GetMediaByID(r.Context(), projectID, mediaID)
	if err != nil || m == nil || !m.S3Key.Valid {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	key := m.S3Key.String + "/media.pdf"
	bucket := h.MediaService.RecordingBucket()
	body, contentLength, err := h.MediaService.GetS3Object(r.Context(), bucket, key)
	if err != nil {
		slog.Error("S3 get media PDF failed", "error", err, "key", key)
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// DownloadMediaPage streams a single page PDF from S3.
// Contract-identical with legacy InCrowdAPI: GET /v1/project/:projectId/interview_media/:mediaId/pages/:page/img.png
func (h *Handler) DownloadMediaPage(w http.ResponseWriter, r *http.Request) {
	projectID, _ := utilities.ParseIDParam(r, "pid")
	mediaID, _ := utilities.ParseIDParam(r, "mediaId")
	pageStr, _ := utilities.ParseStringParam(r, "page")
	page, _ := strconv.Atoi(pageStr)

	if !h.MediaService.Available() || !h.MediaService.S3Configured() {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	m, err := h.MediaService.GetMediaByID(r.Context(), projectID, mediaID)
	if err != nil || m == nil || !m.S3Key.Valid {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	key := fmt.Sprintf("%s/page/%d.pdf", m.S3Key.String, page)
	bucket := h.MediaService.RecordingBucket()
	body, contentLength, err := h.MediaService.GetS3Object(r.Context(), bucket, key)
	if err != nil {
		slog.Error("S3 get media page failed", "error", err, "key", key)
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// GetMediaPageForConference serves a media page PDF for conference participants.
// Contract-identical with legacy InCrowdAPI: GET /v1/interview_media/:conferenceHash/:mediaId/pages/:page/media.pdf
// GetMediaPageForConference serves a media page for a conference participant.
// Contract-identical with legacy InCrowdAPI: validates participant cookie
func (h *Handler) GetMediaPageForConference(w http.ResponseWriter, r *http.Request) {
	confHash, _ := utilities.ParseStringParam(r, "confHash")
	mediaID, _ := utilities.ParseIDParam(r, "mediaId")
	pageStr, _ := utilities.ParseStringParam(r, "page")
	page, _ := strconv.Atoi(pageStr)

	if !h.MediaService.Available() || !h.MediaService.S3Configured() {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	// Legacy validates participant via cookie: ic-participant → participantHash
	// Then checks ConferenceInvitation.readWhere(participantHash, timeSlotId)
	participantHash := ""
	if cookie, cErr := r.Cookie("ic-participant"); cErr == nil {
		participantHash = cookie.Value
	}

	// Look up timeslot by conference hash to verify access and get project ID
	projectID, err := h.MediaService.GetProjectIDByConferenceHash(r.Context(), confHash)
	if err != nil {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
		return
	}

	// Validate participant if cookie is present (legacy security check)
	if participantHash != "" && h.ConferenceService.Available() {
		ci, ciErr := h.ConferenceService.GetByHash(r.Context(), confHash)
		if ciErr != nil || ci == nil {
			utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "conference not found"})
			return
		}
		// Verify participant belongs to this conference timeslot
		participants, pErr := h.ConferenceService.GetParticipants(r.Context(), ci.TimeSlotID)
		if pErr != nil {
			utilities.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "access denied"})
			return
		}
		found := false
		for _, p := range participants {
			if ph, ok := p["participantHash"].(string); ok && ph == participantHash {
				found = true
				break
			}
		}
		if !found {
			utilities.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "access denied"})
			return
		}
	}

	m, err := h.MediaService.GetMediaByID(r.Context(), projectID, mediaID)
	if err != nil || m == nil || !m.S3Key.Valid {
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}

	key := fmt.Sprintf("%s/page/%d.pdf", m.S3Key.String, page)
	bucket := h.MediaService.RecordingBucket()
	body, contentLength, sErr := h.MediaService.GetS3Object(r.Context(), bucket, key)
	if sErr != nil {
		slog.Error("S3 get conference media page failed", "error", sErr, "key", key)
		utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// DeleteProjectMedia deletes interview media from S3 and database.
// Contract-identical with legacy InCrowdAPI: DELETE /v1/interview_media/:projectId/:mediaId
// Response: {} (empty JSON object)
func (h *Handler) DeleteProjectMedia(w http.ResponseWriter, r *http.Request) {
	projectID, _ := utilities.ParseIDParam(r, "pid")
	mediaID, _ := utilities.ParseIDParam(r, "mediaId")

	if h.MediaService.Available() {
		// Get media first for S3 cleanup
		m, _ := h.MediaService.GetMediaByID(r.Context(), projectID, mediaID)
		if m != nil && m.Shared {
			utilities.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "cannot delete shared media"})
			return
		}

		// Delete S3 objects if S3 is configured and media has an S3 key
		if m != nil && m.S3Key.Valid && h.MediaService.S3Configured() {
			bucket := h.MediaService.RecordingBucket()
			s3Root := m.S3Key.String
			// Delete main PDF
			_ = h.MediaService.DeleteS3Object(r.Context(), bucket, s3Root+"/media.pdf")
			// Delete page PDFs
			for i := 1; i <= m.PageCount; i++ {
				_ = h.MediaService.DeleteS3Object(r.Context(), bucket, fmt.Sprintf("%s/page/%d.pdf", s3Root, i))
			}
		}

		if err := h.MediaService.DeleteMedia(r.Context(), projectID, mediaID); err != nil {
			slog.Error("delete media failed", "error", err)
			utilities.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
			return
		}
		utilities.WriteJSON(w, http.StatusOK, map[string]any{})
		return
	}
	utilities.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "media not found"})
}

// ──────────────────────────────────────────────
// Subscription Domain
// ──────────────────────────────────────────────
