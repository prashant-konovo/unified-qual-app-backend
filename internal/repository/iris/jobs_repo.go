package iris

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// JobsRepository defines DB operations for scheduled background jobs.
type JobsRepository interface {
	// IssueQualHonorarium
	GetPayableTimeSlots(ctx context.Context) ([]PayableTimeSlot, error)
	GrantCredit(ctx context.Context, params CreditParams) (int64, error)
	MarkHonorariumPaid(ctx context.Context, timeSlotID, creditID int64) error

	// CompleteProjects
	GetProjectsToComplete(ctx context.Context) ([]ProjectToComplete, error)
	CompleteProject(ctx context.Context, projectID int64) error

	// EndConferences
	GetStaleConferenceHashes(ctx context.Context, hashes []string) ([]string, error)

	// CloseSurveyJob
	GetSurveysToClose(ctx context.Context) ([]SurveyToClose, error)
	CloseSurveyByJob(ctx context.Context, surveyID int64) error
	SetSurveyToInReview(ctx context.Context, surveyID int64) error
	GetSurveyCrowdTypes(ctx context.Context, surveyID int64) ([]int64, error)
	LogActivity(ctx context.Context, params ActivityLogParams) error
	GetFeatureFlag(ctx context.Context, key string) (int64, error)

	// MonthlyTranscriptsJob
	GetPreviousMonthTranscripts(ctx context.Context) ([]TranscriptRecord, error)
	UpdateTranscriptTotal(ctx context.Context, id int64, total float64) error

	// RemindIntervieweesDayBefore + 30MinBefore
	GetTomorrowInterviews(ctx context.Context) ([]InterviewReminder, error)
	GetUpcomingInterviews(ctx context.Context) ([]InterviewReminder, error)

	// RemindToConfirmSchedule
	GetUnconfirmedAnswers(ctx context.Context) ([]UnconfirmedAnswer, error)

	// UpdateCalendarPushWatch
	GetCalendarWatchConfig(ctx context.Context) (*CalendarWatch, error)
	UpdateCalendarWatch(ctx context.Context, watchID, resourceID string, expiration time.Time) error
}

// Compile-time interface satisfaction check.
var _ JobsRepository = (*JobsRepo)(nil)

// ══════════════════════════════════════════════════════════════════
// Jobs Repository — DB operations for scheduled background jobs.
// Matches InCrowdAPI Scala actor queries (PaymentManager, QualInterviewManager,
// CloseSurveyJob, MonthlyTranscriptsJob, RemindInterviewees, etc.)
// ══════════════════════════════════════════════════════════════════

// ── Types ──────────────────────────────────────────────────────

type PayableTimeSlot struct {
	TimeSlotID         int64
	ProjectID          int64
	UserSurveyID       int64
	UserID             int64
	SurveyID           int64
	StartTime          time.Time
	PromisedHonorarium int64
	UserEmail          string
	UserTypeID         int64
	MarketRewards      bool
	SFProjectID        sql.NullString
	BrandTypeID        int64
}

type CreditParams struct {
	UserID       int64
	Amount       int64
	ReasonID     int64 // 7 = qual interview
	Reason       string
	UserSurveyID int64
	SystemUserID int64
}

type ProjectToComplete struct {
	ID int64
}

type SurveyToClose struct {
	SurveyID          int64
	SubscriptionID    int64
	SFProjectStatus   string
	SFProjectID       string
	NeedsToBeReviewed bool
	IsInReview        bool
	Status            int
}

type TranscriptRecord struct {
	ID                    int64
	TranscriptAudiofileID int64
}

type InterviewReminder struct {
	TimeSlotID      int64
	UserID          int64
	UserEmail       string
	StartTime       time.Time
	EndTime         time.Time
	ConferenceHash  string
	ParticipantHash string
	ProjectName     string
	TransportTypeID int
	PhoneNumber     sql.NullString
}

type UnconfirmedAnswer struct {
	AnswerID    int64
	TimeSlotID  int64
	GCalEventID sql.NullString
	UserEmail   string
}

type CalendarWatch struct {
	CalendarID string
	WatchID    string
	ResourceID string
	Expiration time.Time
}

type ActivityLogParams struct {
	ObjectType   string
	ObjectID     int64
	Action       string
	SubID        int64
	Description  string
	IndirectDesc string
}

// ── Repository ────────────────────────────────────────────────

type JobsRepo struct {
	db *sql.DB
	ro *sql.DB
}

func NewJobsRepo(db, ro *sql.DB) *JobsRepo {
	if ro == nil {
		ro = db
	}
	return &JobsRepo{db: db, ro: ro}
}

func (r *JobsRepo) readDB() *sql.DB {
	if r.ro != nil {
		return r.ro
	}
	return r.db
}

// ── IssueQualHonorarium ──────────────────────────────────────

const payableTimeSlotsSQL = `
SELECT ts.id, ts.project_id, us.id AS user_survey_id, us.user_id,
       a.survey_id, ts.start_time, ts.promised_honorarium,
       u.email, u.user_type_id, m.rewards,
       s.salesforce_project_id, sub.brand_type_id
FROM time_slot ts
JOIN answer_details ad ON ad.time_slot_id = ts.id
JOIN answer a ON a.id = ad.answer_id
JOIN user_survey us ON us.id = a.user_survey_id
JOIN ic_user u ON u.id = us.user_id
JOIN market m ON m.id = u.market_id
JOIN survey s ON s.id = a.survey_id
JOIN subscription sub ON sub.id = s.subscription_id
WHERE ts.stop_payment = 0
  AND ts.honorarium_paid = 0
  AND ts.start_time + INTERVAL 1 DAY < NOW()
  AND ts.promised_honorarium IS NOT NULL
  AND u.opted_out = 0
  AND us.is_invalid = 0
  AND us.is_test = 0`

func (r *JobsRepo) GetPayableTimeSlots(ctx context.Context) ([]PayableTimeSlot, error) {
	rows, err := r.readDB().QueryContext(ctx, payableTimeSlotsSQL)
	if err != nil {
		return nil, fmt.Errorf("GetPayableTimeSlots: %w", err)
	}
	defer rows.Close()

	var result []PayableTimeSlot
	for rows.Next() {
		var ts PayableTimeSlot
		if err := rows.Scan(
			&ts.TimeSlotID, &ts.ProjectID, &ts.UserSurveyID, &ts.UserID,
			&ts.SurveyID, &ts.StartTime, &ts.PromisedHonorarium,
			&ts.UserEmail, &ts.UserTypeID, &ts.MarketRewards,
			&ts.SFProjectID, &ts.BrandTypeID,
		); err != nil {
			return nil, fmt.Errorf("GetPayableTimeSlots scan: %w", err)
		}
		result = append(result, ts)
	}
	return result, rows.Err()
}

// GrantCredit creates a credit + credit_grant record and returns the credit ID.
// Matches Scala CreditOrder.grantCredit flow.
func (r *JobsRepo) GrantCredit(ctx context.Context, params CreditParams) (int64, error) {
	// Get the latest InCrowd internal credit order (company_id=3)
	var creditOrderID, subscriptionID, countryID int64
	err := r.readDB().QueryRowContext(ctx,
		`SELECT co.id, co.subscription_id
		 FROM credit_order co
		 WHERE co.company_id = 3
		 ORDER BY co.id DESC LIMIT 1`).Scan(&creditOrderID, &subscriptionID)
	if err != nil {
		return 0, fmt.Errorf("GrantCredit get credit order: %w", err)
	}

	// Get user's country
	err = r.readDB().QueryRowContext(ctx,
		`SELECT country_id FROM ic_user WHERE id = ?`, params.UserID).Scan(&countryID)
	if err != nil {
		return 0, fmt.Errorf("GrantCredit get user country: %w", err)
	}

	// Insert credit record
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO credit (credit_order_id, owner_id, credit_status_id, country_id,
		  created_on, subscription_id, price, created_by, transfer_note, cost,
		  transferred_on, user_survey_id)
		 VALUES (?, ?, 2, ?, NOW(), ?, 0, 3, 'Self-Replenishing', ?, NOW(), ?)`,
		creditOrderID, params.UserID, countryID, subscriptionID,
		params.Amount, params.UserSurveyID)
	if err != nil {
		return 0, fmt.Errorf("GrantCredit insert credit: %w", err)
	}
	creditID, _ := res.LastInsertId()

	// Insert credit_grant record
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO credit_grant (user_id, credit_id, reason_id, reason, given_by)
		 VALUES (?, ?, ?, ?, ?)`,
		params.UserID, creditID, params.ReasonID, params.Reason, params.SystemUserID)
	if err != nil {
		return 0, fmt.Errorf("GrantCredit insert credit_grant: %w", err)
	}

	return creditID, nil
}

func (r *JobsRepo) MarkHonorariumPaid(ctx context.Context, timeSlotID, creditID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE time_slot SET honorarium_paid = 1, credit_id = ?,
		  status_id = 7, status_modified_by = 1
		 WHERE id = ?`, creditID, timeSlotID)
	return err
}

// ── CompleteProjects ─────────────────────────────────────────

const projectsToCompleteSQL = `
SELECT p.id FROM project p
JOIN subscription sub ON sub.id = p.subscription_id
WHERE p.project_status_id = 7
  AND sub.plan_id = 23
  AND p.finalized_on IS NOT NULL
  AND TIMESTAMPDIFF(HOUR, p.finalized_on, UTC_TIMESTAMP()) >= 72`

func (r *JobsRepo) GetProjectsToComplete(ctx context.Context) ([]ProjectToComplete, error) {
	rows, err := r.readDB().QueryContext(ctx, projectsToCompleteSQL)
	if err != nil {
		return nil, fmt.Errorf("GetProjectsToComplete: %w", err)
	}
	defer rows.Close()

	var result []ProjectToComplete
	for rows.Next() {
		var p ProjectToComplete
		if err := rows.Scan(&p.ID); err != nil {
			return nil, fmt.Errorf("GetProjectsToComplete scan: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *JobsRepo) CompleteProject(ctx context.Context, projectID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE project SET completed_on = UTC_TIMESTAMP(), project_status_id = 13 WHERE id = ?`,
		projectID)
	return err
}

// ── EndConferences ───────────────────────────────────────────

func (r *JobsRepo) GetStaleConferenceHashes(ctx context.Context, hashes []string) ([]string, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(hashes))
	args := make([]any, len(hashes))
	for i, h := range hashes {
		placeholders[i] = "?"
		args[i] = h
	}
	query := fmt.Sprintf(
		`SELECT conference_hash FROM time_slot
		 WHERE conference_hash IN (%s)
		   AND end_time <= DATE_SUB(NOW(), INTERVAL 30 MINUTE)`,
		strings.Join(placeholders, ","))

	rows, err := r.readDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetStaleConferenceHashes: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		result = append(result, hash)
	}
	return result, rows.Err()
}

// ── CloseSurveyJob ──────────────────────────────────────────

const surveysToCloseSQL = `
SELECT s.id, s.subscription_id, sf.salesforce_project_status,
       sf.salesforce_project_id, s.needs_to_be_reviewed, s.status
FROM survey s
JOIN salesforce_project sf
  ON s.salesforce_project_id = sf.salesforce_project_id
  AND sf.salesforce_project_status IN ('Post Fieldwork','Complete','Cancelled')
JOIN subscription sub ON sub.id = s.subscription_id AND sub.brand_type_id = 2
WHERE s.status IN (1, 2, 7, 8)`

// Survey status constants matching InCrowdAPI
const (
	surveyStatusInReview = 7
	surveyStatusClosed   = 6
)

func (r *JobsRepo) GetSurveysToClose(ctx context.Context) ([]SurveyToClose, error) {
	rows, err := r.readDB().QueryContext(ctx, surveysToCloseSQL)
	if err != nil {
		return nil, fmt.Errorf("GetSurveysToClose: %w", err)
	}
	defer rows.Close()

	var result []SurveyToClose
	for rows.Next() {
		var s SurveyToClose
		if err := rows.Scan(
			&s.SurveyID, &s.SubscriptionID, &s.SFProjectStatus,
			&s.SFProjectID, &s.NeedsToBeReviewed, &s.Status,
		); err != nil {
			return nil, fmt.Errorf("GetSurveysToClose scan: %w", err)
		}
		s.IsInReview = s.Status == surveyStatusInReview
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *JobsRepo) CloseSurveyByJob(ctx context.Context, surveyID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE survey SET status = ?, closed_on = NOW() WHERE id = ?`,
		surveyStatusClosed, surveyID)
	return err
}

func (r *JobsRepo) SetSurveyToInReview(ctx context.Context, surveyID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE survey SET status = ? WHERE id = ?`,
		surveyStatusInReview, surveyID)
	return err
}

func (r *JobsRepo) GetSurveyCrowdTypes(ctx context.Context, surveyID int64) ([]int64, error) {
	rows, err := r.readDB().QueryContext(ctx,
		`SELECT c.type_id FROM survey_crowd sc
		 JOIN crowd c ON c.id = sc.crowd_id
		 WHERE sc.survey_id = ?`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []int64
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

func (r *JobsRepo) LogActivity(ctx context.Context, params ActivityLogParams) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO activity_log
		  (object_type, object_id, action, subscription_id,
		   object_description, indirect_object_description, created_on)
		 VALUES (?, ?, ?, ?, ?, ?, NOW())`,
		params.ObjectType, params.ObjectID, params.Action,
		params.SubID, params.Description, params.IndirectDesc)
	return err
}

func (r *JobsRepo) GetFeatureFlag(ctx context.Context, key string) (int64, error) {
	var val int64
	err := r.readDB().QueryRowContext(ctx,
		`SELECT value FROM api_configuration WHERE name = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return 1, nil // default enabled
	}
	return val, err
}

// ── MonthlyTranscriptsJob ───────────────────────────────────

func (r *JobsRepo) GetPreviousMonthTranscripts(ctx context.Context) ([]TranscriptRecord, error) {
	prevMonth := time.Now().AddDate(0, -1, 0)
	month := int(prevMonth.Month())
	year := prevMonth.Year()

	rows, err := r.readDB().QueryContext(ctx,
		`SELECT id, transcript_audiofile_id FROM qual_deliverable
		 WHERE transcript_status = 'Completed'
		   AND transcript_audiofile_id IS NOT NULL
		   AND MONTH(transcript_starttime) = ?
		   AND YEAR(transcript_starttime) = ?
		   AND total IS NULL`, month, year)
	if err != nil {
		return nil, fmt.Errorf("GetPreviousMonthTranscripts: %w", err)
	}
	defer rows.Close()

	var result []TranscriptRecord
	for rows.Next() {
		var t TranscriptRecord
		if err := rows.Scan(&t.ID, &t.TranscriptAudiofileID); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *JobsRepo) UpdateTranscriptTotal(ctx context.Context, id int64, total float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE qual_deliverable SET total = ? WHERE id = ?`, total, id)
	return err
}

// ── RemindIntervieweesDayBefore ─────────────────────────────

const interviewReminderBaseSQL = `
SELECT ts.id, us.user_id, u.email, ts.start_time, ts.end_time,
       ts.conference_hash, ci.participant_hash,
       p.name_public, ad.transport_type_id, u.telephone_number
FROM time_slot ts
JOIN answer_details ad ON ad.time_slot_id = ts.id
JOIN answer a ON a.id = ad.answer_id
JOIN user_survey us ON us.id = a.user_survey_id
  AND us.is_test = 0 AND us.is_invalid = 0
  AND us.user_survey_status_id = 3
JOIN ic_user u ON u.id = us.user_id
JOIN conference_invitation ci ON ci.time_slot_id = ts.id
  AND ci.user_id = us.user_id AND ci.role = 1
JOIN project p ON p.id = ts.project_id
WHERE ts.status_id = 2`

func (r *JobsRepo) GetTomorrowInterviews(ctx context.Context) ([]InterviewReminder, error) {
	query := interviewReminderBaseSQL + `
  AND DATE(ts.start_time) = DATE(DATE_ADD(NOW(), INTERVAL 1 DAY))`
	return r.scanInterviewReminders(ctx, query)
}

func (r *JobsRepo) GetUpcomingInterviews(ctx context.Context) ([]InterviewReminder, error) {
	query := interviewReminderBaseSQL + `
  AND ts.start_time BETWEEN DATE_ADD(NOW(), INTERVAL 28 MINUTE)
                        AND DATE_ADD(NOW(), INTERVAL 32 MINUTE)`
	return r.scanInterviewReminders(ctx, query)
}

func (r *JobsRepo) scanInterviewReminders(ctx context.Context, query string) ([]InterviewReminder, error) {
	rows, err := r.readDB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("scanInterviewReminders: %w", err)
	}
	defer rows.Close()

	var result []InterviewReminder
	for rows.Next() {
		var ir InterviewReminder
		if err := rows.Scan(
			&ir.TimeSlotID, &ir.UserID, &ir.UserEmail,
			&ir.StartTime, &ir.EndTime,
			&ir.ConferenceHash, &ir.ParticipantHash,
			&ir.ProjectName, &ir.TransportTypeID, &ir.PhoneNumber,
		); err != nil {
			return nil, fmt.Errorf("scanInterviewReminders scan: %w", err)
		}
		result = append(result, ir)
	}
	return result, rows.Err()
}

// ── RemindToConfirmSchedule ─────────────────────────────────

func (r *JobsRepo) GetUnconfirmedAnswers(ctx context.Context) ([]UnconfirmedAnswer, error) {
	rows, err := r.readDB().QueryContext(ctx,
		`SELECT a.id, ad.time_slot_id, tse.gcal_event_id, u.email
		 FROM answer a
		 JOIN answer_details ad ON ad.answer_id = a.id
		 JOIN user_survey us ON us.id = a.user_survey_id
		 JOIN ic_user u ON u.id = us.user_id
		 JOIN time_slot ts ON ts.id = ad.time_slot_id
		 LEFT JOIN time_slot_event tse ON tse.time_slot_id = ts.id AND tse.user_id = us.user_id
		 WHERE a.confirmed = 0
		   AND a.created_on BETWEEN DATE_SUB(NOW(), INTERVAL 4 DAY)
		                        AND DATE_SUB(NOW(), INTERVAL 1 DAY)
		   AND ts.start_time > NOW()`)
	if err != nil {
		return nil, fmt.Errorf("GetUnconfirmedAnswers: %w", err)
	}
	defer rows.Close()

	var result []UnconfirmedAnswer
	for rows.Next() {
		var ua UnconfirmedAnswer
		if err := rows.Scan(&ua.AnswerID, &ua.TimeSlotID, &ua.GCalEventID, &ua.UserEmail); err != nil {
			return nil, err
		}
		result = append(result, ua)
	}
	return result, rows.Err()
}

// ── UpdateCalendarPushWatch ─────────────────────────────────

func (r *JobsRepo) GetCalendarWatchConfig(ctx context.Context) (*CalendarWatch, error) {
	var cw CalendarWatch
	err := r.readDB().QueryRowContext(ctx,
		`SELECT calendar_id, watch_id, resource_id, expiration
		 FROM calendar_push_watch
		 ORDER BY id DESC LIMIT 1`).Scan(
		&cw.CalendarID, &cw.WatchID, &cw.ResourceID, &cw.Expiration)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetCalendarWatchConfig: %w", err)
	}
	return &cw, nil
}

func (r *JobsRepo) UpdateCalendarWatch(ctx context.Context, watchID, resourceID string, expiration time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO calendar_push_watch (calendar_id, watch_id, resource_id, expiration, created_on)
		 VALUES ((SELECT calendar_id FROM calendar_push_watch ORDER BY id DESC LIMIT 1),
		  ?, ?, ?, NOW())`,
		watchID, resourceID, expiration)
	return err
}
