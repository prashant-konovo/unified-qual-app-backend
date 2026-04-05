package dto

// ScheduleInterviewRequest is the request body for scheduling an interview.
type ScheduleInterviewRequest struct {
	TimeSlotID  int64 `json:"timeSlotId"  validate:"required,gt=0"`
	ModeratorID int64 `json:"moderatorId"`
	ResponderID int64 `json:"responderId"`
}

// CancelInterviewRequest is the request body for cancelling an interview.
type CancelInterviewRequest struct {
	Reason string `json:"reason"`
}

// RescheduleInterviewRequest is the request body for rescheduling an interview.
type RescheduleInterviewRequest struct {
	NewStartTime string `json:"newStartTime"`
	NewEndTime   string `json:"newEndTime"`
	Reason       string `json:"reason"`
}
