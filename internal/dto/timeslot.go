package dto

import (
	"github.com/InCrowd/unified-qual-api/internal/utilities"
	"time"

	qs "github.com/InCrowd/unified-qual-api/internal/repository/qs"
)

// ──────────────────────────────────────────────
// Timeslot response mappers
// ──────────────────────────────────────────────

// TimeslotFromListRow converts a QS TimeSlotListRow to a response map.
func TimeslotFromListRow(s qs.TimeSlotListRow) map[string]any {
	item := map[string]any{
		"id":                     s.ID,
		"projectId":              s.ProjectID,
		"projectName":            s.ProjectName,
		"startTime":              s.StartTime.Format(time.RFC3339),
		"endTime":                s.EndTime.Format(time.RFC3339),
		"duration":               s.Duration,
		"statusId":               s.StatusID,
		"status":                 s.StatusName,
		"confirmed":              s.Confirmed,
		"isInvalid":              s.IsInvalid,
		"isInvalidatedInterview": s.IsInvalidatedInterview,
		"source":                 "qs",
		"serviceCategory":        "MRA",
		"modifiedOn":             s.ModifiedOn.Format(time.RFC3339),
	}
	if s.ModeratorID.Valid {
		item["moderatorId"] = s.ModeratorID.Int64
		item["moderatorName"] = s.ModeratorName.String
		item["isHost"] = s.IsHost.Valid && s.IsHost.Bool
	}
	if s.ResponderID.Valid {
		item["responderId"] = s.ResponderID.Int64
		item["responderName"] = s.ResponderName.String
	}
	if s.ConferenceHash.Valid {
		item["conferenceHash"] = s.ConferenceHash.String
	}
	if s.InvalidationReasonCode.Valid {
		item["invalidationReasonCode"] = s.InvalidationReasonCode.String
	}
	return item
}

// TimeslotFromDetail converts a QS TimeSlot (detail view) to a response map.
func TimeslotFromDetail(ts *qs.TimeSlot) map[string]any {
	result := map[string]any{
		"id":                     ts.ID,
		"projectId":              ts.ProjectID,
		"startTime":              ts.StartTime.Format(time.RFC3339),
		"endTime":                ts.EndTime.Format(time.RFC3339),
		"confirmed":              ts.Confirmed,
		"statusId":               ts.StatusID,
		"duration":               ts.Duration,
		"isInvalid":              ts.IsInvalid,
		"isInvalidatedInterview": ts.IsInvalidatedInterview,
		"isPreviousNoShow":       ts.IsPreviousNoShow,
		"source":                 "qs",
		"serviceCategory":        "MRA",
		"modifiedOn":             ts.ModifiedOn.Format(time.RFC3339),
	}
	if ts.ConferenceHash.Valid {
		result["conferenceHash"] = ts.ConferenceHash.String
	}
	if ts.ParticipantHash.Valid {
		result["participantHash"] = ts.ParticipantHash.String
	}
	if ts.InvalidationReasonCode.Valid {
		result["invalidationReasonCode"] = ts.InvalidationReasonCode.String
	}
	if ts.InvalidationReasonText.Valid {
		result["invalidationReasonText"] = ts.InvalidationReasonText.String
	}
	return result
}

// TimeslotModerator converts a moderator_time_slot row to a response map.
func TimeslotModerator(m qs.ModeratorTimeSlot) map[string]any {
	return map[string]any{
		"moderatorId": m.ModeratorID,
		"isHost":      m.IsHost,
	}
}

// TimeslotRespondent builds a respondent sub-object for timeslot detail.
func TimeslotRespondent(r *qs.Respondent) map[string]any {
	return map[string]any{
		"id":        r.ID,
		"firstName": r.FirstName,
		"lastName":  r.LastName,
		"timeZone":  r.TimeZone.String,
	}
}

// SlotFromListRowSimple builds a minimal slot response for available slots listing.
func SlotFromListRowSimple(s qs.TimeSlotListRow) map[string]any {
	item := map[string]any{
		"id":        s.ID,
		"projectId": s.ProjectID,
		"startTime": s.StartTime.Format(time.RFC3339),
		"endTime":   s.EndTime.Format(time.RFC3339),
		"duration":  s.Duration,
	}
	if s.ModeratorID.Valid {
		item["moderatorId"] = s.ModeratorID.Int64
		item["moderatorName"] = s.ModeratorName.String
	}
	return item
}

// BuildTimeSlotResponse returns the timeslot fields matching legacy getTimeSlotByIdForCancelReschedule response.
func BuildTimeSlotResponse(ts *qs.TimeSlot) map[string]any {
return map[string]any{
"id":                     ts.ID,
"projectId":              ts.ProjectID,
"isInvalidatedInterview": ts.IsInvalidatedInterview,
"isInvalidateEmailSent":  ts.IsInvalidateEmailSent,
"invalidationReasonCode": utilities.NullStr(ts.InvalidationReasonCode),
"startTime":              ts.StartTime,
"endTime":                ts.EndTime,
"statusId":               ts.StatusID,
"duration":               ts.Duration,
"isInvalid":              ts.IsInvalid,
}
}

// LsAssignTimeslotModeratorRequest is the request body for assigning a moderator to a timeslot (LS brand).
type LsAssignTimeslotModeratorRequest struct {
	ModeratorID int64 `json:"moderatorId" validate:"required,gt=0"`
	IsHost      bool  `json:"isHost"`
}

// LsUpdateTimeslotObserversRequest is the request body for adding/removing timeslot observers (LS brand).
type LsUpdateTimeslotObserversRequest struct {
	ProjectID int64    `json:"projectId" validate:"required,gt=0"`
	ToAdd     []string `json:"toAdd"`
	ToDelete  []string `json:"toDelete"`
}
