package qs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// TimeSlot maps to the QS `time_slot` table (25 columns verified).
type TimeSlot struct {
	ID                       int64          `json:"id"`
	ProjectID                int64          `json:"projectId"`
	StartTime                time.Time      `json:"startTime"`
	EndTime                  time.Time      `json:"endTime"`
	Confirmed                bool           `json:"confirmed"`
	ConferenceHash           sql.NullString `json:"conferenceHash"`
	ParticipantHash          sql.NullString `json:"participantHash"`
	StatusID                 int            `json:"statusId"`
	StatusModifiedBy         sql.NullInt64  `json:"statusModifiedBy"`
	Duration                 int            `json:"duration"`
	IsInvalid                int            `json:"isInvalid"`
	ModifiedOn               time.Time      `json:"modifiedOn"`
	RespondentModifiedBy     sql.NullInt64  `json:"respondentModifiedBy"`
	QsPath                   sql.NullInt64  `json:"qsPath"`
	ReminderSent30           sql.NullBool   `json:"reminderSent30"`
	ReminderSent24           sql.NullBool   `json:"reminderSent24"`
	UpdatedOn                sql.NullTime   `json:"updatedOn"`
	HasImportedOverlap       sql.NullBool   `json:"hasImportedOverlap"`
	IsInvalidatedInterview   bool           `json:"isInvalidatedInterview"`
	IsInvalidateEmailSent    bool           `json:"isInvalidateEmailSent"`
	InvalidationReasonCode   sql.NullString `json:"invalidationReasonCode"`
	InvalidationReasonText   sql.NullString `json:"invalidationReasonText"`
	InvalidatedByUserID      sql.NullInt64  `json:"invalidatedByUserId"`
	IsPreviousNoShow         bool           `json:"isPreviousNoShow"`
	IsIneligibleMailSent     bool           `json:"isIneligibleMailSent"`
}

// TimeSlotListRow is a flattened row for list queries with JOINs.
type TimeSlotListRow struct {
	ID                     int64          `json:"id"`
	ProjectID              int64          `json:"projectId"`
	ProjectName            string         `json:"projectName"`
	StartTime              time.Time      `json:"startTime"`
	EndTime                time.Time      `json:"endTime"`
	Duration               int            `json:"duration"`
	StatusID               int            `json:"statusId"`
	StatusName             string         `json:"statusName"`
	Confirmed              bool           `json:"confirmed"`
	IsInvalid              int            `json:"isInvalid"`
	ModeratorID            sql.NullInt64  `json:"moderatorId"`
	ModeratorName          sql.NullString `json:"moderatorName"`
	IsHost                 sql.NullBool   `json:"isHost"`
	ResponderID            sql.NullInt64  `json:"responderId"`
	ResponderName          sql.NullString `json:"responderName"`
	ConferenceHash         sql.NullString `json:"conferenceHash"`
	IsInvalidatedInterview bool           `json:"isInvalidatedInterview"`
	InvalidationReasonCode sql.NullString `json:"invalidationReasonCode"`
	ModifiedOn             time.Time      `json:"modifiedOn"`
}

// TimeSlotStatus maps to the QS `time_slot_status` table.
type TimeSlotStatus struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

// ModeratorTimeSlot maps to the QS `moderator_time_slot` junction table.
type ModeratorTimeSlot struct {
	ID           int64     `json:"id"`
	ModeratorID  int64     `json:"moderatorId"`
	TimeSlotID   int64     `json:"timeSlotId"`
	IsHost       bool      `json:"isHost"`
	ModifiedOn   time.Time `json:"modifiedOn"`
}

// TimeSlotRepo handles QS time_slot CRUD.
type TimeSlotRepo struct {
	db *sql.DB
}

// NewTimeSlotRepo creates a new QS timeslot repository.
func NewTimeSlotRepo(db *sql.DB) *TimeSlotRepo {
	return &TimeSlotRepo{db: db}
}

const timeSlotListQuery = `
SELECT
	ts.id, ts.project_id, p.name AS project_name,
	ts.start_time, ts.end_time, ts.duration,
	ts.status_id, tss.description AS status_name,
	ts.confirmed, ts.is_invalid,
	mts.moderator_id, CONCAT(u.first_name, ' ', u.last_name) AS moderator_name, mts.is_host,
	cirts.responder_id, CONCAT(r.first_name, ' ', r.last_name) AS responder_name,
	ts.conference_hash,
	ts.is_invalidated_interview, ts.invalidation_reason_code,
	ts.modified_on
FROM time_slot ts
JOIN project p ON p.id = ts.project_id
JOIN time_slot_status tss ON tss.id = ts.status_id
LEFT JOIN moderator_time_slot mts ON mts.time_slot_id = ts.id
LEFT JOIN user u ON u.id = mts.moderator_id
LEFT JOIN conference_invitation_responder_time_slot cirts ON cirts.time_slot_id = ts.id
LEFT JOIN responder r ON r.id = cirts.responder_id
`

// List returns timeslots with optional filters.
func (repo *TimeSlotRepo) List(ctx context.Context, page, pageSize int, projectID *int64, statusID *int, moderatorID *int64, from *time.Time, to *time.Time) ([]TimeSlotListRow, int, error) {
	var conditions []string
	var args []any

	if projectID != nil {
		conditions = append(conditions, "ts.project_id = ?")
		args = append(args, *projectID)
	}
	if statusID != nil {
		conditions = append(conditions, "ts.status_id = ?")
		args = append(args, *statusID)
	}
	if moderatorID != nil {
		conditions = append(conditions, "mts.moderator_id = ?")
		args = append(args, *moderatorID)
	}
	if from != nil {
		conditions = append(conditions, "ts.start_time >= ?")
		args = append(args, *from)
	}
	if to != nil {
		conditions = append(conditions, "ts.start_time <= ?")
		args = append(args, *to)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQ := "SELECT COUNT(DISTINCT ts.id) FROM time_slot ts LEFT JOIN moderator_time_slot mts ON mts.time_slot_id = ts.id " + where
	var total int
	if err := repo.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count timeslots: %w", err)
	}

	query := timeSlotListQuery + where + " ORDER BY ts.start_time DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list timeslots: %w", err)
	}
	defer rows.Close()

	var result []TimeSlotListRow
	for rows.Next() {
		var r TimeSlotListRow
		if err := rows.Scan(
			&r.ID, &r.ProjectID, &r.ProjectName,
			&r.StartTime, &r.EndTime, &r.Duration,
			&r.StatusID, &r.StatusName,
			&r.Confirmed, &r.IsInvalid,
			&r.ModeratorID, &r.ModeratorName, &r.IsHost,
			&r.ResponderID, &r.ResponderName,
			&r.ConferenceHash,
			&r.IsInvalidatedInterview, &r.InvalidationReasonCode,
			&r.ModifiedOn,
		); err != nil {
			return nil, 0, fmt.Errorf("scan timeslot row: %w", err)
		}
		result = append(result, r)
	}
	return result, total, rows.Err()
}

// GetByID returns a single timeslot with full details.
func (repo *TimeSlotRepo) GetByID(ctx context.Context, id int64) (*TimeSlot, error) {
	const q = `SELECT
		id, project_id, start_time, end_time, confirmed,
		conference_hash, participant_hash, status_id, status_modified_by,
		duration, is_invalid, modified_on, respondent_modified_by, qsPath,
		reminder_sent_30, reminder_sent_24, updated_on, has_imported_overlap,
		is_invalidated_interview, is_invalidate_email_sent,
		invalidation_reason_code, invalidation_reason_text, invalidated_by_user_id,
		is_previous_no_show, is_ineligible_mail_sent
	FROM time_slot WHERE id = ?`

	var ts TimeSlot
	err := repo.db.QueryRowContext(ctx, q, id).Scan(
		&ts.ID, &ts.ProjectID, &ts.StartTime, &ts.EndTime, &ts.Confirmed,
		&ts.ConferenceHash, &ts.ParticipantHash, &ts.StatusID, &ts.StatusModifiedBy,
		&ts.Duration, &ts.IsInvalid, &ts.ModifiedOn, &ts.RespondentModifiedBy, &ts.QsPath,
		&ts.ReminderSent30, &ts.ReminderSent24, &ts.UpdatedOn, &ts.HasImportedOverlap,
		&ts.IsInvalidatedInterview, &ts.IsInvalidateEmailSent,
		&ts.InvalidationReasonCode, &ts.InvalidationReasonText, &ts.InvalidatedByUserID,
		&ts.IsPreviousNoShow, &ts.IsIneligibleMailSent,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get timeslot %d: %w", id, err)
	}
	return &ts, nil
}

// GetModerators returns moderators assigned to a timeslot.
func (repo *TimeSlotRepo) GetModerators(ctx context.Context, timeSlotID int64) ([]ModeratorTimeSlot, error) {
	const q = `SELECT id, moderator_id, time_slot_id, is_host, modified_on
		FROM moderator_time_slot WHERE time_slot_id = ?`
	rows, err := repo.db.QueryContext(ctx, q, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("get timeslot moderators: %w", err)
	}
	defer rows.Close()

	var result []ModeratorTimeSlot
	for rows.Next() {
		var m ModeratorTimeSlot
		if err := rows.Scan(&m.ID, &m.ModeratorID, &m.TimeSlotID, &m.IsHost, &m.ModifiedOn); err != nil {
			return nil, fmt.Errorf("scan moderator_time_slot: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// GetRespondent returns the respondent linked to a timeslot via CIRTS junction.
func (repo *TimeSlotRepo) GetRespondent(ctx context.Context, timeSlotID int64) (*Respondent, error) {
	const q = `SELECT r.id, r.first_name, r.last_name, r.title, r.external_responder_id,
		r.sess_key, r.time_zone, r.time_zone_abbr, r.language_country, r.modified_on
		FROM responder r
		JOIN conference_invitation_responder_time_slot cirts ON cirts.responder_id = r.id
		WHERE cirts.time_slot_id = ?
		LIMIT 1`

	var resp Respondent
	err := repo.db.QueryRowContext(ctx, q, timeSlotID).Scan(
		&resp.ID, &resp.FirstName, &resp.LastName, &resp.Title,
		&resp.ExternalResponderID, &resp.SessKey,
		&resp.TimeZone, &resp.TimeZoneAbbr, &resp.LanguageCountry, &resp.ModifiedOn,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get respondent for timeslot %d: %w", timeSlotID, err)
	}
	return &resp, nil
}

// Create inserts a new timeslot.
func (repo *TimeSlotRepo) Create(ctx context.Context, ts *TimeSlot) (int64, error) {
	const q = `INSERT INTO time_slot
		(project_id, start_time, end_time, confirmed, status_id, duration)
		VALUES (?, ?, ?, ?, ?, ?)`
	res, err := repo.db.ExecContext(ctx, q,
		ts.ProjectID, ts.StartTime, ts.EndTime, ts.Confirmed, ts.StatusID, ts.Duration)
	if err != nil {
		return 0, fmt.Errorf("create timeslot: %w", err)
	}
	return res.LastInsertId()
}

// AssignModerator links a moderator to a timeslot.
func (repo *TimeSlotRepo) AssignModerator(ctx context.Context, moderatorID, timeSlotID int64, isHost bool) (int64, error) {
	const q = `INSERT INTO moderator_time_slot (moderator_id, time_slot_id, is_host) VALUES (?, ?, ?)`
	res, err := repo.db.ExecContext(ctx, q, moderatorID, timeSlotID, isHost)
	if err != nil {
		return 0, fmt.Errorf("assign moderator to timeslot: %w", err)
	}
	return res.LastInsertId()
}

// Update modifies timeslot fields dynamically.
func (repo *TimeSlotRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	var setClauses []string
	var args []any
	for col, val := range fields {
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	args = append(args, id)
	q := "UPDATE time_slot SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	_, err := repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update timeslot %d: %w", id, err)
	}
	return nil
}

// Delete removes a timeslot (hard delete).
func (repo *TimeSlotRepo) Delete(ctx context.Context, id int64) error {
	// First remove junction rows
	if _, err := repo.db.ExecContext(ctx, "DELETE FROM moderator_time_slot WHERE time_slot_id = ?", id); err != nil {
		slog.ErrorContext(ctx, "delete moderator_time_slot failed", "time_slot_id", id, "error", err)
	}
	if _, err := repo.db.ExecContext(ctx, "DELETE FROM conference_invitation_responder_time_slot WHERE time_slot_id = ?", id); err != nil {
		slog.ErrorContext(ctx, "delete cirts failed", "time_slot_id", id, "error", err)
	}
	_, err := repo.db.ExecContext(ctx, "DELETE FROM time_slot WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete timeslot %d: %w", id, err)
	}
	return nil
}

// ListByModerator returns timeslots for a specific moderator.
func (repo *TimeSlotRepo) ListByModerator(ctx context.Context, moderatorID int64, page, pageSize int) ([]TimeSlotListRow, int, error) {
	return repo.List(ctx, page, pageSize, nil, nil, &moderatorID, nil, nil)
}

// ListByProject returns timeslots for a specific project.
func (repo *TimeSlotRepo) ListByProject(ctx context.Context, projectID int64, page, pageSize int) ([]TimeSlotListRow, int, error) {
	return repo.List(ctx, page, pageSize, &projectID, nil, nil, nil, nil)
}

// ListStatuses returns all timeslot statuses.
func (repo *TimeSlotRepo) ListStatuses(ctx context.Context) ([]TimeSlotStatus, error) {
	rows, err := repo.db.QueryContext(ctx, "SELECT id, description FROM time_slot_status ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list timeslot statuses: %w", err)
	}
	defer rows.Close()

	var result []TimeSlotStatus
	for rows.Next() {
		var s TimeSlotStatus
		if err := rows.Scan(&s.ID, &s.Description); err != nil {
			return nil, fmt.Errorf("scan timeslot status: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// BookingReward represents reward tracking for a timeslot/booking.
type BookingReward struct {
	TimeSlotID   int64  `json:"timeSlotId"`
	RewardPoints int    `json:"rewardPoints"`
	RewardStatus string `json:"rewardStatus"`
}

// GetReward returns reward info for a timeslot.
func (repo *TimeSlotRepo) GetReward(ctx context.Context, timeSlotID int64) (*BookingReward, error) {
	var br BookingReward
	err := repo.db.QueryRowContext(ctx,
		"SELECT time_slot_id, reward_points, reward_status FROM booking_reward WHERE time_slot_id = ?",
		timeSlotID,
	).Scan(&br.TimeSlotID, &br.RewardPoints, &br.RewardStatus)
	if err == sql.ErrNoRows {
		return &BookingReward{TimeSlotID: timeSlotID, RewardStatus: "not_credited"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reward %d: %w", timeSlotID, err)
	}
	return &br, nil
}

// UpsertReward inserts or updates reward info for a timeslot.
func (repo *TimeSlotRepo) UpsertReward(ctx context.Context, timeSlotID int64, points int, status string) error {
	q := `INSERT INTO booking_reward (time_slot_id, reward_points, reward_status)
	      VALUES (?, ?, ?)
	      ON DUPLICATE KEY UPDATE reward_points = VALUES(reward_points), reward_status = VALUES(reward_status)`
	_, err := repo.db.ExecContext(ctx, q, timeSlotID, points, status)
	if err != nil {
		return fmt.Errorf("upsert reward %d: %w", timeSlotID, err)
	}
	slog.InfoContext(ctx, "upserted booking reward", "timeSlotId", timeSlotID, "points", points, "status", status)
	return nil
}

// InvalidateInterviewMRA sets is_invalidated_interview=1 with reason and user info.
func (repo *TimeSlotRepo) InvalidateInterviewMRA(ctx context.Context, timeSlotID int64, reasonCode string, reasonText *string, invalidatedByUserID int64) error {
	q := `UPDATE time_slot
	      SET is_invalidated_interview = 1,
	          invalidation_reason_code = ?,
	          invalidation_reason_text = ?,
	          invalidated_by_user_id = ?,
	          updated_on = NOW()
	      WHERE id = ?`
	_, err := repo.db.ExecContext(ctx, q, reasonCode, reasonText, invalidatedByUserID, timeSlotID)
	if err != nil {
		return fmt.Errorf("invalidate interview %d: %w", timeSlotID, err)
	}
	return nil
}

// HasCompletedPaymentMRA checks if a timeslot has a COMPLETED payment record.
func (repo *TimeSlotRepo) HasCompletedPaymentMRA(ctx context.Context, timeSlotID int64) (bool, error) {
	const q = `SELECT EXISTS(
		SELECT 1 FROM time_slot_payment_history
		WHERE time_slot_id = ? AND payment_status = 'COMPLETED'
	)`
	var exists bool
	if err := repo.db.QueryRowContext(ctx, q, timeSlotID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check payment status for timeslot %d: %w", timeSlotID, err)
	}
	return exists, nil
}
