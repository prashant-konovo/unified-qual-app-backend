package qs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Project maps to the QS `project` table (18 columns verified).
type Project struct {
	ID                    int64          `json:"id"`
	Name                  string         `json:"name"`
	ExternalSurveyID      sql.NullString `json:"externalSurveyId"`
	SalesforceJobNumber   sql.NullString `json:"salesforceJobNumber"`
	ClientID              sql.NullInt64  `json:"clientId"`
	SampleSize            sql.NullInt64  `json:"sampleSize"`
	CreatedOn             time.Time      `json:"createdOn"`
	CreatedBy             sql.NullInt64  `json:"createdBy"`
	InterviewLength       sql.NullInt64  `json:"interviewLength"`
	QualModeratorID       sql.NullInt64  `json:"qualModeratorId"`
	ModifiedOn            sql.NullTime   `json:"modifiedOn"`
	ModifiedBy            sql.NullInt64  `json:"modifiedBy"`
	ProjectStatusID       int            `json:"projectStatusId"`
	SchedulerGenerated    bool           `json:"schedulerGenerated"`
	PostScreeninBuffer    sql.NullString `json:"postScreeninBuffer"` // decimal stored as string
	ModeratorBuffer       sql.NullString `json:"moderatorBuffer"`    // decimal stored as string
	Shghash               sql.NullString `json:"shghash"`
	ProjectExternalClient sql.NullString `json:"projectExternalClient"`
}

// ProjectListRow is a flattened row for list queries.
type ProjectListRow struct {
	ID                  int64          `json:"id"`
	Name                string         `json:"name"`
	SalesforceJobNumber sql.NullString `json:"salesforceJobNumber"`
	ClientID            sql.NullInt64  `json:"clientId"`
	ClientCompany       sql.NullString `json:"clientCompany"`
	SampleSize          sql.NullInt64  `json:"sampleSize"`
	InterviewLength     sql.NullInt64  `json:"interviewLength"`
	ProjectStatusID     int            `json:"projectStatusId"`
	ProjectStatusName   string         `json:"projectStatusName"`
	CreatedOn           time.Time      `json:"createdOn"`
	ModifiedOn          sql.NullTime   `json:"modifiedOn"`
	ScheduledCount      int            `json:"scheduledCount"`
	CompletedCount      int            `json:"completedCount"`
}

// Topic maps to the QS `topics` table.
type Topic struct {
	ID         int64          `json:"id"`
	TopicName  string         `json:"topicName"`
	ProjectID  int64          `json:"projectId"`
	LanguageID sql.NullInt64  `json:"languageId"`
	CreatedOn  sql.NullTime   `json:"createdOn"`
	ModifiedOn sql.NullTime   `json:"modifiedOn"`
}

// ProjectRepo provides CRUD for the QS project table.
type ProjectRepo struct {
	db *sql.DB
}

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

const qsProjectListQuery = `
SELECT p.id, p.name, p.salesforce_job_number, p.client_id,
       c.company AS client_company,
       p.sample_size, p.interview_length,
       p.project_status_id, ps.name AS project_status_name,
       p.created_on, p.modified_on,
       COALESCE(sch.scheduled_count, 0) AS scheduled_count,
       COALESCE(comp.completed_count, 0) AS completed_count
FROM project p
JOIN project_status ps ON ps.id = p.project_status_id
LEFT JOIN client c ON c.id = p.client_id
LEFT JOIN (
    SELECT ts.project_id, COUNT(*) AS scheduled_count
    FROM time_slot ts
    WHERE ts.status_id IN (2,3,4,5)
    GROUP BY ts.project_id
) sch ON sch.project_id = p.id
LEFT JOIN (
    SELECT ts.project_id, COUNT(*) AS completed_count
    FROM time_slot ts
    WHERE ts.status_id = 5
    GROUP BY ts.project_id
) comp ON comp.project_id = p.id
WHERE 1=1
`

// List returns QS projects with pagination and optional filters.
func (r *ProjectRepo) List(ctx context.Context, page, pageSize int, statusID *int, search string) ([]ProjectListRow, int, error) {
	where := ""
	args := []any{}

	if statusID != nil {
		where += " AND p.project_status_id = ?"
		args = append(args, *statusID)
	}
	if search != "" {
		where += " AND p.name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	// Count
	countQ := "SELECT COUNT(*) FROM project p WHERE 1=1" + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count qs projects: %w", err)
	}

	// Paginated list
	offset := (page - 1) * pageSize
	listQ := qsProjectListQuery + where + " ORDER BY p.created_on DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list qs projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectListRow
	for rows.Next() {
		var p ProjectListRow
		if err := rows.Scan(
			&p.ID, &p.Name, &p.SalesforceJobNumber, &p.ClientID,
			&p.ClientCompany,
			&p.SampleSize, &p.InterviewLength,
			&p.ProjectStatusID, &p.ProjectStatusName,
			&p.CreatedOn, &p.ModifiedOn,
			&p.ScheduledCount, &p.CompletedCount,
		); err != nil {
			return nil, 0, fmt.Errorf("scan qs project row: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate qs project rows: %w", err)
	}

	slog.Debug("qs projects listed", "count", len(projects), "total", total, "page", page)
	return projects, total, nil
}

// GetByID returns a single QS project.
func (r *ProjectRepo) GetByID(ctx context.Context, id int64) (*Project, error) {
	q := `SELECT id, name, external_survey_id, salesforce_job_number, client_id,
	             sample_size, created_on, created_by, interview_length, qual_moderator_id,
	             modified_on, modified_by, project_status_id, scheduler_generated,
	             post_screenin_buffer, moderator_buffer, shghash, project_external_client
	      FROM project WHERE id = ?`

	var p Project
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.Name, &p.ExternalSurveyID, &p.SalesforceJobNumber, &p.ClientID,
		&p.SampleSize, &p.CreatedOn, &p.CreatedBy, &p.InterviewLength, &p.QualModeratorID,
		&p.ModifiedOn, &p.ModifiedBy, &p.ProjectStatusID, &p.SchedulerGenerated,
		&p.PostScreeninBuffer, &p.ModeratorBuffer, &p.Shghash, &p.ProjectExternalClient,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get qs project %d: %w", id, err)
	}
	return &p, nil
}

// GetTopics returns topics for a QS project.
func (r *ProjectRepo) GetTopics(ctx context.Context, projectID int64) ([]Topic, error) {
	q := `SELECT id, topic_name, project_id, language_id, created_on, modified_on
	      FROM topics WHERE project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get qs topics for project %d: %w", projectID, err)
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.TopicName, &t.ProjectID, &t.LanguageID, &t.CreatedOn, &t.ModifiedOn); err != nil {
			return nil, fmt.Errorf("scan qs topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// Create inserts a new QS project.
func (r *ProjectRepo) Create(ctx context.Context, p *Project) (int64, error) {
	q := `INSERT INTO project (name, external_survey_id, salesforce_job_number, client_id,
	                           sample_size, created_on, created_by, interview_length,
	                           project_status_id, post_screenin_buffer, moderator_buffer)
	      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, q,
		p.Name, p.ExternalSurveyID, p.SalesforceJobNumber, p.ClientID,
		p.SampleSize, time.Now().UTC(), p.CreatedBy, p.InterviewLength,
		1, // project_status_id = 1 (Defining)
		p.PostScreeninBuffer, p.ModeratorBuffer,
	)
	if err != nil {
		return 0, fmt.Errorf("create qs project: %w", err)
	}
	return res.LastInsertId()
}

// CreateProjectFull performs the legacy 9-query transaction for project creation.
// Creates: external_client, project, survey, third_party_survey, question, survey_question,
// participant_group, topics, and returns the project detail via SELECT.
func (r *ProjectRepo) CreateProjectFull(ctx context.Context, req map[string]any) (map[string]any, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	sfJobNumber, _ := req["salesForceJobNumber"].(string)
	sfClientId, _ := req["salesForceClientId"].(string)
	clientID, _ := req["clientId"].(float64)
	name, _ := req["name"].(string)
	externalSurveyID, _ := req["surveyId"].(string)
	sampleSize, _ := req["sampleSize"].(float64)
	interviewLength, _ := req["interviewLength"].(float64)
	userID, _ := req["id"].(float64)
	postScreeninBuffer, _ := req["postScreeninBuffer"].(float64)
	moderatorBuffer, _ := req["moderatorBuffer"].(float64)
	topicName, _ := req["topicName"].(string)
	sfJobNumberText, _ := req["SalesForceJobNumberText"].(string)

	// 1. INSERT external_client
	res1, err := tx.ExecContext(ctx,
		"INSERT INTO external_client (external_client_project_id, external_client_account_id, client_id) VALUES (?, ?, ?)",
		sfJobNumber, sfClientId, int64(clientID))
	if err != nil {
		return nil, fmt.Errorf("insert external_client: %w", err)
	}
	extClientID, _ := res1.LastInsertId()

	// 2. INSERT project
	res2, err := tx.ExecContext(ctx,
		`INSERT INTO project (name, external_survey_id, sample_size, salesforce_job_number, client_id,
		 created_by, interview_length, qual_moderator_id, modified_by, project_status_id,
		 post_screenin_buffer, moderator_buffer, project_external_client)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
		name, externalSurveyID, int64(sampleSize), sfJobNumberText, int64(clientID),
		int64(userID), int64(interviewLength), int64(userID), int64(userID),
		postScreeninBuffer, moderatorBuffer, extClientID)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	projectID, _ := res2.LastInsertId()

	// 3. INSERT survey
	res3, err := tx.ExecContext(ctx,
		"INSERT INTO survey (client_id, created_by, project_id, owned_by) VALUES (?, ?, ?, ?)",
		int64(clientID), int64(userID), projectID, int64(userID))
	if err != nil {
		return nil, fmt.Errorf("insert survey: %w", err)
	}
	surveyID, _ := res3.LastInsertId()

	// 4. INSERT third_party_survey
	_, err = tx.ExecContext(ctx,
		"INSERT INTO third_party_survey (type_id, qs_tool_survey_id) VALUES (1, ?)", surveyID)
	if err != nil {
		return nil, fmt.Errorf("insert third_party_survey: %w", err)
	}

	// 5. INSERT question
	res5, err := tx.ExecContext(ctx,
		"INSERT INTO question (title, comments) VALUES ('Scheduler', 1)")
	if err != nil {
		return nil, fmt.Errorf("insert question: %w", err)
	}
	questionID, _ := res5.LastInsertId()

	// 6. INSERT survey_question
	_, err = tx.ExecContext(ctx,
		"INSERT INTO survey_question (question_id, survey_id) VALUES (?, ?)", questionID, surveyID)
	if err != nil {
		return nil, fmt.Errorf("insert survey_question: %w", err)
	}

	// 7. INSERT participant_group
	_, err = tx.ExecContext(ctx,
		"INSERT INTO participant_group (survey_id, created_by) VALUES (?, ?)", surveyID, int64(userID))
	if err != nil {
		return nil, fmt.Errorf("insert participant_group: %w", err)
	}

	// 8. INSERT topics
	_, err = tx.ExecContext(ctx,
		"INSERT INTO topics (topic_name, created_by, project_id, language_id) VALUES (?, ?, ?, 1)",
		topicName, int64(userID), projectID)
	if err != nil {
		return nil, fmt.Errorf("insert topics: %w", err)
	}

	// 9. SELECT project details (matching legacy query exactly)
	q := `SELECT project.id, participant_group.id as participantGroupId, project.name,
	      project.client_id as clientId, project.created_on as createdOn,
	      project.created_by as createdBy, project.modified_on as modifiedOn,
	      project.qual_moderator_id as qualModeratorId,
	      project.project_status_id as projectStatusId,
	      project.interview_length as interviewLength,
	      project.sample_size as sampleSize,
	      project.salesforce_job_number as salesForceJobNumber,
	      project.external_survey_id as externalSurveyId,
	      project_status.name as projectStatus,
	      survey.id as surveyId,
	      post_screenin_buffer as postScreeninBuffer,
	      moderator_buffer as moderatorBuffer,
	      (SELECT t.topic_name FROM topics AS t WHERE t.project_id=project.id AND t.language_id=1) as topic_name
	      FROM project
	      INNER JOIN project_status ON project.project_status_id = project_status.id
	      INNER JOIN survey ON project.id = survey.project_id
	      INNER JOIN participant_group ON survey.id = participant_group.survey_id
	      WHERE project.id = ?`

	row := tx.QueryRowContext(ctx, q, projectID)
	var result struct {
		ID, ParticipantGroupID, ClientID, CreatedBy, QualModeratorID, ProjectStatusID int64
		InterviewLength, SampleSize                                                   sql.NullInt64
		Name, ProjectStatus                                                           string
		SalesForceJobNumber, ExternalSurveyID, TopicName                              sql.NullString
		PostScreeninBuffer, ModeratorBuffer                                           sql.NullString
		CreatedOn, ModifiedOn                                                         sql.NullString
		SurveyID                                                                      int64
	}
	err = row.Scan(&result.ID, &result.ParticipantGroupID, &result.Name,
		&result.ClientID, &result.CreatedOn, &result.CreatedBy, &result.ModifiedOn,
		&result.QualModeratorID, &result.ProjectStatusID, &result.InterviewLength,
		&result.SampleSize, &result.SalesForceJobNumber, &result.ExternalSurveyID,
		&result.ProjectStatus, &result.SurveyID, &result.PostScreeninBuffer,
		&result.ModeratorBuffer, &result.TopicName)
	if err != nil {
		return nil, fmt.Errorf("select project details: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	record := map[string]any{
		"id":                  result.ID,
		"participantGroupId":  result.ParticipantGroupID,
		"name":                result.Name,
		"clientId":            result.ClientID,
		"createdOn":           result.CreatedOn.String,
		"createdBy":           result.CreatedBy,
		"modifiedOn":          result.ModifiedOn.String,
		"qualModeratorId":     result.QualModeratorID,
		"projectStatusId":     result.ProjectStatusID,
		"interviewLength":     result.InterviewLength.Int64,
		"sampleSize":          result.SampleSize.Int64,
		"salesForceJobNumber": result.SalesForceJobNumber.String,
		"externalSurveyId":    result.ExternalSurveyID.String,
		"projectStatus":       result.ProjectStatus,
		"surveyId":            result.SurveyID,
		"postScreeninBuffer":  result.PostScreeninBuffer.String,
		"moderatorBuffer":     result.ModeratorBuffer.String,
		"topic_name":          result.TopicName.String,
	}
	return record, nil
}

// GetSalesForceJobNumberText looks up job_number_text_c from salesforce_project
// by salesforce_project_id, matching the legacy getSalesForceJobNumberTextBySalesForceId query.
func (r *ProjectRepo) GetSalesForceJobNumberText(ctx context.Context, sfProjectID string) (string, error) {
	if sfProjectID == "" {
		return "", nil
	}
	var jobNumber sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT job_number_text_c FROM salesforce_project WHERE salesforce_project_id = ?",
		sfProjectID).Scan(&jobNumber)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get sf job number text: %w", err)
	}
	return jobNumber.String, nil
}

// Update modifies mutable QS project fields.
func (r *ProjectRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	allowed := map[string]bool{
		"name": true, "external_survey_id": true, "salesforce_job_number": true,
		"client_id": true, "sample_size": true, "interview_length": true,
		"project_status_id": true, "post_screenin_buffer": true, "moderator_buffer": true,
	}

	var setClauses []string
	var args []any
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "modified_on = ?")
	args = append(args, time.Now().UTC())
	args = append(args, id)

	q := fmt.Sprintf("UPDATE project SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update qs project %d: %w", id, err)
	}
	return nil
}

// TimeSlotCounts returns scheduled and completed counts for a given project.
func (r *ProjectRepo) TimeSlotCounts(ctx context.Context, projectID int64) (scheduled, completed int, err error) {
	q := `SELECT
	        COALESCE(SUM(CASE WHEN status_id IN (2,3,4,5) THEN 1 ELSE 0 END), 0),
	        COALESCE(SUM(CASE WHEN status_id = 5 THEN 1 ELSE 0 END), 0)
	      FROM time_slot WHERE project_id = ?`
	err = r.db.QueryRowContext(ctx, q, projectID).Scan(&scheduled, &completed)
	if err != nil {
		err = fmt.Errorf("count qs timeslots for project %d: %w", projectID, err)
	}
	return
}
