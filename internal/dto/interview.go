package dto

// CreateTimeslotRequest is the request body for creating a timeslot.
type CreateTimeslotRequest struct {
	ProjectID   int64  `json:"projectId"  validate:"required,gt=0"`
	StartTime   string `json:"startTime"  validate:"required"`
	EndTime     string `json:"endTime"    validate:"required"`
	Duration    int    `json:"duration"`
	ModeratorID int64  `json:"moderatorId"`
}

// UpdateTimeslotRequest is the request body for updating a timeslot.
type UpdateTimeslotRequest struct {
	StatusID  *int   `json:"statusId"`
	Confirmed *bool  `json:"confirmed"`
	IsInvalid *int   `json:"isInvalid"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Duration  *int   `json:"duration"`
}

// MraGetAllInterviewsRequest is the request body for fetching all interviews (MRA).
type MraGetAllInterviewsRequest struct {
	ExternalClientsIDs []string `json:"externalClientsIds"`
	ProjectsIDs        []string `json:"projectsIds"`
	ProjectAccountID   any      `json:"projectAccountId"`
	UserID             any      `json:"userId"`
	Offset             int      `json:"offset"`
	ActiveTab          string   `json:"activeTab"`
	HandleScroll       any      `json:"handleScroll"`
	PaymentStatusCode  string   `json:"paymentStatusCode"`
}

// MraScheduleInterviewSlot is the nested slot object within schedule/reschedule requests (MRA).
type MraScheduleInterviewSlot struct {
	StartTime               string `json:"startTime"`
	EndTime                 string `json:"endTime"`
	ModeratorAvailabilityID int64  `json:"moderatorAvailabilityId"`
	HasImportedOverlap      any    `json:"hasImportedOverlap"`
}

// MraScheduleInterviewRequest is the request body for scheduling an interview (MRA).
type MraScheduleInterviewRequest struct {
	SurveyID             int64                     `json:"surveyId"`
	ResponderLanguage    string                    `json:"responderLanguage"`
	IsReschedule         bool                      `json:"isReschedule"`
	RescheduleToken      string                    `json:"rescheduleToken"`
	UserTimeZone         string                    `json:"userTimeZone"`
	TimeZoneAbbr         string                    `json:"timeZoneAbbr"`
	QsPath               any                       `json:"qsPath"`
	IsUATTesting         bool                      `json:"isUATTesting"`
	ShgHash              string                    `json:"shgHash"`
	StartedAt            string                    `json:"startedAt"`
	FinishedAt           string                    `json:"finishedAt"`
	Comment              string                    `json:"comment"`
	InvalidateReschedule any                       `json:"invalidateReschedule"`
	Slot                 *MraScheduleInterviewSlot `json:"slot"`
}

// MraRespondentRescheduleRequest is the request body for respondent-initiated reschedule (MRA).
type MraRespondentRescheduleRequest struct {
	TimeSlotID           int64                     `json:"timeSlotId"`
	ResponderLanguage    string                    `json:"responderLanguage"`
	InvalidateReschedule any                       `json:"invalidateReschedule"`
	RespondentIdentifer  string                    `json:"respondentIdentifer"`
	RescheduleToken      string                    `json:"rescheduleToken"`
	SurveyID             int64                     `json:"surveyId"`
	IsReschedule         bool                      `json:"isReschedule"`
	UserTimeZone         string                    `json:"userTimeZone"`
	TimeZoneAbbr         string                    `json:"timeZoneAbbr"`
	QsPath               any                       `json:"qsPath"`
	ShgHash              string                    `json:"shgHash"`
	StartedAt            string                    `json:"startedAt"`
	FinishedAt           string                    `json:"finishedAt"`
	Comment              string                    `json:"comment"`
	Slot                 *MraScheduleInterviewSlot `json:"slot"`
}

// MraInvalidateInterviewRequest is the request body for invalidating an interview (MRA).
type MraInvalidateInterviewRequest struct {
	TimeSlotID             any    `json:"timeSlotId"`
	InvalidationReasonCode string `json:"invalidationReasonCode"`
	InvalidationReasonText string `json:"invalidationReasonText"`
	InvalidatedByUserID    any    `json:"invalidatedByUserId"`
	IsInvalidateEmailSent  *bool  `json:"isInvalidateEmailSent"`
}

// MraSendInvalidateRescheduleMailRequest is the request body for sending
// invalidate/reschedule mail (MRA).
type MraSendInvalidateRescheduleMailRequest struct {
	TimeSlotID       any   `json:"timeSlotId"`
	ParticipantID    any   `json:"participantId"`
	IsIneligibleMail *bool `json:"isIneligibleMail"`
}

// MraCancelRescheduleActionRequest is the request body for the cancel/reschedule action (MRA).
type MraCancelRescheduleActionRequest struct {
	ResponderLanguage string `json:"responderLanguage"`
}
