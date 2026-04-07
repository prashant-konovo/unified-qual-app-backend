package dto

// ConferenceLoginRequest is the request body for conference login.
type ConferenceLoginRequest struct {
	Pin string `json:"pin"`
}

// RecordingUploadCallbackRequest is the request body for the recording upload callback.
type RecordingUploadCallbackRequest struct {
	RecordingURL string `json:"recordingUrl"`
	Bucket       string `json:"bucket"`
	Key          string `json:"key"`
	Duration     int    `json:"duration"`
	Size         int64  `json:"size"`
}

// CreateMeetingRequest is the request body for creating a conference meeting.
type CreateMeetingRequest struct {
	ProjectID      int64  `json:"projectId"`
	SubscriptionID int64  `json:"subscriptionId"`
	ModeratorID    int64  `json:"moderatorId"`
	TimeSlotID     int64  `json:"timeSlotId"`
	ExternalID     string `json:"externalMeetingId"`
}

// CreateTranscriptionOrderRequest is the request body for creating a transcription order.
type CreateTranscriptionOrderRequest struct {
	MeetingID string `json:"meetingId"`
	AudioURL  string `json:"audioUrl"`
}

// SendNotificationEmailRequest is the request body for sending a notification email.
type SendNotificationEmailRequest struct {
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
	Type       string   `json:"type"`
	ProjectID  int64    `json:"projectId"`
}

// SendSMSRequest is the request body for sending an SMS message.
type SendSMSRequest struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Message string `json:"message"`
}

// MraConferenceLinkRequest is the request body for adding or updating a conference link (MRA).
type MraConferenceLinkRequest struct {
	ConferenceLink     string  `json:"conferenceLink"`
	MeetingInformation [][]any `json:"meetingInformation"`
	UserID             any     `json:"userId"`
}

// LsAddConferenceLinkRequest is the request body for creating a conference link (LS brand).
type LsAddConferenceLinkRequest struct {
	TimeSlotID     int64  `json:"timeSlotId"     validate:"required,gt=0"`
	ConferenceHash string `json:"conferenceHash"`
}

// LsUpdateConferenceLinkRequest is the request body for updating a conference link (LS brand).
type LsUpdateConferenceLinkRequest struct {
	TimeSlotID     int64  `json:"timeSlotId"     validate:"required,gt=0"`
	ConferenceHash string `json:"conferenceHash"`
	Pin            string `json:"pin"`
}
