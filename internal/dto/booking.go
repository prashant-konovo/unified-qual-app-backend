package dto

// UpdateBookingRequest is the request body for updating a booking's status.
type UpdateBookingRequest struct {
	StatusID *int `json:"statusId"`
}

// UpdateBookingRewardRequest is the request body for updating a booking's reward.
type UpdateBookingRewardRequest struct {
	RewardPoints int    `json:"rewardPoints"`
	RewardStatus string `json:"rewardStatus"`
}
