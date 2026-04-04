package dto

// LoginRequest is the request body for email/password authentication.
type LoginRequest struct {
	Email         string `json:"email"    validate:"required,email"`
	Password      string `json:"password" validate:"required,min=1"`
	TermsAccepted *bool  `json:"termsAccepted"`
}

// RefreshRequest is the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
	ICUserID     int64  `json:"icUserId"`
	ICAuthToken  string `json:"icAuthToken"`
}
