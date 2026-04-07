package dto

// CreateParticipantRequest is the request body for creating a participant.
type CreateParticipantRequest struct {
	FirstName           string `json:"firstName"           validate:"required"`
	LastName            string `json:"lastName"            validate:"required"`
	Title               string `json:"title"`
	Email               string `json:"email"`
	Phone               string `json:"phone"`
	ExternalResponderID string `json:"externalResponderId"`
	TimeZone            string `json:"timeZone"`
}
