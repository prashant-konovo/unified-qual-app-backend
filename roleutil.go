package qualapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/dto"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// QsRoleName maps QS role_id to a human-readable name.
func QsRoleName(id int) string {
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

// ParseRoleCSV splits a comma-separated role-id string from GROUP_CONCAT.
func ParseRoleCSV(csv string) []int {
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

// MediaToJSON converts ICInterviewMedia to the legacy JSON response shape.
func MediaToJSON(m iris.ICInterviewMedia) map[string]any {
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

// BuildTimeSlotResponse returns the timeslot fields matching legacy getTimeSlotByIdForCancelReschedule response.
func BuildTimeSlotResponse(ts *qs.TimeSlot) map[string]any {
	return map[string]any{
		"id":                     ts.ID,
		"projectId":              ts.ProjectID,
		"isInvalidatedInterview": ts.IsInvalidatedInterview,
		"isInvalidateEmailSent":  ts.IsInvalidateEmailSent,
		"invalidationReasonCode": dto.NullStr(ts.InvalidationReasonCode),
		"startTime":              ts.StartTime,
		"endTime":                ts.EndTime,
		"statusId":               ts.StatusID,
		"duration":               ts.Duration,
		"isInvalid":              ts.IsInvalid,
	}
}
