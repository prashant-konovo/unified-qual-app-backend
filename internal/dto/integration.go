package dto

// LsCheckQualEligibilityRequest is the request body for checking participant eligibility (LS brand).
type LsCheckQualEligibilityRequest struct {
	ResponderID int64 `json:"responderId" validate:"required,gt=0"`
	ProjectID   int64 `json:"projectId"   validate:"required,gt=0"`
}

// MraLogFrontEndEventRequest is the request body for logging a front-end event (MRA).
type MraLogFrontEndEventRequest struct {
	EventName           string `json:"eventName"`
	Error               any    `json:"error"`
	StatusCode          any    `json:"statusCode"`
	SurveyID            any    `json:"surveyId"`
	DecipherSurveyID    any    `json:"decipherSurveyId"`
	ProjectID           any    `json:"projectId"`
	ShgHash             any    `json:"shgHash"`
	QsPath              any    `json:"qsPath"`
	RespondentIdentifer any    `json:"respondentIdentifer"`
}

// MraQualEligibilityRequest is the request body for bulk eligibility status updates (MRA).
type MraQualEligibilityRequest struct {
	ParticipantIDs []any  `json:"participant_ids"`
	UpdatedBy      string `json:"updated_by"`
	Reason         string `json:"reason"`
	Status         string `json:"status"`
}
