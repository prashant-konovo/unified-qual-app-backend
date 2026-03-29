package qs

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Answer maps to the QS answer table.
type Answer struct {
	ID           int64     `json:"id"`
	SurveyID     int64     `json:"surveyId"`
	ResponderID  int64     `json:"responderId"`
	TimeSlotID   int64     `json:"timeSlotId"`
	ModifiedOn   time.Time `json:"modifiedOn"`
}

// AnswerDetail maps to the QS answer_details table.
type AnswerDetail struct {
	ID                int64          `json:"id"`
	AnswerID          int64          `json:"answerId"`
	SurveyQuestionID  int64          `json:"surveyQuestionId"`
	TextResponse      sql.NullString `json:"textResponse"`
	SelectResponse    sql.NullString `json:"selectResponse"`
	NumericResponse   sql.NullInt64  `json:"numericResponse"`
	BooleanResponse   sql.NullBool   `json:"booleanResponse"`
	ModifiedOn        time.Time      `json:"modifiedOn"`
}

// AnswerRepo handles QS answer + answer_details queries.
type AnswerRepo struct {
	db *sql.DB
}

// NewAnswerRepo creates a new QS answer repository.
func NewAnswerRepo(db *sql.DB) *AnswerRepo {
	return &AnswerRepo{db: db}
}

// ListByTimeSlot returns answers for a timeslot.
func (r *AnswerRepo) ListByTimeSlot(ctx context.Context, timeSlotID int64) ([]map[string]any, error) {
	q := `SELECT a.id, a.survey_id, a.responder_id, a.time_slot_id, a.modified_on,
	       CONCAT(resp.first_name, ' ', resp.last_name) AS responder_name
	      FROM answer a
	      JOIN responder resp ON resp.id = a.responder_id
	      WHERE a.time_slot_id = ?`
	rows, err := r.db.QueryContext(ctx, q, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("list answers by timeslot: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var id, survID, respID, tsID int64
		var modOn time.Time
		var respName string
		if err := rows.Scan(&id, &survID, &respID, &tsID, &modOn, &respName); err != nil {
			return nil, fmt.Errorf("scan answer: %w", err)
		}
		result = append(result, map[string]any{
			"id": id, "surveyId": survID, "responderId": respID,
			"timeSlotId": tsID, "modifiedOn": modOn.Format(time.RFC3339),
			"responderName": respName,
		})
	}
	return result, rows.Err()
}

// GetDetails returns answer details for an answer.
func (r *AnswerRepo) GetDetails(ctx context.Context, answerID int64) ([]AnswerDetail, error) {
	q := `SELECT id, answer_id, survey_question_id, text_response, select_response,
	       numeric_response, boolean_response, modified_on
	      FROM answer_details WHERE answer_id = ? ORDER BY survey_question_id`
	rows, err := r.db.QueryContext(ctx, q, answerID)
	if err != nil {
		return nil, fmt.Errorf("get answer details: %w", err)
	}
	defer rows.Close()
	var result []AnswerDetail
	for rows.Next() {
		var d AnswerDetail
		if err := rows.Scan(&d.ID, &d.AnswerID, &d.SurveyQuestionID, &d.TextResponse,
			&d.SelectResponse, &d.NumericResponse, &d.BooleanResponse, &d.ModifiedOn); err != nil {
			return nil, fmt.Errorf("scan answer detail: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// LanguageLocalisation maps to the QS language_localisation table.
type LanguageLocalisation struct {
	ID                  int64  `json:"id"`
	Country             string `json:"country"`
	CountryCode         string `json:"countryCode"`
	LanguageName        string `json:"languageName"`
	LanguageCode        string `json:"languageCode"`
	LangCodeCountryCode string `json:"langCodeCountryCode"`
}

// ListTopics returns topics for a project (uses Topic defined in project_repo.go).
func (r *AnswerRepo) ListTopics(ctx context.Context, projectID int64) ([]Topic, error) {
	q := `SELECT id, topic_name, project_id, language_id, created_on, modified_on
	      FROM topics WHERE project_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()
	var result []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.TopicName, &t.ProjectID, &t.LanguageID, &t.CreatedOn, &t.ModifiedOn); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// ListLocales returns active language localisations.
func (r *AnswerRepo) ListLocales(ctx context.Context) ([]LanguageLocalisation, error) {
	q := `SELECT id, country, country_code, language_name, language_code, langCode_countryCode FROM language_localisation ORDER BY language_name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list locales: %w", err)
	}
	defer rows.Close()
	var result []LanguageLocalisation
	for rows.Next() {
		var l LanguageLocalisation
		if err := rows.Scan(&l.ID, &l.Country, &l.CountryCode, &l.LanguageName, &l.LanguageCode, &l.LangCodeCountryCode); err != nil {
			return nil, fmt.Errorf("scan locale: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

// HonorariumAmount maps to the QS honorarium_amount table.
type HonorariumAmount struct {
	ID         int64  `json:"id"`
	Amount     int    `json:"amount"`
	Currency   string `json:"currency"`
}

// PaymentHistory maps to the QS time_slot_payment_history table.
type PaymentHistory struct {
	ID           int64          `json:"id"`
	TimeSlotID   int64          `json:"timeSlotId"`
	AmountPaid   int            `json:"amountPaid"`
	PaidOn       time.Time      `json:"paidOn"`
	PaymentType  string         `json:"paymentType"`
	Status       string         `json:"status"`
	ExternalRef  sql.NullString `json:"externalRef"`
}

// QSSalesforceProject maps to the QS salesforce_project table.
type QSSalesforceProject struct {
	ID                  int64          `json:"id"`
	SalesforceProjectID string         `json:"salesforceProjectId"`
	Name                string         `json:"name"`
	Number              sql.NullString `json:"number"`
}

// QSSalesforceAccount maps to the QS salesforce_account table.
type QSSalesforceAccount struct {
	ID                   int64  `json:"id"`
	SalesforceAccountID  string `json:"salesforceAccountId"`
	Name                 string `json:"name"`
}

// ExternalClient maps to the QS external_client table.
type ExternalClient struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

// ListHonorariumAmounts returns all honorarium amounts.
func (r *AnswerRepo) ListHonorariumAmounts(ctx context.Context) ([]HonorariumAmount, error) {
	q := `SELECT id, amount, currency FROM honorarium_amount ORDER BY amount`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list honorarium amounts: %w", err)
	}
	defer rows.Close()
	var result []HonorariumAmount
	for rows.Next() {
		var h HonorariumAmount
		if err := rows.Scan(&h.ID, &h.Amount, &h.Currency); err != nil {
			return nil, fmt.Errorf("scan honorarium: %w", err)
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// ListPaymentHistory returns payment history for a timeslot.
func (r *AnswerRepo) ListPaymentHistory(ctx context.Context, timeSlotID int64) ([]PaymentHistory, error) {
	q := `SELECT id, time_slot_id, amount_paid, paid_on, payment_type, status, external_ref
	      FROM time_slot_payment_history WHERE time_slot_id = ? ORDER BY paid_on DESC`
	rows, err := r.db.QueryContext(ctx, q, timeSlotID)
	if err != nil {
		return nil, fmt.Errorf("list payment history: %w", err)
	}
	defer rows.Close()
	var result []PaymentHistory
	for rows.Next() {
		var p PaymentHistory
		if err := rows.Scan(&p.ID, &p.TimeSlotID, &p.AmountPaid, &p.PaidOn,
			&p.PaymentType, &p.Status, &p.ExternalRef); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// CreatePaymentRecord inserts a payment history record.
func (r *AnswerRepo) CreatePaymentRecord(ctx context.Context, timeSlotID int64, amount int, paymentType, status string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO time_slot_payment_history (time_slot_id, amount_paid, paid_on, payment_type, status)
		 VALUES (?, ?, NOW(), ?, ?)`, timeSlotID, amount, paymentType, status)
	if err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	return res.LastInsertId()
}

// CreateCustomHonorarium inserts a custom honorarium record.
func (r *AnswerRepo) CreateCustomHonorarium(ctx context.Context, timeSlotID int64, amount int, reason string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO time_slot_custom_honorarium (time_slot_id, amount, reason) VALUES (?, ?, ?)`,
		timeSlotID, amount, reason)
	if err != nil {
		return 0, fmt.Errorf("create custom honorarium: %w", err)
	}
	return res.LastInsertId()
}

// ListSalesforceProjects returns QS salesforce projects.
func (r *AnswerRepo) ListSalesforceProjects(ctx context.Context) ([]QSSalesforceProject, error) {
	q := `SELECT id, salesforce_project_id, name, number FROM salesforce_project ORDER BY name LIMIT 500`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list qs sf projects: %w", err)
	}
	defer rows.Close()
	var result []QSSalesforceProject
	for rows.Next() {
		var s QSSalesforceProject
		if err := rows.Scan(&s.ID, &s.SalesforceProjectID, &s.Name, &s.Number); err != nil {
			return nil, fmt.Errorf("scan qs sf project: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListSalesforceAccounts returns QS salesforce accounts.
func (r *AnswerRepo) ListSalesforceAccounts(ctx context.Context) ([]QSSalesforceAccount, error) {
	q := `SELECT id, salesforce_account_id, name FROM salesforce_account ORDER BY name LIMIT 500`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list qs sf accounts: %w", err)
	}
	defer rows.Close()
	var result []QSSalesforceAccount
	for rows.Next() {
		var s QSSalesforceAccount
		if err := rows.Scan(&s.ID, &s.SalesforceAccountID, &s.Name); err != nil {
			return nil, fmt.Errorf("scan qs sf account: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListExternalClients returns QS external clients.
func (r *AnswerRepo) ListExternalClients(ctx context.Context) ([]ExternalClient, error) {
	q := `SELECT id, name, is_active FROM external_client WHERE is_active = 1 ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list external clients: %w", err)
	}
	defer rows.Close()
	var result []ExternalClient
	for rows.Next() {
		var c ExternalClient
		if err := rows.Scan(&c.ID, &c.Name, &c.IsActive); err != nil {
			return nil, fmt.Errorf("scan external client: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// ListProjectsUsers returns users assigned to a project (QS projects_users table).
func (r *AnswerRepo) ListProjectsUsers(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT pu.project_id, pu.user_id,
	       u.first_name, u.last_name, u.email
	      FROM projects_users pu
	      JOIN user u ON u.id = pu.user_id
	      WHERE pu.project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project users: %w", err)
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var pID, uID int64
		var fn, ln sql.NullString
		var email sql.NullString
		if err := rows.Scan(&pID, &uID, &fn, &ln, &email); err != nil {
			return nil, fmt.Errorf("scan project user: %w", err)
		}
		result = append(result, map[string]any{
			"projectId": pID, "userId": uID,
			"firstName": fn.String, "lastName": ln.String, "email": email.String,
		})
	}
	return result, rows.Err()
}

// CommunicationTemplate maps to QS communication_template table.
type CommunicationTemplate struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	IsActive bool   `json:"isActive"`
}

// GetCommunicationTemplate returns a communication template by name.
func (r *AnswerRepo) GetCommunicationTemplate(ctx context.Context, name string) (*CommunicationTemplate, error) {
	q := `SELECT id, name, subject, body, is_active FROM communication_template WHERE name = ? AND is_active = 1`
	var t CommunicationTemplate
	err := r.db.QueryRowContext(ctx, q, name).Scan(&t.ID, &t.Name, &t.Subject, &t.Body, &t.IsActive)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	return &t, nil
}

// ListCommunicationTemplates returns all active communication templates.
func (r *AnswerRepo) ListCommunicationTemplates(ctx context.Context) ([]CommunicationTemplate, error) {
	q := `SELECT id, name, subject, body, is_active FROM communication_template WHERE is_active = 1 ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()
	var result []CommunicationTemplate
	for rows.Next() {
		var t CommunicationTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Subject, &t.Body, &t.IsActive); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// GetParticipantEligibility returns participant eligibility status.
func (r *AnswerRepo) GetParticipantEligibility(ctx context.Context, responderID, projectID int64) (map[string]any, error) {
	q := `SELECT id, responder_id, project_id, status, modified_on
	      FROM participant_eligibility_status WHERE responder_id = ? AND project_id = ?`
	var id int64
	var rID, pID int64
	var status string
	var modOn time.Time
	err := r.db.QueryRowContext(ctx, q, responderID, projectID).Scan(&id, &rID, &pID, &status, &modOn)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get eligibility: %w", err)
	}
	return map[string]any{
		"id": id, "responderId": rID, "projectId": pID,
		"status": status, "modifiedOn": modOn.Format(time.RFC3339),
	}, nil
}

// NativeSurvey represents a row from the native QS survey table.
type NativeSurvey struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"projectId"`
	CreatedBy int64     `json:"createdBy"`
	CreatedOn time.Time `json:"createdOn"`
	OwnedBy   int64     `json:"ownedBy"`
}

// ListNativeSurveysByProject returns native QS surveys for a project.
func (r *AnswerRepo) ListNativeSurveysByProject(ctx context.Context, projectID int64) ([]NativeSurvey, error) {
	q := `SELECT id, project_id, created_by, created_on, owned_by FROM survey WHERE project_id = ? ORDER BY created_on DESC`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list native surveys: %w", err)
	}
	defer rows.Close()
	var result []NativeSurvey
	for rows.Next() {
		var s NativeSurvey
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.CreatedBy, &s.CreatedOn, &s.OwnedBy); err != nil {
			return nil, fmt.Errorf("scan native survey: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
