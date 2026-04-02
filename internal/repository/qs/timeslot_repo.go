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

// GetInvalidTimeSlotStatusMRA returns invalid timeslot statuses for a respondent+project.
// Status IDs: 2=pending, 5=cancelled, 6=respondent_cancelled, 9=completed, 12=no_show.
func (repo *TimeSlotRepo) GetInvalidTimeSlotStatusMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error) {
	q := `SELECT t.status_id AS statusId FROM time_slot t
	      INNER JOIN answer_details ad ON ad.time_slot_id = t.id
	      INNER JOIN responder r ON ad.responder_id = r.id
	      AND r.external_responder_id = ? AND t.project_id = ?
	      AND t.status_id IN (2,5,6,9,12)
	      AND t.is_invalidated_interview = 0`
	rows, err := repo.db.QueryContext(ctx, q, externalResponderID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get invalid timeslot status mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var statusID int
		if err := rows.Scan(&statusID); err != nil {
			return nil, fmt.Errorf("scan invalid timeslot status: %w", err)
		}
		records = append(records, map[string]any{"statusId": statusID})
	}
	return records, rows.Err()
}

// GetInvalidTimeSlotStatusForRespRescMRA returns invalid statuses for respondent reschedule (excludes status 2).
func (repo *TimeSlotRepo) GetInvalidTimeSlotStatusForRespRescMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error) {
	q := `SELECT t.status_id AS statusId FROM time_slot t
	      INNER JOIN answer_details ad ON ad.time_slot_id = t.id
	      INNER JOIN responder r ON ad.responder_id = r.id
	      AND r.external_responder_id = ? AND t.project_id = ?
	      AND t.status_id IN (5,6,9,12)
	      AND t.is_invalidated_interview = 0`
	rows, err := repo.db.QueryContext(ctx, q, externalResponderID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get invalid timeslot status resp resc mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var statusID int
		if err := rows.Scan(&statusID); err != nil {
			return nil, fmt.Errorf("scan invalid timeslot status resp resc: %w", err)
		}
		records = append(records, map[string]any{"statusId": statusID})
	}
	return records, rows.Err()
}

// GetPendingTimeslotByProjectAndResponderMRA returns pending/invalidated timeslots for a responder.
func (repo *TimeSlotRepo) GetPendingTimeslotByProjectAndResponderMRA(ctx context.Context, projectID, responderID int64) ([]map[string]any, error) {
	q := `SELECT t.id, t.project_id, t.start_time, t.end_time, t.status_id,
	      t.is_invalidated_interview
	      FROM time_slot t
	      INNER JOIN answer_details ad ON t.id = ad.time_slot_id
	      WHERE t.project_id = ? AND ad.responder_id = ?
	      AND (t.status_id = 2 OR t.is_invalidated_interview = 1)`
	rows, err := repo.db.QueryContext(ctx, q, projectID, responderID)
	if err != nil {
		return nil, fmt.Errorf("get pending timeslot by project and responder mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var id, pID int64
		var st, et string
		var statusID int
		var isInvalidatedInterview bool
		if err := rows.Scan(&id, &pID, &st, &et, &statusID, &isInvalidatedInterview); err != nil {
			return nil, fmt.Errorf("scan pending timeslot: %w", err)
		}
		records = append(records, map[string]any{
			"id":                       id,
			"project_id":               pID,
			"start_time":               st,
			"end_time":                 et,
			"status_id":                statusID,
			"is_invalidated_interview": isInvalidatedInterview,
		})
	}
	return records, rows.Err()
}

// GetModeratorTimeSlotsMRA returns moderator timeslots matching legacy getModeratorTimeSlotsByModeratorId.
func (r *TimeSlotRepo) GetModeratorTimeSlotsMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
q := `SELECT t.id AS timeSlotId, t.start_time AS startTime, t.end_time AS endTime,
t.status_id AS completed, t.duration,
t.is_invalidated_interview AS isInvalidatedInterview,
t.is_invalidate_email_sent AS isInvalidateEmailSent,
CASE WHEN EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id = t.id AND tsph.payment_status = 'COMPLETED') THEN true ELSE false END AS paymentStatus,
(CASE WHEN t.has_imported_overlap IS NULL THEN '0' ELSE t.has_imported_overlap END) AS importedOverLap,
ad.responder_id AS intervieweeId,
t.project_id AS projectId,
COALESCE(topics.topic_name, '') AS topicName,
p.name AS projectName
FROM time_slot t
INNER JOIN project p ON p.id = t.project_id
LEFT JOIN topics ON p.id = topics.project_id AND topics.language_id = 1
INNER JOIN answer_details ad ON ad.time_slot_id = t.id
WHERE t.id IN (SELECT time_slot_id FROM moderator_time_slot WHERE moderator_id = ?)
AND t.status_id IN (2, 7, 8, 9) AND p.client_id = ?`
rows, err := r.db.QueryContext(ctx, q, moderatorID, clientID)
if err != nil {
return nil, fmt.Errorf("get moderator timeslots mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var tsID, projectID, intervieweeID int64
var completed, duration int
var isInvalidatedInterview, isInvalidateEmailSent, paymentStatus bool
var importedOverLap, topicName, projectName, startTime, endTime string
if err := rows.Scan(&tsID, &startTime, &endTime, &completed, &duration,
&isInvalidatedInterview, &isInvalidateEmailSent, &paymentStatus,
&importedOverLap, &intervieweeID, &projectID, &topicName, &projectName); err != nil {
return nil, fmt.Errorf("scan moderator timeslot mra: %w", err)
}
records = append(records, map[string]any{
"timeSlotId": tsID, "startTime": startTime, "endTime": endTime,
"completed": completed, "duration": duration,
"isInvalidatedInterview": isInvalidatedInterview,
"isInvalidateEmailSent": isInvalidateEmailSent,
"paymentStatus": paymentStatus, "importedOverLap": importedOverLap,
"intervieweeId": intervieweeID, "projectId": projectID,
"topicName": topicName, "projectName": projectName,
})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}

// GetModeratorTimeSlotsWithFilterMRA returns moderator timeslots with project exclusion filter.
func (r *TimeSlotRepo) GetModeratorTimeSlotsWithFilterMRA(ctx context.Context, moderatorID, clientID int64, excludeProjectIDs []int64) ([]map[string]any, error) {
if len(excludeProjectIDs) == 0 {
return r.GetModeratorTimeSlotsMRA(ctx, moderatorID, clientID)
}
placeholders := make([]string, len(excludeProjectIDs))
args := []any{moderatorID}
for i, pid := range excludeProjectIDs {
placeholders[i] = "?"
args = append(args, pid)
}
args = append(args, clientID)

q := fmt.Sprintf(`SELECT t.id AS timeSlotId, t.start_time AS startTime, t.end_time AS endTime,
t.status_id AS completed, t.duration,
(CASE WHEN t.has_imported_overlap IS NULL THEN '0' ELSE t.has_imported_overlap END) AS importedOverLap,
ad.responder_id AS intervieweeId,
t.project_id AS projectId,
p.name AS projectName,
COALESCE(topics.topic_name, '') AS topicName
FROM time_slot t
INNER JOIN project p ON p.id = t.project_id
INNER JOIN answer_details ad ON ad.time_slot_id = t.id
LEFT JOIN topics ON t.project_id = topics.project_id AND topics.language_id = 1
WHERE t.id IN (SELECT time_slot_id FROM moderator_time_slot WHERE moderator_id = ?)
AND t.status_id IN (2, 7, 8, 9)
AND t.project_id NOT IN (%s)
AND p.client_id = ?`, strings.Join(placeholders, ","))
rows, err := r.db.QueryContext(ctx, q, args...)
if err != nil {
return nil, fmt.Errorf("get moderator timeslots with filter mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var tsID, projectID, intervieweeID int64
var completed, duration int
var importedOverLap, topicName, projectName, startTime, endTime string
if err := rows.Scan(&tsID, &startTime, &endTime, &completed, &duration,
&importedOverLap, &intervieweeID, &projectID, &projectName, &topicName); err != nil {
return nil, fmt.Errorf("scan moderator timeslot with filter mra: %w", err)
}
records = append(records, map[string]any{
"timeSlotId": tsID, "startTime": startTime, "endTime": endTime,
"completed": completed, "duration": duration,
"importedOverLap": importedOverLap,
"intervieweeId": intervieweeID, "projectId": projectID,
"projectName": projectName, "topicName": topicName,
})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}

// GetAllInterviewsMRA returns all interviews for a moderator matching legacy getAllInterviews query.
// Supports search, project exclusion, and payment status filtering.
func (r *TimeSlotRepo) GetAllInterviewsMRA(ctx context.Context, moderatorID int64, search string, excludeProjectIDs []int64, paymentStatusCode string) ([]map[string]any, error) {
baseSelect := `SELECT project.external_survey_id AS externalSurveyId,
COALESCE(salesforce_account.name, '') AS clientName,
responder.external_responder_id AS participantId,
moderator_info.id AS moderatorId,
time_slot.duration,
(CASE WHEN time_slot.has_imported_overlap IS NULL THEN '0' ELSE time_slot.has_imported_overlap END) AS importedOverLap,
time_slot.start_time AS startTime,
time_slot.end_time AS endTime,
time_slot.id AS timeSlotId,
COALESCE(time_slot.conference_hash, '') AS conferenceHash,
moderator_info.first_name AS firstName,
moderator_info.last_name AS lastName,
project.name AS projectName,
project.id AS projectId,
COALESCE(topics.topic_name, '') AS topicName,
COALESCE(conference_invitation.conference_link, '') AS conferenceLink,
COALESCE(project.salesforce_job_number, '') AS salesforceJobNumber,
COALESCE(defaultHonorarium.amount, 0) AS defaultHonorariumAmount,
COALESCE(defaultHonorarium.currency, '') AS defaultHonorariumCurrency,
COALESCE(customHonorarium.new_value, 0) AS customHonorariumAmount,
COALESCE(payments.totalAmount, 0) AS totalAmount,
COALESCE(payments.paymentDate, '') AS paymentDate,
COALESCE(payments.source, '') AS source,
COALESCE(payments.currency, '') AS currency,
COALESCE(responder.first_name, '') AS participantFirstName,
COALESCE(responder.last_name, '') AS participantLastName,
(CASE
WHEN EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id=time_slot.id AND tsph.payment_status='COMPLETED') THEN 'CREDITED'
WHEN NOT EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id=time_slot.id AND tsph.payment_status='COMPLETED') AND time_slot.end_time < CURDATE() - INTERVAL 7 DAY THEN 'PAST_DUE'
ELSE 'NOT_CREDITED' END) AS paymentStatus`

baseFrom := `
FROM responder
INNER JOIN answer_details ON responder.id = answer_details.responder_id
INNER JOIN time_slot ON time_slot.id = answer_details.time_slot_id
INNER JOIN (SELECT user.id AS id, user.first_name AS first_name, user.last_name AS last_name,
moderator_time_slot.time_slot_id FROM moderator_time_slot
INNER JOIN user ON user.id = moderator_time_slot.moderator_id
WHERE moderator_time_slot.is_host) moderator_info ON moderator_info.time_slot_id = time_slot.id
INNER JOIN project ON time_slot.project_id = project.id
LEFT JOIN topics ON project.id = topics.project_id AND topics.language_id = 1
INNER JOIN conference_invitation_responder_time_slot ON conference_invitation_responder_time_slot.time_slot_id = time_slot.id
INNER JOIN conference_invitation ON conference_invitation.id = conference_invitation_responder_time_slot.conference_invitation_id
INNER JOIN client ON client.id = project.client_id
LEFT JOIN external_client ON project.project_external_client = external_client.id
LEFT JOIN salesforce_account ON salesforce_account.salesforce_account_id = external_client.external_client_account_id
LEFT JOIN (SELECT SUM(tsph.amount) AS totalAmount, MAX(tsph.payment_date) AS paymentDate,
MAX(tsph.source) AS source, MAX(tsph.currency) AS currency, time_slot_id
FROM time_slot_payment_history tsph WHERE tsph.payment_status='COMPLETED'
GROUP BY tsph.time_slot_id) payments ON payments.time_slot_id=time_slot.id
LEFT JOIN (SELECT MAX(ha.honorarium) AS amount, MAX(ha.currency) AS currency, ts.id AS timeSlotId
FROM time_slot ts JOIN time_slot_event tse ON tse.time_slot_id = ts.id
JOIN responder r ON r.id = tse.responder_id JOIN honorarium_amount ha ON ha.sessKey = r.sess_key
GROUP BY ts.id) defaultHonorarium ON defaultHonorarium.timeSlotId = time_slot.id
LEFT JOIN time_slot_custom_honorarium customHonorarium ON customHonorarium.time_slot_id = time_slot.id`

where := " WHERE moderator_info.id = ? AND time_slot.status_id IN (2, 7, 8, 9)"
args := []any{moderatorID}

if search != "" {
where += " AND (project.name LIKE ? OR project.salesforce_job_number LIKE ? OR moderator_info.first_name LIKE ? OR moderator_info.last_name LIKE ?)"
likeVal := "%" + search + "%"
args = append(args, likeVal, likeVal, likeVal, likeVal)
}

if len(excludeProjectIDs) > 0 {
ph := make([]string, len(excludeProjectIDs))
for i, pid := range excludeProjectIDs {
ph[i] = "?"
args = append(args, pid)
}
where += " AND project.id NOT IN (" + strings.Join(ph, ",") + ")"
}

switch paymentStatusCode {
case "CREDITED":
where += " AND EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id=time_slot.id AND tsph.payment_status='COMPLETED')"
case "NOT_CREDITED":
where += " AND NOT EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id=time_slot.id AND tsph.payment_status='COMPLETED')"
case "PAST_DUE":
where += " AND NOT EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id=time_slot.id AND tsph.payment_status='COMPLETED') AND time_slot.end_time < CURDATE() - INTERVAL 7 DAY"
}

q := baseSelect + baseFrom + where + " ORDER BY time_slot.start_time ASC"
rows, err := r.db.QueryContext(ctx, q, args...)
if err != nil {
return nil, fmt.Errorf("get all interviews mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var (
externalSurveyID, clientName, importedOverLap, startTime, endTime                                          string
conferenceHash, firstName, lastName, projectName, topicName, conferenceLink, salesforceJobNumber            string
defaultHonorariumCurrency, paymentDate, paymentSource, paymentCurrency, participantFirstName, participantLastName string
paymentStatus                                                                                               string
participantID                                                                                               string
moderatorIDResult, duration, timeSlotID, projectID                                                          int64
defaultHonorariumAmount, customHonorariumAmount, totalAmount                                                float64
)
if err := rows.Scan(
&externalSurveyID, &clientName, &participantID, &moderatorIDResult, &duration,
&importedOverLap, &startTime, &endTime, &timeSlotID, &conferenceHash,
&firstName, &lastName, &projectName, &projectID, &topicName, &conferenceLink,
&salesforceJobNumber, &defaultHonorariumAmount, &defaultHonorariumCurrency,
&customHonorariumAmount, &totalAmount, &paymentDate, &paymentSource, &paymentCurrency,
&participantFirstName, &participantLastName, &paymentStatus,
); err != nil {
return nil, fmt.Errorf("scan interview mra: %w", err)
}
records = append(records, map[string]any{
"externalSurveyId": externalSurveyID, "clientName": clientName,
"participantId": participantID, "moderatorId": moderatorIDResult,
"duration": duration, "importedOverLap": importedOverLap,
"startTime": startTime, "endTime": endTime,
"timeSlotId": timeSlotID, "conferenceHash": conferenceHash,
"firstName": firstName, "lastName": lastName,
"projectName": projectName, "projectId": projectID,
"topicName": topicName, "conferenceLink": conferenceLink,
"salesforceJobNumber": salesforceJobNumber,
"defaultHonorariumAmount": defaultHonorariumAmount,
"defaultHonorariumCurrency": defaultHonorariumCurrency,
"customHonorariumAmount": customHonorariumAmount,
"totalAmount": totalAmount, "paymentDate": paymentDate,
"source": paymentSource, "currency": paymentCurrency,
"participantFirstName": participantFirstName,
"participantLastName": participantLastName,
"paymentStatus": paymentStatus,
})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}

// GetModeratorsForTimeSlotMRA returns moderators for a timeslot matching legacy camelCase fields.
func (r *TimeSlotRepo) GetModeratorsForTimeSlotMRA(ctx context.Context, timeslotID int64) ([]map[string]any, error) {
q := `SELECT time_slot.id AS timeSlotId,
moderator_time_slot.moderator_id AS moderatorId,
moderator_time_slot.is_host AS isHost
FROM time_slot
INNER JOIN project ON time_slot.project_id = project.id
INNER JOIN moderator_time_slot ON moderator_time_slot.time_slot_id = time_slot.id
WHERE time_slot.id = ?`
rows, err := r.db.QueryContext(ctx, q, timeslotID)
if err != nil {
return nil, fmt.Errorf("get moderators for timeslot mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var timeSlotID, moderatorID int64
var isHost bool
if err := rows.Scan(&timeSlotID, &moderatorID, &isHost); err != nil {
return nil, err
}
records = append(records, map[string]any{
"timeSlotId":  timeSlotID,
"moderatorId": moderatorID,
"isHost":      isHost,
})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}

// GetStartEndTimeBySlotIdMRA returns start_time and end_time for a timeslot.
func (r *TimeSlotRepo) GetStartEndTimeBySlotIdMRA(ctx context.Context, timeslotID int64) (string, string, error) {
q := `SELECT start_time, end_time FROM time_slot WHERE id = ?`
var startTime, endTime string
err := r.db.QueryRowContext(ctx, q, timeslotID).Scan(&startTime, &endTime)
if err != nil {
return "", "", fmt.Errorf("get start end time by slot id mra: %w", err)
}
return startTime, endTime, nil
}

// ModeratorInfoForSlotMRA holds moderator info for a timeslot.
type ModeratorInfoForSlotMRA struct {
ID              int64
IsHost          bool
FirstName       string
LastName        string
}

// GetModeratorsInfoForSlotMRA returns moderator info for a timeslot.
func (r *TimeSlotRepo) GetModeratorsInfoForSlotMRA(ctx context.Context, timeslotID int64) ([]ModeratorInfoForSlotMRA, error) {
q := `SELECT moderator_id, is_host, first_name, last_name
FROM moderator_time_slot
INNER JOIN user ON user.id = moderator_time_slot.moderator_id
WHERE time_slot_id = ?`
rows, err := r.db.QueryContext(ctx, q, timeslotID)
if err != nil {
return nil, fmt.Errorf("get moderators info for slot mra: %w", err)
}
defer rows.Close()
var results []ModeratorInfoForSlotMRA
for rows.Next() {
var m ModeratorInfoForSlotMRA
if err := rows.Scan(&m.ID, &m.IsHost, &m.FirstName, &m.LastName); err != nil {
return nil, err
}
results = append(results, m)
}
return results, rows.Err()
}

// GetModeratorConflictForSlotMRA checks if a moderator has a conflict with a timeslot.
func (r *TimeSlotRepo) GetModeratorConflictForSlotMRA(ctx context.Context, moderatorID int64, startTime, endTime string, timeslotID int64) (int, error) {
q := `SELECT COUNT(time_slot.id) AS hasConflict
FROM time_slot
INNER JOIN moderator_time_slot ON time_slot.id = moderator_time_slot.time_slot_id
WHERE moderator_time_slot.moderator_id = ?
AND time_slot.is_invalid = FALSE
AND time_slot.status_id IN (2, 7, 8, 9)
AND time_slot.id != ?
AND (
(time_slot.start_time >= ? AND time_slot.start_time < ?)
OR (time_slot.end_time > ? AND time_slot.end_time <= ?)
)`
var hasConflict int
err := r.db.QueryRowContext(ctx, q, moderatorID, timeslotID, startTime, endTime, startTime, endTime).Scan(&hasConflict)
if err != nil {
return 0, fmt.Errorf("get moderator conflict for slot mra: %w", err)
}
return hasConflict, nil
}

// ──────────────────────────────────────────────
// MRA #55 — PM Timeslots
// ──────────────────────────────────────────────

const pmTimeSlotBaseQueryMRA = `SELECT t.id AS timeSlotId, t.start_time AS startTime, t.end_time AS endTime,
t.status_id AS completed, t.duration,
t.is_invalidated_interview AS isInvalidatedInterview,
t.is_invalidate_email_sent AS isInvalidateEmailSent,
(CASE WHEN EXISTS (SELECT 1 FROM time_slot_payment_history tsph WHERE tsph.time_slot_id = t.id AND tsph.payment_status = 'COMPLETED') THEN TRUE ELSE FALSE END) AS paymentStatus,
(CASE WHEN t.has_imported_overlap IS NULL THEN '0' ELSE t.has_imported_overlap END) AS importedOverLap,
ad.responder_id AS intervieweeId, t.project_id AS projectId, p.name AS projectName,
COALESCE(topics.topic_name, '') AS topicName, mt.moderator_id AS moderatorId
FROM time_slot t
INNER JOIN project p ON p.id = t.project_id
INNER JOIN moderator_time_slot mt ON mt.time_slot_id = t.id
INNER JOIN answer_details ad ON ad.time_slot_id = t.id
LEFT JOIN topics ON p.id = topics.project_id AND topics.language_id = 1
WHERE p.client_id = ? AND t.status_id IN (2, 7, 8, 9)`

func (repo *TimeSlotRepo) scanPMTimeSlotRows(rows *sql.Rows) ([]map[string]any, error) {
	defer rows.Close()
	var records []map[string]any
	for rows.Next() {
		var tsID, projectID, intervieweeID, moderatorID int64
		var completed, duration int
		var isInvalidatedInterview, isInvalidateEmailSent, paymentStatus bool
		var importedOverLap, topicName, projectName, startTime, endTime string
		if err := rows.Scan(&tsID, &startTime, &endTime, &completed, &duration,
			&isInvalidatedInterview, &isInvalidateEmailSent, &paymentStatus,
			&importedOverLap, &intervieweeID, &projectID, &projectName,
			&topicName, &moderatorID); err != nil {
			return nil, fmt.Errorf("scan pm timeslot mra: %w", err)
		}
		records = append(records, map[string]any{
			"timeSlotId": tsID, "startTime": startTime, "endTime": endTime,
			"completed": completed, "duration": duration,
			"isInvalidatedInterview": isInvalidatedInterview,
			"isInvalidateEmailSent":  isInvalidateEmailSent,
			"paymentStatus": paymentStatus, "importedOverLap": importedOverLap,
			"intervieweeId": intervieweeID, "projectId": projectID,
			"projectName": projectName, "topicName": topicName,
			"moderatorId": moderatorID,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// GetPMTimeSlotsByClientIdMRA returns PM timeslots for a client (no filter).
func (repo *TimeSlotRepo) GetPMTimeSlotsByClientIdMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	rows, err := repo.db.QueryContext(ctx, pmTimeSlotBaseQueryMRA, clientID)
	if err != nil {
		return nil, fmt.Errorf("get pm timeslots by client id mra: %w", err)
	}
	return repo.scanPMTimeSlotRows(rows)
}

// GetPMTimeSlotsByClientIdWithProjectFilterMRA returns PM timeslots excluding specified projects.
func (repo *TimeSlotRepo) GetPMTimeSlotsByClientIdWithProjectFilterMRA(ctx context.Context, clientID int64, projectIDs []int64) ([]map[string]any, error) {
	if len(projectIDs) == 0 {
		return repo.GetPMTimeSlotsByClientIdMRA(ctx, clientID)
	}
	placeholders := make([]string, len(projectIDs))
	args := []any{clientID}
	for i, id := range projectIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := pmTimeSlotBaseQueryMRA + " AND p.id NOT IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get pm timeslots with project filter mra: %w", err)
	}
	return repo.scanPMTimeSlotRows(rows)
}

// GetPMTimeSlotsByClientIdWithModeratorFilterMRA returns PM timeslots excluding specified moderators.
func (repo *TimeSlotRepo) GetPMTimeSlotsByClientIdWithModeratorFilterMRA(ctx context.Context, clientID int64, moderatorIDs []int64) ([]map[string]any, error) {
	if len(moderatorIDs) == 0 {
		return repo.GetPMTimeSlotsByClientIdMRA(ctx, clientID)
	}
	placeholders := make([]string, len(moderatorIDs))
	args := []any{clientID}
	for i, id := range moderatorIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := pmTimeSlotBaseQueryMRA + " AND mt.moderator_id NOT IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get pm timeslots with moderator filter mra: %w", err)
	}
	return repo.scanPMTimeSlotRows(rows)
}

// GetAllPendingInterviewsPerProjectMRA returns pending interviews for a project grouped by moderator.
func (repo *TimeSlotRepo) GetAllPendingInterviewsPerProjectMRA(ctx context.Context, projectID, clientID int64) ([]map[string]any, error) {
	q := `SELECT t.id AS timeSlotId, t.start_time AS startTime, t.end_time AS endTime,
ad.responder_id AS participantId, t.project_id AS projectId, p.name AS projectName,
p.salesforce_job_number AS salesforce, mt.moderator_id AS moderatorId,
us.first_name AS firstName, us.last_name AS lastName
FROM time_slot t
INNER JOIN project p ON p.id = t.project_id
INNER JOIN moderator_time_slot mt ON mt.time_slot_id = t.id
INNER JOIN user us ON us.id = mt.moderator_id
INNER JOIN answer_details ad ON ad.time_slot_id = t.id
WHERE p.client_id = ? AND t.status_id = 2 AND p.id = ?
AND mt.moderator_id IN (SELECT moderator_id FROM projects_users WHERE project_id = ?)`
	rows, err := repo.db.QueryContext(ctx, q, clientID, projectID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get all pending interviews per project mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var tsID, participantID, pID, moderatorID int64
		var projectName, salesforce, firstName, lastName, startTime, endTime string
		if err := rows.Scan(&tsID, &startTime, &endTime,
			&participantID, &pID, &projectName,
			&salesforce, &moderatorID,
			&firstName, &lastName); err != nil {
			return nil, fmt.Errorf("scan pending interview per project mra: %w", err)
		}
		records = append(records, map[string]any{
			"timeSlotId":    tsID,
			"startTime":     startTime,
			"endTime":       endTime,
			"participantId": participantID,
			"projectId":     pID,
			"projectName":   projectName,
			"salesforce":    salesforce,
			"moderatorId":   moderatorID,
			"firstName":     firstName,
			"lastName":      lastName,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}
