package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	qs "github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// JSON / HTTP helpers
// ──────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Brand", "unified")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func id() string { return uuid.New().String() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func parsePagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// extractBearerToken extracts the JWT bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	// Also check IC-Auth and CognitoToken headers (legacy patterns)
	if t := r.Header.Get("IC-Auth"); t != "" {
		return t
	}
	if t := r.Header.Get("CognitoToken"); t != "" {
		return t
	}
	return ""
}

// ──────────────────────────────────────────────
// Null-safe helpers for JSON serialization
// ──────────────────────────────────────────────

func nullStr(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

func nullInt64(ni sql.NullInt64) any {
	if ni.Valid {
		return ni.Int64
	}
	return nil
}

func nullTime(nt sql.NullTime) any {
	if nt.Valid {
		return nt.Time.Format(time.RFC3339)
	}
	return nil
}

func toNullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func toNullInt64(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}

// ──────────────────────────────────────────────
// Role / CSV helpers
// ──────────────────────────────────────────────

// qsRoleName maps QS role_id to a human-readable name.
func qsRoleName(id int) string {
	switch id {
	case 1:
		return "moderator"
	case 2:
		return "manager"
	case 3:
		return "admin"
	default:
		return fmt.Sprintf("role_%d", id)
	}
}

// parseRoleCSV splits a comma-separated role-id string from GROUP_CONCAT.
func parseRoleCSV(csv string) []int {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			ids = append(ids, v)
		}
	}
	return ids
}

// mediaToJSON converts ICInterviewMedia to the legacy JSON response shape.
func mediaToJSON(m iris.ICInterviewMedia) map[string]any {
	result := map[string]any{
		"id": m.ID, "name": m.Name, "description": m.Description,
		"projectId": m.ProjectID, "s3Key": nil, "hash": nil,
		"status": m.Status, "createdOn": m.CreatedOn.Format(time.RFC3339),
		"createdBy": m.CreatedBy, "pageCount": m.PageCount,
		"pagesProcessed": m.PagesProcessed, "shared": m.Shared,
	}
	if m.S3Key.Valid {
		result["s3Key"] = m.S3Key.String
	}
	if m.Hash.Valid {
		result["hash"] = m.Hash.String
	}
	return result
}

// buildTimeSlotResponse returns the timeslot fields matching legacy getTimeSlotByIdForCancelReschedule response.
func buildTimeSlotResponse(ts *qs.TimeSlot) map[string]any {
	return map[string]any{
		"id":                     ts.ID,
		"projectId":              ts.ProjectID,
		"isInvalidatedInterview": ts.IsInvalidatedInterview,
		"isInvalidateEmailSent":  ts.IsInvalidateEmailSent,
		"invalidationReasonCode": nullStr(ts.InvalidationReasonCode),
		"startTime":              ts.StartTime,
		"endTime":                ts.EndTime,
		"statusId":               ts.StatusID,
		"duration":               ts.Duration,
		"isInvalid":              ts.IsInvalid,
	}
}
