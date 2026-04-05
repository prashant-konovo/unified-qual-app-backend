package dto

// SendReminderRequest is the request body for sending a reminder notification.
type SendReminderRequest struct {
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
	Type       string   `json:"type"`
	ProjectID  int64    `json:"projectId"`
}
