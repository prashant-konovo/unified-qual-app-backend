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

// AddConferenceLinkMRA inserts meeting information per language, inserts conference link,
// and updates project modified_on — matching legacy addConferenceLinkService exactly.
func (r *ConferenceRepo) AddConferenceLinkMRA(ctx context.Context, projectID int64, participantGroupID int64, userID int64, conferenceLink string, meetingInformation [][]any) (map[string]any, error) {
	// Language code → language_id mapping (matches legacy switch)
	langMap := map[string]int{
		"en_us": 1, "fr_fr": 2, "fr_ca": 3,
		"de_de": 4, "es_es": 5, "it_it": 6, "pt_br": 7,
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lastResult map[string]any
	for _, entry := range meetingInformation {
		if len(entry) < 2 {
			continue
		}
		langCode, _ := entry[0].(string)
		meetingInfo, _ := entry[1].(string)
		langID, ok := langMap[langCode]
		if !ok {
			continue
		}

		// INSERT INTO project_meeting_translation
		res, err := tx.ExecContext(ctx,
			`INSERT INTO project_meeting_translation(project_id, language_id, meeting_information, created_by) VALUES (?, ?, ?, ?)`,
			projectID, langID, meetingInfo, userID,
		)
		if err != nil {
			return nil, fmt.Errorf("insert meeting translation: %w", err)
		}

		// UPDATE project modified_on (legacy does this in each transaction iteration)
		_, err = tx.ExecContext(ctx,
			`UPDATE project SET modified_on = NOW() WHERE id = ?`, projectID,
		)
		if err != nil {
			return nil, fmt.Errorf("update project modified_on: %w", err)
		}

		insertID, _ := res.LastInsertId()
		lastResult = map[string]any{
			"numberOfRecordsUpdated": 1,
			"insertId":              insertID,
		}
	}

	// INSERT INTO conference_invitation (legacy columns)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO conference_invitation(conference_link, participant_group_id, user_id) VALUES (?, ?, ?)`,
		conferenceLink, participantGroupID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert conference invitation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return lastResult, nil
}

// GetExistingMeetingLanguagesMRA returns language codes that already have meeting info for a project.
func (r *ConferenceRepo) GetExistingMeetingLanguagesMRA(ctx context.Context, projectID int64) (map[string]bool, error) {
	q := `SELECT l.langCode_countryCode
		FROM project_meeting_translation pmt
		JOIN language_localisation l ON l.id = pmt.language_id
		WHERE pmt.project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get existing meeting languages: %w", err)
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan language code: %w", err)
		}
		result[code] = true
	}
	return result, rows.Err()
}

// UpdateConferenceLinkMRA updates conference_invitation + project_meeting_translation
// matching legacy updateConferenceLinkService exactly.
// For each meetingInformation entry:
//   - If language exists: UPDATE conference_invitation, UPDATE project.modified_on, UPDATE project_meeting_translation
//   - If new language: INSERT project_meeting_translation, UPDATE project.modified_on
func (r *ConferenceRepo) UpdateConferenceLinkMRA(ctx context.Context, projectID, participantGroupID int64, userID int64, conferenceLink string, meetingInformation [][]any, existingLangs map[string]bool) error {
	langMap := map[string]int{
		"en_us": 1, "fr_fr": 2, "fr_ca": 3,
		"de_de": 4, "es_es": 5, "it_it": 6, "pt_br": 7,
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, entry := range meetingInformation {
		if len(entry) < 2 {
			continue
		}
		langCode, _ := entry[0].(string)
		meetingInfo, _ := entry[1].(string)
		langID, ok := langMap[langCode]
		if !ok {
			continue
		}

		if existingLangs[langCode] {
			// Legacy updateConferenceLink transaction:
			// 1. UPDATE conference_invitation SET conference_link WHERE participant_group_id
			_, err = tx.ExecContext(ctx,
				`UPDATE conference_invitation SET conference_link = ? WHERE participant_group_id = ?`,
				conferenceLink, participantGroupID,
			)
			if err != nil {
				return fmt.Errorf("update conference_invitation: %w", err)
			}

			// 2. UPDATE project.modified_on
			_, err = tx.ExecContext(ctx,
				`UPDATE project SET modified_on = NOW() WHERE id = ?`, projectID,
			)
			if err != nil {
				return fmt.Errorf("update project modified_on: %w", err)
			}

			// 3. UPDATE project_meeting_translation
			_, err = tx.ExecContext(ctx,
				`UPDATE project_meeting_translation SET meeting_information = ? WHERE project_id = ? AND language_id = ?`,
				meetingInfo, projectID, langID,
			)
			if err != nil {
				return fmt.Errorf("update meeting translation: %w", err)
			}
		} else {
			// Legacy addMeetingInfo transaction:
			// 1. INSERT project_meeting_translation
			_, err = tx.ExecContext(ctx,
				`INSERT INTO project_meeting_translation(project_id, language_id, meeting_information, created_by) VALUES (?, ?, ?, ?)`,
				projectID, langID, meetingInfo, userID,
			)
			if err != nil {
				return fmt.Errorf("insert meeting translation: %w", err)
			}

			// 2. UPDATE project.modified_on
			_, err = tx.ExecContext(ctx,
				`UPDATE project SET modified_on = NOW() WHERE id = ?`, projectID,
			)
			if err != nil {
				return fmt.Errorf("update project modified_on: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// GetPendingTimeSlotsCountMRA returns count of pending (status_id=2) non-invalidated timeslots for a project.
func (r *ConferenceRepo) GetPendingTimeSlotsCountMRA(ctx context.Context, projectID int64) (int64, error) {
	q := `SELECT count(*) FROM time_slot WHERE project_id = ? AND status_id = 2 AND is_invalidated_interview = 0`
	var count int64
	if err := r.db.QueryRowContext(ctx, q, projectID).Scan(&count); err != nil {
		return 0, fmt.Errorf("get pending timeslots count: %w", err)
	}
	return count, nil
}

// GetConferenceLinkByProjectMRA returns conference link + multi-language meeting information
// matching legacy getConferenceLink SQL exactly.
// NOTE: Legacy passes participant_group_id but actually uses it as project_id in the WHERE clause.
func (r *ConferenceRepo) GetConferenceLinkByProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT
		c.id AS conferenceId,
		c.conference_link,
		pmt.language_id,
		pmt.meeting_information,
		pmt.project_id,
		l.id AS lang_id,
		l.langCode_countryCode,
		l.name AS lang_name
	FROM conference_invitation c
	JOIN participant_group pg ON c.participant_group_id = pg.id
	JOIN survey s ON s.id = pg.survey_id
	JOIN project p ON p.id = s.project_id
	JOIN project_meeting_translation pmt ON pmt.project_id = p.id
	JOIN conference_invitation ci ON ci.participant_group_id = pg.id
	JOIN language_localisation l ON l.id = pmt.language_id
	WHERE p.id = ?`

	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get conference link by project: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var confID, langID, projectIDVal int64
		var confLink, meetingInfo, langCode, langName string
		if err := rows.Scan(&confID, &confLink, &langID, &meetingInfo, &projectIDVal, &langID, &langCode, &langName); err != nil {
			return nil, fmt.Errorf("scan conference link row: %w", err)
		}
		records = append(records, map[string]any{
			"conferenceId":          confID,
			"conference_link":       confLink,
			"language_id":           langID,
			"meeting_information":   meetingInfo,
			"project_id":            projectIDVal,
			"langCode_countryCode":  langCode,
			"name":                  langName,
		})
	}
	return records, rows.Err()
}
