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

// ChangePasswordRequest is the request body for changing a password.
type ChangePasswordRequest struct {
	OldPassword string `json:"currentPassword"`
	NewPassword string `json:"newPassword"`
	Password    string `json:"password"`
	Token       string `json:"token"`
	UserID      int64  `json:"userId"`
}

// LogoutRequest is the request body for logging out.
type LogoutRequest struct {
	ICUserID    int64  `json:"icUserId"`
	ICAuthToken string `json:"icAuthToken"`
}

// AcceptTermsRequest is the request body for accepting terms.
type AcceptTermsRequest struct {
	UserID int64 `json:"userId" validate:"required,gt=0"`
}

// SSOCallbackRequest is the request body for the SSO callback.
type SSOCallbackRequest struct {
	Code        string `json:"code"        validate:"required"`
	RedirectURI string `json:"redirectUri" validate:"required"`
}
