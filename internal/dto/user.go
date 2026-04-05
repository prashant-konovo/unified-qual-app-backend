package dto

import (
"fmt"
"strconv"
"strings"
)

// QsRoleName maps QS role_id to a human-readable name.
func QsRoleName(id int) string {
switch id {
case 1:
return "moderator"
case 2:
return "manager"
case 3:
return "admin"
default:
return fmt.Sprintf("role_%d", id)
}
}

// UpdateUserRequest is the request body for updating a user.
type UpdateUserRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	TimeZone  string `json:"timeZone"`
	Source    string `json:"source"`
}

// CreateEventLogRequest is the request body for creating an event log.
type CreateEventLogRequest struct {
	EventType   string `json:"eventType"`
	Description string `json:"description"`
	UserID      int64  `json:"userId"`
	ProjectID   int64  `json:"projectId"`
	TimeSlotID  int64  `json:"timeSlotId"`
	MetaData    string `json:"metaData"`
}

// UpsertUserTimeZoneRequest is the request body for upserting a user's time zone.
type UpsertUserTimeZoneRequest struct {
	UserID               int64  `json:"userId"`
	UserSelectedTimeZone string `json:"userSelectedTimeZone"`
}

// MraPatchUserRequest is the request body for admin password reset (MRA).
type MraPatchUserRequest struct {
	Password string `json:"password"`
	Token    string `json:"token"`
}

// MraPasswordRequest is the request body for password-only operations (MRA).
type MraPasswordRequest struct {
	Password string `json:"password"`
}

// LsSendPasswordResetRequest is the request body for sending a password reset email (LS brand).
type LsSendPasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// LsCheckUserQsToolI2Request is the request body for checking user QS Tool and I2 status (LS brand).
type LsCheckUserQsToolI2Request struct {
	Email string `json:"email" validate:"required,email"`
}

// LsUnsubscribeUserRequest is the request body for unsubscribing a user (LS brand).
type LsUnsubscribeUserRequest struct {
	PmUserID            string `json:"pmUserId"`
	AllowContactByEmail int    `json:"allowContactByEmail"`
}

// ParseRoleCSV splits a comma-separated role-id string from GROUP_CONCAT.
func ParseRoleCSV(csv string) []int {
if csv == "" {
return nil
}
parts := strings.Split(csv, ",")
ids := make([]int, 0, len(parts))
for _, p := range parts {
v, err := strconv.Atoi(strings.TrimSpace(p))
if err == nil {
ids = append(ids, v)
}
}
return ids
}
