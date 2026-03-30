package qs

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ConferenceInvitation maps to the QS conference_invitation table.
type ConferenceInvitation struct {
	ID             int64          `json:"id"`
	ConferenceHash string         `json:"conferenceHash"`
	TimeSlotID     int64          `json:"timeSlotId"`
	Pin            sql.NullString `json:"pin"`
	ModifiedOn     time.Time      `json:"modifiedOn"`
}

// ConferenceResponderTimeSlot maps to conference_invitation_responder_time_slot.
type ConferenceResponderTimeSlot struct {
	ID              int64     `json:"id"`
	TimeSlotID      int64     `json:"timeSlotId"`
	ResponderID     int64     `json:"responderId"`
	ParticipantHash string    `json:"participantHash"`
	ModifiedOn      time.Time `json:"modifiedOn"`
}

// ConferenceRepo handles QS conference/meeting queries.
type ConferenceRepo struct {
	db *sql.DB
}

// NewConferenceRepo creates a new QS conference repository.
func NewConferenceRepo(db *sql.DB) *ConferenceRepo {
	return &ConferenceRepo{db: db}
}

// GetByHash returns a conference invitation by its hash.
func (r *ConferenceRepo) GetByHash(ctx context.Context, hash string) (*ConferenceInvitation, error) {
	q := `SELECT id, conference_hash, time_slot_id, pin, modified_on
	      FROM conference_invitation WHERE conference_hash = ?`
	var ci ConferenceInvitation
	err := r.db.QueryRowContext(ctx, q, hash).Scan(&ci.ID, &ci.ConferenceHash, &ci.TimeSlotID, &ci.Pin, &ci.ModifiedOn)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conference by hash: %w", err)
	}
	return &ci, nil
}

// GetParticipants returns responders linked to a timeslot through conference invitation.
func (r *ConferenceRepo) GetParticipants(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	q := `SELECT cirts.id, cirts.time_slot_id, cirts.responder_id, cirts.participant_hash,
	       resp.first_name, resp.last_name
	      FROM conference_invitation_responder_time_slot cirts
	      JOIN responder resp ON resp.id = cirts.responder_id
	      WHERE cirts.time_slot_id = ?`
	rows, err := r.db.QueryContext(ctx, q, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("get conference participants: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, tsID, respID int64
		var pHash, fn, ln string
		if err := rows.Scan(&id, &tsID, &respID, &pHash, &fn, &ln); err != nil {
			return nil, fmt.Errorf("scan conference participant: %w", err)
		}
		result = append(result, map[string]any{
			"id": id, "timeSlotId": tsID, "responderId": respID,
			"participantHash": pHash, "firstName": fn, "lastName": ln,
		})
	}
	return result, rows.Err()
}

// GetAttendeesByMeetingID returns attendees info for a Chime meeting.
func (r *ConferenceRepo) GetAttendeesByMeetingID(ctx context.Context, meetingID string) ([]map[string]any, error) {
	q := `SELECT ts.id AS time_slot_id, ts.project_id,
	       mts.moderator_id, CONCAT(mu.first_name, ' ', mu.last_name) AS moderator_name,
	       cirts.responder_id, CONCAT(resp.first_name, ' ', resp.last_name) AS responder_name
	      FROM conference_invitation ci
	      JOIN time_slot ts ON ts.id = ci.time_slot_id
	      LEFT JOIN moderator_time_slot mts ON mts.time_slot_id = ts.id
	      LEFT JOIN user mu ON mu.id = mts.moderator_id
	      LEFT JOIN conference_invitation_responder_time_slot cirts ON cirts.time_slot_id = ts.id
	      LEFT JOIN responder resp ON resp.id = cirts.responder_id
	      WHERE ci.conference_hash = ?`
	rows, err := r.db.QueryContext(ctx, q, meetingID)
	if err != nil {
		return nil, fmt.Errorf("get attendees: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var tsID, projectID int64
		var modID sql.NullInt64
		var modName sql.NullString
		var respID sql.NullInt64
		var respName sql.NullString
		if err := rows.Scan(&tsID, &projectID, &modID, &modName, &respID, &respName); err != nil {
			return nil, fmt.Errorf("scan attendee: %w", err)
		}
		m := map[string]any{"timeSlotId": tsID, "projectId": projectID}
		if modID.Valid {
			m["moderatorId"] = modID.Int64
			m["moderatorName"] = modName.String
		}
		if respID.Valid {
			m["responderId"] = respID.Int64
			m["responderName"] = respName.String
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetMeetingMetadata returns the conference hash + project + timeslot info.
func (r *ConferenceRepo) GetMeetingMetadata(ctx context.Context, conferenceHash string) (map[string]any, error) {
	q := `SELECT ci.id, ci.conference_hash, ci.time_slot_id, ci.pin,
	       ts.project_id, ts.start_time, ts.end_time, ts.duration, ts.status_id, p.name AS project_name
	      FROM conference_invitation ci
	      JOIN time_slot ts ON ts.id = ci.time_slot_id
	      JOIN project p ON p.id = ts.project_id
	      WHERE ci.conference_hash = ?`
	var ciID, tsID, projectID int64
	var hash string
	var pin sql.NullString
	var startTime, endTime time.Time
	var duration, statusID int
	var projectName string
	err := r.db.QueryRowContext(ctx, q, conferenceHash).Scan(
		&ciID, &hash, &tsID, &pin,
		&projectID, &startTime, &endTime, &duration, &statusID, &projectName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting metadata: %w", err)
	}
	m := map[string]any{
		"conferenceId":   ciID,
		"conferenceHash": hash,
		"timeSlotId":     tsID,
		"projectId":      projectID,
		"projectName":    projectName,
		"startTime":      startTime.Format(time.RFC3339),
		"endTime":        endTime.Format(time.RFC3339),
		"duration":       duration,
		"statusId":       statusID,
	}
	if pin.Valid {
		m["pin"] = pin.String
	}
	return m, nil
}

// Login validates a conference hash and optional pin, returns conference data.
func (r *ConferenceRepo) Login(ctx context.Context, conferenceHash, pin string) (map[string]any, error) {
	ci, err := r.GetByHash(ctx, conferenceHash)
	if err != nil {
		return nil, err
	}
	if ci == nil {
		return nil, fmt.Errorf("conference not found")
	}
	if ci.Pin.Valid && ci.Pin.String != "" && ci.Pin.String != pin {
		return nil, fmt.Errorf("invalid pin")
	}
	meta, err := r.GetMeetingMetadata(ctx, conferenceHash)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

// CreateConferenceLink creates a conference invitation link for a timeslot.
func (r *ConferenceRepo) CreateConferenceLink(ctx context.Context, timeSlotID, projectID int64, conferenceHash string) (int64, error) {
	q := `INSERT INTO conference_invitation (time_slot_id, conference_hash, created_on) VALUES (?, ?, NOW())`
	res, err := r.db.ExecContext(ctx, q, timeSlotID, conferenceHash)
	if err != nil {
		return 0, fmt.Errorf("create conference link: %w", err)
	}
	return res.LastInsertId()
}

// UpdateConferenceLink updates a conference invitation's hash/pin.
func (r *ConferenceRepo) UpdateConferenceLink(ctx context.Context, timeSlotID int64, conferenceHash, pin string) error {
	q := `UPDATE conference_invitation SET conference_hash = ?, pin = ? WHERE time_slot_id = ?`
	_, err := r.db.ExecContext(ctx, q, conferenceHash, pin, timeSlotID)
	if err != nil {
		return fmt.Errorf("update conference link: %w", err)
	}
	return nil
}

// GetConferenceLinkByTimeSlotID returns conference data by timeslot ID.
func (r *ConferenceRepo) GetConferenceLinkByTimeSlotID(ctx context.Context, timeSlotID int64) (map[string]any, error) {
	q := `SELECT id, time_slot_id, conference_hash, COALESCE(pin, '') as pin FROM conference_invitation WHERE time_slot_id = ?`
	var id, tsID int64
	var hash, pinVal string
	err := r.db.QueryRowContext(ctx, q, timeSlotID).Scan(&id, &tsID, &hash, &pinVal)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conf link by ts: %w", err)
	}
	return map[string]any{"id": id, "timeSlotId": tsID, "conferenceHash": hash, "pin": pinVal}, nil
}

// UpdateRecordingStatus updates recording metadata for a conference by meeting/hash ID.
func (r *ConferenceRepo) UpdateRecordingStatus(ctx context.Context, meetingID, status, bucket, key string) error {
	q := `UPDATE conference_invitation SET recording_status = ?, recording_bucket = ?, recording_key = ?, modified_on = NOW()
	      WHERE conference_hash = ?`
	_, err := r.db.ExecContext(ctx, q, status, bucket, key, meetingID)
	if err != nil {
		return fmt.Errorf("update recording status: %w", err)
	}
	return nil
}
