package model

import "time"

// Project represents a project entity used across both IRIS and QS databases.
type Project struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Brand           string     `json:"brand"`
	Status          string     `json:"status"`
	InterviewLength int        `json:"interviewLength"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	CompletedCount  int        `json:"completedCount"`
	Moderators      []Moderator `json:"moderators,omitempty"`
}

// Moderator represents a moderator assigned to a project or available for scheduling.
type Moderator struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Role      string `json:"role,omitempty"`
	Status    string `json:"status,omitempty"`
}

// Survey represents a survey linked to a project.
type Survey struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Questions   []any     `json:"questions,omitempty"`
	Rules       []any     `json:"rules,omitempty"`
}

// Timeslot represents an interview time slot.
type Timeslot struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"projectId"`
	ModeratorID   string    `json:"moderatorId"`
	ModeratorName string    `json:"moderatorName,omitempty"`
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	Timezone      string    `json:"timezone"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Participant represents a participant/respondent.
type Participant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Booking represents a booked interview slot.
type Booking struct {
	ID            string    `json:"id"`
	TimeslotID    string    `json:"timeslotId"`
	ParticipantID string    `json:"participantId"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
}
