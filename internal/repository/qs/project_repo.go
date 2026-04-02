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

// GetProjectDetailsMRA returns project details matching the legacy getProjectDetails query exactly.
// Complex JOIN: project + project_status + survey + participant_group + external_client +
// salesforce_account + time_slot counts + topics (language filtered).
func (r *ProjectRepo) GetProjectDetailsMRA(ctx context.Context, projectID int64) (map[string]any, error) {
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
	      scheduler_generated as schedulerGenerated,
	      ifnull(c.scheduled, 0) as scheduled,
	      ifnull(r.completed, 0) as completed,
	      post_screenin_buffer as postScreeninBuffer,
	      project.moderator_buffer as moderatorBuffer,
	      project.shghash,
	      salesaccount.salesforce_account_id as salesForceAccountId,
	      salesaccount.name as salesForceAccountName,
	      t.topic_name as topicName
	      FROM project
	      INNER JOIN project_status ON project.project_status_id = project_status.id
	      INNER JOIN survey ON project.id = survey.project_id
	      INNER JOIN participant_group ON survey.id = participant_group.survey_id
	      LEFT JOIN external_client exc ON exc.id = project.project_external_client
	      LEFT JOIN salesforce_account salesaccount ON salesaccount.salesforce_account_id = exc.external_client_account_id
	      LEFT JOIN (SELECT project_id as time_slot_project_id, COUNT(*) as scheduled
	                 FROM time_slot WHERE time_slot.status_id=2 AND time_slot.is_invalid=false
	                 AND time_slot.end_time >= UTC_TIMESTAMP()
	                 GROUP BY project_id ORDER BY scheduled DESC) c ON c.time_slot_project_id = project.id
	      LEFT JOIN (SELECT project_id as time_slot_project_id, COUNT(*) as completed
	                 FROM time_slot WHERE time_slot.status_id=9 AND time_slot.is_invalid=false
	                 AND time_slot.is_invalidated_interview = 0
	                 GROUP BY project_id ORDER BY completed DESC) r ON r.time_slot_project_id = project.id
	      LEFT JOIN (SELECT topic_name, project_id FROM topics t
	                 JOIN language_localisation l ON t.language_id = l.id
	                 WHERE langCode_countryCode = 'en_us') t ON t.project_id = project.id
	      WHERE project.id = ?`

	var (
		id, participantGroupID                                                int64
		clientID, createdBy, qualModeratorID, projectStatusID, surveyID      sql.NullInt64
		interviewLength, sampleSize, scheduled, completed                    sql.NullInt64
		name, projectStatus                                                  string
		salesForceJobNumber, externalSurveyID, postScreeninBuffer            sql.NullString
		moderatorBuffer, shghash, salesForceAccountId, salesForceAccountName sql.NullString
		topicName                                                            sql.NullString
		createdOn, modifiedOn                                                sql.NullString
		schedulerGenerated                                                   sql.NullBool
	)

	err := r.db.QueryRowContext(ctx, q, projectID).Scan(
		&id, &participantGroupID, &name, &clientID, &createdOn, &createdBy,
		&modifiedOn, &qualModeratorID, &projectStatusID, &interviewLength,
		&sampleSize, &salesForceJobNumber, &externalSurveyID, &projectStatus,
		&surveyID, &schedulerGenerated, &scheduled, &completed,
		&postScreeninBuffer, &moderatorBuffer, &shghash,
		&salesForceAccountId, &salesForceAccountName, &topicName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project details mra %d: %w", projectID, err)
	}

	record := map[string]any{
		"id":                    id,
		"participantGroupId":    participantGroupID,
		"name":                  name,
		"clientId":              clientID.Int64,
		"createdOn":             createdOn.String,
		"createdBy":             createdBy.Int64,
		"modifiedOn":            modifiedOn.String,
		"qualModeratorId":       qualModeratorID.Int64,
		"projectStatusId":       projectStatusID.Int64,
		"interviewLength":       interviewLength.Int64,
		"sampleSize":            sampleSize.Int64,
		"salesForceJobNumber":   salesForceJobNumber.String,
		"externalSurveyId":      externalSurveyID.String,
		"projectStatus":         projectStatus,
		"surveyId":              surveyID.Int64,
		"schedulerGenerated":    schedulerGenerated.Bool,
		"scheduled":             scheduled.Int64,
		"completed":             completed.Int64,
		"postScreeninBuffer":    postScreeninBuffer.String,
		"moderatorBuffer":       moderatorBuffer.String,
		"shghash":               shghash.String,
		"salesForceAccountId":   salesForceAccountId.String,
		"salesForceAccountName": salesForceAccountName.String,
		"topicName":             topicName.String,
	}
	return record, nil
}

// GetProjectsMRA returns projects matching the legacy getProjects query.
// Includes: project_status, time_slot counts, user (creator), external_client, topics.
// Filters by externalClientsIds, creatorId, status, search, sort.
func (r *ProjectRepo) GetProjectsMRA(ctx context.Context, creatorID, status int, sort, search string, externalClientIDs []string) ([]map[string]any, error) {
	var q string
	args := []any{}

	baseSelect := `SELECT project.id, project.name, project_status.name AS project_status_name,
	      salesforce_job_number, project.client_id, sample_size, created_on,
	      project.created_by, interview_length, qual_moderator_id,
	      project.modified_on, modified_by, project_status_id,
	      IFNULL(c.scheduled, 0) AS scheduled,
	      IFNULL(r.completed, 0) AS completed,
	      user.first_name AS firstName, user.last_name AS lastName,
	      project.project_external_client,
	      topics.topic_name as topicName`

	if search != "" {
		baseSelect = `SELECT project.id, project.name, project_status.name AS project_status_name,
		      external_survey_id, salesforce_job_number, project.client_id, sample_size, created_on,
		      project.created_by, interview_length, qual_moderator_id,
		      project.modified_on, modified_by, project_status_id,
		      IFNULL(c.scheduled, 0) AS scheduled,
		      IFNULL(r.completed, 0) AS completed,
		      user.first_name AS firstName, user.last_name AS lastName,
		      project.project_external_client,
		      topics.topic_name as topicName`
	}

	baseJoins := ` FROM project
	      LEFT JOIN topics ON project.id = topics.project_id AND topics.language_id = 1
	      INNER JOIN project_status ON project.project_status_id = project_status.id
	      LEFT JOIN (SELECT project_id AS time_slot_project_id, COUNT(*) AS scheduled
	                 FROM time_slot WHERE time_slot.status_id = 2 AND time_slot.is_invalid = FALSE
	                 AND time_slot.end_time >= UTC_TIMESTAMP()
	                 GROUP BY project_id ORDER BY scheduled DESC) c ON c.time_slot_project_id = project.id
	      LEFT JOIN (SELECT project_id AS time_slot_project_id, COUNT(*) AS completed
	                 FROM time_slot WHERE time_slot.status_id = 9 AND time_slot.is_invalid = FALSE
	                 AND time_slot.is_invalidated_interview = 0
	                 GROUP BY project_id ORDER BY completed DESC) r ON r.time_slot_project_id = project.id
	      INNER JOIN user ON project.created_by = user.id
	      INNER JOIN external_client ON project.project_external_client = external_client.id`

	// Build IN clause for externalClientIDs
	placeholders := make([]string, len(externalClientIDs))
	for i, id := range externalClientIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	inClause := "(" + strings.Join(placeholders, ",") + ")"

	if search != "" {
		q = baseSelect + baseJoins +
			` WHERE (? = -1 OR project.created_by = ?) AND (? = -1 OR project.project_status_id = ?)` +
			` AND ((project.name LIKE ? OR project.external_survey_id LIKE ? OR project.salesforce_job_number LIKE ?))` +
			` AND external_client.external_client_account_id IN ` + inClause +
			` AND project.client_id = 1 ORDER BY modified_on DESC`
		searchPattern := "%" + search + "%"
		args = append([]any{creatorID, creatorID, status, status, searchPattern, searchPattern, searchPattern}, args...)
	} else {
		q = baseSelect + baseJoins +
			` WHERE project.client_id = 1` +
			` AND external_client.external_client_account_id IN ` + inClause +
			` AND (? = -1 OR project.created_by = ?)` +
			` AND (? = -1 OR project.project_status_id = ?)` +
			` ORDER BY ` + sort + ` DESC`
		args = append(args, creatorID, creatorID, status, status)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get projects mra: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var (
			id, clientID, createdBy, qualModeratorID, modifiedBy, projectStatusID int64
			sampleSize, interviewLength, scheduled, completed                     sql.NullInt64
			projectExternalClient                                                 sql.NullInt64
			name, projectStatusName                                               string
			sfJobNumber, firstName, lastName, topicName                            sql.NullString
			createdOn, modifiedOn                                                 sql.NullString
		)

		if search != "" {
			var externalSurveyID sql.NullString
			if err := rows.Scan(&id, &name, &projectStatusName, &externalSurveyID,
				&sfJobNumber, &clientID, &sampleSize, &createdOn,
				&createdBy, &interviewLength, &qualModeratorID,
				&modifiedOn, &modifiedBy, &projectStatusID,
				&scheduled, &completed, &firstName, &lastName,
				&projectExternalClient, &topicName); err != nil {
				return nil, fmt.Errorf("scan project row: %w", err)
			}
			results = append(results, map[string]any{
				"id": id, "name": name, "project_status_name": projectStatusName,
				"external_survey_id": externalSurveyID.String,
				"salesforce_job_number": sfJobNumber.String, "client_id": clientID,
				"sample_size": sampleSize.Int64, "created_on": createdOn.String,
				"created_by": createdBy, "interview_length": interviewLength.Int64,
				"qual_moderator_id": qualModeratorID, "modified_on": modifiedOn.String,
				"modified_by": modifiedBy, "project_status_id": projectStatusID,
				"scheduled": scheduled.Int64, "completed": completed.Int64,
				"firstName": firstName.String, "lastName": lastName.String,
				"project_external_client": projectExternalClient.Int64,
				"topicName": topicName.String,
			})
		} else {
			if err := rows.Scan(&id, &name, &projectStatusName,
				&sfJobNumber, &clientID, &sampleSize, &createdOn,
				&createdBy, &interviewLength, &qualModeratorID,
				&modifiedOn, &modifiedBy, &projectStatusID,
				&scheduled, &completed, &firstName, &lastName,
				&projectExternalClient, &topicName); err != nil {
				return nil, fmt.Errorf("scan project row: %w", err)
			}
			results = append(results, map[string]any{
				"id": id, "name": name, "project_status_name": projectStatusName,
				"salesforce_job_number": sfJobNumber.String, "client_id": clientID,
				"sample_size": sampleSize.Int64, "created_on": createdOn.String,
				"created_by": createdBy, "interview_length": interviewLength.Int64,
				"qual_moderator_id": qualModeratorID, "modified_on": modifiedOn.String,
				"modified_by": modifiedBy, "project_status_id": projectStatusID,
				"scheduled": scheduled.Int64, "completed": completed.Int64,
				"firstName": firstName.String, "lastName": lastName.String,
				"project_external_client": projectExternalClient.Int64,
				"topicName": topicName.String,
			})
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results, rows.Err()
}

// GetProjectsForModsMRA returns projects matching the legacy getProjectsForMods query.
// Simpler variant for when request body is empty.
func (r *ProjectRepo) GetProjectsForModsMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	q := `SELECT project.id, project.name, project_status.name as project_status_name,
	      external_survey_id, salesforce_job_number, client_id, sample_size, created_on,
	      created_by, interview_length, qual_moderator_id, project.modified_on,
	      modified_by, project_status_id,
	      ifnull(c.scheduled, 0) as scheduled,
	      ifnull(r.completed, 0) as completed,
	      user.first_name as firstName, user.last_name as lastName
	      FROM project
	      INNER JOIN project_status ON project.project_status_id = project_status.id
	      LEFT JOIN (SELECT project_id as time_slot_project_id, COUNT(*) as scheduled
	                 FROM time_slot WHERE time_slot.status_id=2 AND time_slot.is_invalid=false
	                 AND time_slot.end_time >= UTC_TIMESTAMP()
	                 GROUP BY project_id ORDER BY scheduled DESC) c ON c.time_slot_project_id = project.id
	      LEFT JOIN (SELECT project_id as time_slot_project_id, COUNT(*) as completed
	                 FROM time_slot WHERE time_slot.status_id=9 AND time_slot.is_invalid=false
	                 AND time_slot.is_invalidated_interview = 0
	                 GROUP BY project_id ORDER BY completed DESC) r ON r.time_slot_project_id = project.id
	      INNER JOIN user ON project.created_by = user.id
	      WHERE client_id = ?`

	rows, err := r.db.QueryContext(ctx, q, clientID)
	if err != nil {
		return nil, fmt.Errorf("get projects for mods mra: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var (
			id, cID, createdBy, qualModeratorID, modifiedBy, projectStatusID int64
			sampleSize, interviewLength, scheduled, completed                sql.NullInt64
			name, projectStatusName                                          string
			externalSurveyID, sfJobNumber, firstName, lastName               sql.NullString
			createdOn, modifiedOn                                            sql.NullString
		)
		if err := rows.Scan(&id, &name, &projectStatusName, &externalSurveyID,
			&sfJobNumber, &cID, &sampleSize, &createdOn,
			&createdBy, &interviewLength, &qualModeratorID, &modifiedOn,
			&modifiedBy, &projectStatusID, &scheduled, &completed,
			&firstName, &lastName); err != nil {
			return nil, fmt.Errorf("scan project for mods row: %w", err)
		}
		results = append(results, map[string]any{
			"id": id, "name": name, "project_status_name": projectStatusName,
			"external_survey_id": externalSurveyID.String,
			"salesforce_job_number": sfJobNumber.String, "client_id": cID,
			"sample_size": sampleSize.Int64, "created_on": createdOn.String,
			"created_by": createdBy, "interview_length": interviewLength.Int64,
			"qual_moderator_id": qualModeratorID, "modified_on": modifiedOn.String,
			"modified_by": modifiedBy, "project_status_id": projectStatusID,
			"scheduled": scheduled.Int64, "completed": completed.Int64,
			"firstName": firstName.String, "lastName": lastName.String,
		})
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results, rows.Err()
}

// SaveUserSelection saves user account/client selections matching legacy side effect.
// Deletes existing selections and inserts new ones in a transaction.
func (r *ProjectRepo) SaveUserSelection(ctx context.Context, userID int64, accountIDs, clientIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete existing account selections
	_, err = tx.ExecContext(ctx, "DELETE FROM user_account_selection WHERE userid = ?", userID)
	if err != nil {
		return fmt.Errorf("delete account selections: %w", err)
	}

	// Insert account selections
	for _, accID := range accountIDs {
		accID = strings.TrimSpace(accID)
		if accID == "" || accID == "''" {
			continue
		}
		accID = strings.Trim(accID, "'")
		_, err = tx.ExecContext(ctx, "INSERT INTO user_account_selection (account_selection_account_id, userid) VALUES (?, ?)", accID, userID)
		if err != nil {
			return fmt.Errorf("insert account selection: %w", err)
		}
	}

	// Delete existing client selections
	_, err = tx.ExecContext(ctx, "DELETE FROM user_client_selection WHERE userid = ?", userID)
	if err != nil {
		return fmt.Errorf("delete client selections: %w", err)
	}

	// Insert client selections
	for _, clID := range clientIDs {
		clID = strings.TrimSpace(clID)
		if clID == "" || clID == "''" {
			continue
		}
		clID = strings.Trim(clID, "'")
		_, err = tx.ExecContext(ctx, "INSERT INTO user_client_selection (client_selection_account_id, userid) VALUES (?, ?)", clID, userID)
		if err != nil {
			return fmt.Errorf("insert client selection: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// UpdateSchedulerGenerated sets scheduler_generated=1 for a project (legacy default path).
func (r *ProjectRepo) UpdateSchedulerGenerated(ctx context.Context, projectID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE project SET scheduler_generated = 1 WHERE id = ?", projectID)
	if err != nil {
		return fmt.Errorf("update scheduler_generated: %w", err)
	}
	return nil
}

// UpdatePostScreenInBuffer updates post_screenin_buffer and modified_on for a project.
func (r *ProjectRepo) UpdatePostScreenInBuffer(ctx context.Context, projectID int64, buffer float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE project SET post_screenin_buffer = ?, modified_on = ? WHERE id = ?",
		buffer, time.Now().UTC(), projectID)
	if err != nil {
		return fmt.Errorf("update post_screenin_buffer: %w", err)
	}
	return nil
}

// UpdateModeratorBufferMRA updates moderator_buffer and modified_on for a project.
func (r *ProjectRepo) UpdateModeratorBufferMRA(ctx context.Context, projectID int64, buffer float64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE project SET moderator_buffer = ?, modified_on = ? WHERE id = ?",
		buffer, time.Now().UTC(), projectID)
	if err != nil {
		return fmt.Errorf("update moderator_buffer: %w", err)
	}
	return nil
}

// UpdateExternalSurveyID updates external_survey_id and modified_on for a project.
func (r *ProjectRepo) UpdateExternalSurveyID(ctx context.Context, projectID int64, surveyID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE project SET external_survey_id = ?, modified_on = ? WHERE id = ?",
		surveyID, time.Now().UTC(), projectID)
	if err != nil {
		return fmt.Errorf("update external_survey_id: %w", err)
	}
	return nil
}

// GetProjectModeratorIDs returns user_ids of moderators assigned to a project.
func (r *ProjectRepo) GetProjectModeratorIDs(ctx context.Context, projectID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT user_id FROM projects_users WHERE project_id = ?", projectID)
	if err != nil {
		return nil, fmt.Errorf("get project mod ids: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ResetProjectModeratorsMRA performs the legacy diff-based moderator reset.
// Compares newIDs vs existingIDs, then adds/removes in projects_users and cleans moderator_time_range.
func (r *ProjectRepo) ResetProjectModeratorsMRA(ctx context.Context, projectID int64, newIDs, existingIDs []int64) error {
	existingSet := map[int64]bool{}
	for _, id := range existingIDs {
		existingSet[id] = true
	}
	newSet := map[int64]bool{}
	for _, id := range newIDs {
		newSet[id] = true
	}

	var modsToAdd, modsToRemove []int64
	for _, id := range existingIDs {
		if !newSet[id] {
			modsToRemove = append(modsToRemove, id)
		}
	}
	for _, id := range newIDs {
		if !existingSet[id] {
			modsToAdd = append(modsToAdd, id)
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if len(newIDs) > 0 {
		// Remove mods no longer in list
		if len(modsToRemove) > 0 {
			placeholders := make([]string, len(modsToRemove))
			args := []any{projectID}
			for i, id := range modsToRemove {
				placeholders[i] = "?"
				args = append(args, id)
			}
			_, err = tx.ExecContext(ctx,
				"DELETE FROM projects_users WHERE project_id = ? AND user_id IN ("+strings.Join(placeholders, ",")+")", args...)
			if err != nil {
				return fmt.Errorf("delete removed mods: %w", err)
			}
		}
		// Add new mods
		for _, id := range modsToAdd {
			_, err = tx.ExecContext(ctx,
				"INSERT INTO projects_users (project_id, user_id) VALUES (?, ?)", projectID, id)
			if err != nil {
				return fmt.Errorf("insert mod: %w", err)
			}
		}
		// Clean stale moderator_time_range
		if len(modsToRemove) > 0 {
			_, err = tx.ExecContext(ctx,
				"DELETE FROM moderator_time_range WHERE moderator_id NOT IN (SELECT user_id FROM projects_users WHERE project_id = ?) AND project_id = ?",
				projectID, projectID)
			if err != nil {
				return fmt.Errorf("clean moderator_time_range: %w", err)
			}
		}
	} else {
		// Empty moderatorIds → delete all
		_, err = tx.ExecContext(ctx,
			"DELETE FROM projects_users WHERE project_id = ?", projectID)
		if err != nil {
			return fmt.Errorf("delete all mods: %w", err)
		}
		_, err = tx.ExecContext(ctx,
			"DELETE FROM moderator_time_range WHERE project_id = ?", projectID)
		if err != nil {
			return fmt.Errorf("delete all time ranges: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// UnassignModeratorFromProject removes a moderator from project and their time range.
func (r *ProjectRepo) UnassignModeratorFromProject(ctx context.Context, userID, projectID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx,
		"DELETE FROM projects_users WHERE user_id = ? AND project_id = ?", userID, projectID)
	if err != nil {
		return fmt.Errorf("delete projects_users: %w", err)
	}
	_, err = tx.ExecContext(ctx,
		"DELETE FROM moderator_time_range WHERE project_id = ? AND moderator_id = ?", projectID, userID)
	if err != nil {
		return fmt.Errorf("delete moderator_time_range: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// GetModeratorsList returns moderators for a project matching legacy getModeratorsList query.
func (r *ProjectRepo) GetModeratorsList(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT user.first_name as firstName, user.last_name as lastName,
	      user.id as id, user_client.client_id as clientId,
	      COUNT(DISTINCT moderator_time_slot.id) as interviewCount,
	      mtr.start_time as startTime, mtr.end_time as endTime, mtr.timezone as timezone
	      FROM user
	      INNER JOIN user_client ON user.id = user_client.user_id
	      INNER JOIN projects_users ON user_client.user_id = projects_users.user_id
	      INNER JOIN user_role ON user.id = user_role.user_id
	      LEFT OUTER JOIN (SELECT moderator_time_slot.* FROM moderator_time_slot
	           INNER JOIN time_slot ON time_slot.id = moderator_time_slot.time_slot_id
	           AND time_slot.status_id = 2 AND time_slot.is_invalid = FALSE
	           AND time_slot.project_id = ?) moderator_time_slot ON moderator_time_slot.moderator_id = user.id
	      LEFT OUTER JOIN (SELECT * FROM moderator_time_range WHERE project_id = ?) mtr ON mtr.moderator_id = user.id
	      WHERE user_role.role_id = 1 AND user.deleted = 0 AND projects_users.project_id = ?
	      GROUP BY user.id`

	rows, err := r.db.QueryContext(ctx, q, projectID, projectID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get moderators list: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var (
			firstName, lastName                string
			id, clientID, interviewCount       int64
			startTime, endTime, timezone       sql.NullString
		)
		if err := rows.Scan(&firstName, &lastName, &id, &clientID,
			&interviewCount, &startTime, &endTime, &timezone); err != nil {
			return nil, fmt.Errorf("scan moderator row: %w", err)
		}
		results = append(results, map[string]any{
			"firstName":      firstName,
			"lastName":       lastName,
			"id":             id,
			"clientId":       clientID,
			"interviewCount": interviewCount,
			"startTime":      startTime.String,
			"endTime":        endTime.String,
			"timezone":       timezone.String,
		})
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results, rows.Err()
}

// GetEmailTemplateMRA returns body_content from communication_template by type_id and language.
func (r *ProjectRepo) GetEmailTemplateMRA(ctx context.Context, typeID int, language string) (map[string]any, error) {
	if language == "" {
		language = "en_us"
	}
	var bodyContent sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT body_content FROM communication_template WHERE communication_type_id = ? AND language_code = ?",
		typeID, language).Scan(&bodyContent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get email template: %w", err)
	}
	return map[string]any{"body_content": bodyContent.String}, nil
}

// HandleProjectExportMRA runs the legacy export query returning timeslot rows with 19 columns.
func (r *ProjectRepo) HandleProjectExportMRA(ctx context.Context, projectID int64, pmTimeZone, pmTimeZoneAbbr, rescheduleLinkPrefix string) ([][]string, error) {
	q := fmt.Sprintf(`SELECT
		responder.external_responder_id AS participantId,
		time_slot.duration,
		CONCAT(CONVERT_TZ(time_slot.start_time, 'UTC', '%s'),' ', '%s') AS startTime,
		responder.first_name AS ResonderFirstName,
		responder.last_name AS ResonderLastName,
		responder.sess_key AS sessKey,
		CASE WHEN responder.time_zone IS NULL THEN CONCAT(CONVERT_TZ(time_slot.start_time, 'UTC','America/New_York'), ' ')
		     WHEN responder.time_zone IS NOT NULL THEN CONCAT(CONVERT_TZ(time_slot.start_time, 'UTC', responder.time_zone), ' ')
		END AS respondentInterviewTime,
		project_responder_comm_address.honorarium AS Honorarium,
		CONCAT(CONVERT_TZ(time_slot.updated_on, 'UTC', '%s'),' ', '%s') AS ModifiedOn,
		(SELECT address FROM responder_communication_address WHERE transport_type_id = 1
		 AND responder_communication_address.id = project_responder_comm_address.responder_comm_address_id) AS Email,
		(SELECT GROUP_CONCAT(DISTINCT rca1.address SEPARATOR ', ') FROM responder_communication_address rca1
		 WHERE rca1.transport_type_id=4 AND rca1.responder_id = project_responder_comm_address.responder_id) AS Phone,
		conference_invitation.conference_link AS conferenceLink,
		moderator_info.first_name AS ModeratorFirstName,
		moderator_info.last_name AS ModeratorLastName,
		project_manager_info.first_name AS PMFirstName,
		project_manager_info.last_name AS PMLastName,
		CASE
		    WHEN time_slot.status_id = 9 AND time_slot.is_invalidated_interview = 0 THEN 'Complete'
		    WHEN time_slot.status_id = 9 AND time_slot.is_invalidated_interview = 1 THEN 'Invalidated'
		    WHEN time_slot.status_id = 5 THEN 'Cancelled'
		    WHEN time_slot.status_id = 6 THEN 'Participant Cancelled'
		    WHEN time_slot.status_id = 3 THEN 'Moderator Rescheduled'
		    WHEN time_slot.status_id = 4 THEN 'Participant Rescheduled'
		    WHEN time_slot.status_id = 11 THEN 'Project Manager Rescheduled'
		    WHEN time_slot.status_id = 12 THEN 'Project Manager Cancelled'
		    ELSE 'Scheduled'
		END AS Status,
		'' AS Comment,
		responder.time_zone AS responderTimezone,
		'' AS rescheduleLink
	FROM responder
	INNER JOIN answer_details ON answer_details.responder_id = responder.id
	INNER JOIN time_slot ON time_slot.id = answer_details.time_slot_id
	INNER JOIN conference_invitation_responder_time_slot ON conference_invitation_responder_time_slot.time_slot_id = time_slot.id
	INNER JOIN conference_invitation ON conference_invitation.id = conference_invitation_responder_time_slot.conference_invitation_id
	INNER JOIN (SELECT user.id AS id, user.first_name, user.last_name, moderator_time_slot.time_slot_id
	            FROM moderator_time_slot INNER JOIN user ON user.id = moderator_time_slot.moderator_id
	            WHERE moderator_time_slot.is_host) moderator_info ON moderator_info.time_slot_id = time_slot.id
	INNER JOIN project ON time_slot.project_id = project.id
	INNER JOIN (SELECT user.id AS id, user.first_name, user.last_name, user.time_zone FROM user) project_manager_info
	      ON project_manager_info.id = project.created_by
	INNER JOIN client ON client.id = project.client_id
	INNER JOIN responder_communication_address ON answer_details.responder_id = responder_communication_address.responder_id
	INNER JOIN project_responder_comm_address ON project_responder_comm_address.responder_comm_address_id = responder_communication_address.id
	WHERE time_slot.project_id = ? AND project_responder_comm_address.project_id = ?
	ORDER BY time_slot.start_time ASC`, pmTimeZone, pmTimeZoneAbbr, pmTimeZone, pmTimeZoneAbbr)

	rows, err := r.db.QueryContext(ctx, q, projectID, projectID)
	if err != nil {
		return nil, fmt.Errorf("handle project export: %w", err)
	}
	defer rows.Close()

	var results [][]string
	for rows.Next() {
		var (
			participantId, duration, startTime, firstName, lastName, sessKey         sql.NullString
			respondentTime, honorarium, modifiedOn, email, phone, confLink           sql.NullString
			modFirstName, modLastName, pmFirstName, pmLastName, status, comment      sql.NullString
			respTimezone, rescheduleLink                                             sql.NullString
		)
		if err := rows.Scan(&participantId, &duration, &startTime, &firstName, &lastName, &sessKey,
			&respondentTime, &honorarium, &modifiedOn, &email, &phone, &confLink,
			&modFirstName, &modLastName, &pmFirstName, &pmLastName, &status, &comment,
			&respTimezone, &rescheduleLink); err != nil {
			return nil, fmt.Errorf("scan export row: %w", err)
		}
		results = append(results, []string{
			participantId.String, duration.String, startTime.String, firstName.String, lastName.String,
			sessKey.String, respondentTime.String, honorarium.String, modifiedOn.String,
			email.String, phone.String, confLink.String, modFirstName.String, modLastName.String,
			pmFirstName.String, pmLastName.String, status.String, comment.String, rescheduleLink.String,
		})
	}
	return results, rows.Err()
}

// HandleProjectNoTimeslotExportMRA returns respondents without timeslots for export.
func (r *ProjectRepo) HandleProjectNoTimeslotExportMRA(ctx context.Context, projectID int64, pmTimeZone, pmTimeZoneAbbr string) ([][]string, error) {
	q := fmt.Sprintf(`SELECT responder.external_responder_id AS participantId,
		'' AS duration, '' AS startTime,
		responder.first_name AS ResonderFirstName, responder.last_name AS ResonderLastName,
		responder.sess_key AS sessKey, '' AS respondentInterviewTime,
		project_responder_comm_address.honorarium AS Honorarium,
		CONCAT(CONVERT_TZ(answer_comment.modified_on, 'UTC', '%s'),' ','%s') AS ModifiedOn,
		(SELECT address FROM responder_communication_address WHERE transport_type_id = 1
		 AND responder_communication_address.id = project_responder_comm_address.responder_comm_address_id) AS Email,
		(SELECT GROUP_CONCAT(DISTINCT rca1.address SEPARATOR ', ') FROM responder_communication_address rca1
		 WHERE rca1.transport_type_id=4 AND rca1.responder_id = project_responder_comm_address.responder_id) AS Phone,
		'' AS conferenceLink, '' AS ModeratorFirstName, '' AS ModeratorLastName,
		project_manager_info.first_name AS PMFirstName, project_manager_info.last_name AS PMLastName,
		'' AS Status, answer_comment.comment AS Comment, '' AS rescheduleLink
	FROM responder
	INNER JOIN answer_details ON answer_details.responder_id = responder.id
	INNER JOIN answer_comment ON answer_comment.answer_id = answer_details.answer_id
	INNER JOIN answer ON answer.id = answer_details.answer_id
	INNER JOIN survey_question ON survey_question.question_id = answer.question_id
	INNER JOIN survey ON survey.id = survey_question.survey_id
	INNER JOIN project ON survey.project_id = project.id
	INNER JOIN (SELECT user.id AS id, user.first_name, user.last_name, user.time_zone FROM user) project_manager_info
	      ON project_manager_info.id = project.created_by
	INNER JOIN responder_communication_address ON answer_details.responder_id = responder_communication_address.responder_id
	INNER JOIN project_responder_comm_address ON project_responder_comm_address.responder_comm_address_id = responder_communication_address.id
	WHERE project.id = ? AND answer_details.no_timeslot_selected = 1 AND project_responder_comm_address.project_id = ?`,
		pmTimeZone, pmTimeZoneAbbr)

	rows, err := r.db.QueryContext(ctx, q, projectID, projectID)
	if err != nil {
		return nil, fmt.Errorf("handle no-timeslot export: %w", err)
	}
	defer rows.Close()

	var results [][]string
	for rows.Next() {
		var (
			participantId, duration, startTime, firstName, lastName, sessKey         sql.NullString
			respondentTime, honorarium, modifiedOn, email, phone, confLink           sql.NullString
			modFirstName, modLastName, pmFirstName, pmLastName, status, comment      sql.NullString
			rescheduleLink                                                           sql.NullString
		)
		if err := rows.Scan(&participantId, &duration, &startTime, &firstName, &lastName, &sessKey,
			&respondentTime, &honorarium, &modifiedOn, &email, &phone, &confLink,
			&modFirstName, &modLastName, &pmFirstName, &pmLastName, &status, &comment,
			&rescheduleLink); err != nil {
			return nil, fmt.Errorf("scan no-timeslot row: %w", err)
		}
		results = append(results, []string{
			participantId.String, duration.String, startTime.String, firstName.String, lastName.String,
			sessKey.String, respondentTime.String, honorarium.String, modifiedOn.String,
			email.String, phone.String, confLink.String, modFirstName.String, modLastName.String,
			pmFirstName.String, pmLastName.String, status.String, comment.String, rescheduleLink.String,
		})
	}
	return results, rows.Err()
}

// GetProjectName returns the name of a project by ID.
func (r *ProjectRepo) GetProjectName(ctx context.Context, projectID int64) (string, error) {
	var name string
	err := r.db.QueryRowContext(ctx, "SELECT name FROM project WHERE id = ?", projectID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get project name: %w", err)
	}
	return name, nil
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

// UpdateSampleSizeMRA updates sample_size + modified_on for a project (no status change).
func (r *ProjectRepo) UpdateSampleSizeMRA(ctx context.Context, projectID int64, sampleSize int64) error {
	q := `UPDATE project SET sample_size = ?, modified_on = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, q, sampleSize, projectID)
	if err != nil {
		return fmt.Errorf("update sample size: %w", err)
	}
	return nil
}

// UpdateSampleSizeProjectStatusMRA updates sample_size + project_status_id + modified_on.
func (r *ProjectRepo) UpdateSampleSizeProjectStatusMRA(ctx context.Context, projectID int64, sampleSize int64, projectStatusID int64) error {
	q := `UPDATE project SET sample_size = ?, project_status_id = ?, modified_on = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, q, sampleSize, projectStatusID, projectID)
	if err != nil {
		return fmt.Errorf("update sample size with status: %w", err)
	}
	return nil
}

// ModeratorTimeRange represents a row from moderator_time_range.
type ModeratorTimeRange struct {
	ModeratorID int64
	StartTime   string
	EndTime     string
	Timezone    string
}

// GetModeratorsTimeRangePerProject returns moderator_time_range rows for a project.
func (r *ProjectRepo) GetModeratorsTimeRangePerProject(ctx context.Context, projectID int64) ([]ModeratorTimeRange, error) {
	q := `SELECT moderator_id, start_time, end_time, timezone FROM moderator_time_range WHERE project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get moderators time range: %w", err)
	}
	defer rows.Close()

	var results []ModeratorTimeRange
	for rows.Next() {
		var mtr ModeratorTimeRange
		if err := rows.Scan(&mtr.ModeratorID, &mtr.StartTime, &mtr.EndTime, &mtr.Timezone); err != nil {
			return nil, fmt.Errorf("scan moderator time range: %w", err)
		}
		results = append(results, mtr)
	}
	return results, rows.Err()
}

// GetAllModeratorsAvailabilityPerClient returns moderator availability for all moderators
// assigned to a project, filtered by client.
func (r *ProjectRepo) GetAllModeratorsAvailabilityPerClient(ctx context.Context, clientID int64, projectID int64) ([]ModeratorAvailability, error) {
	q := `SELECT moderator_availability.id, moderator_id, client_id, start_time, end_time
	      FROM moderator_availability
	      INNER JOIN projects_users ON moderator_availability.moderator_id = projects_users.user_id
	      WHERE client_id = ? AND project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, clientID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get moderators availability: %w", err)
	}
	defer rows.Close()

	var results []ModeratorAvailability
	for rows.Next() {
		var ma ModeratorAvailability
		if err := rows.Scan(&ma.ID, &ma.ModeratorID, &ma.ClientID, &ma.StartTime, &ma.EndTime); err != nil {
			return nil, fmt.Errorf("scan moderator availability: %w", err)
		}
		results = append(results, ma)
	}
	return results, rows.Err()
}

// GetProjectStatusByID returns the project_status_id for a project.
func (r *ProjectRepo) GetProjectStatusByID(ctx context.Context, projectID int64) (int64, error) {
	var statusID int64
	err := r.db.QueryRowContext(ctx, `SELECT project_status_id FROM project WHERE id = ?`, projectID).Scan(&statusID)
	if err != nil {
		return 0, fmt.Errorf("get project status: %w", err)
	}
	return statusID, nil
}

// UpsertModeratorTimeRangePerProject inserts or updates a moderator_time_range record.
func (r *ProjectRepo) UpsertModeratorTimeRangePerProject(ctx context.Context, projectID, moderatorID int64, startTime, endTime, timezone string) error {
	q := `INSERT INTO moderator_time_range (moderator_id, project_id, start_time, end_time, timezone)
	      VALUES (?, ?, ?, ?, ?)
	      ON DUPLICATE KEY UPDATE start_time = VALUES(start_time), end_time = VALUES(end_time), timezone = VALUES(timezone)`
	_, err := r.db.ExecContext(ctx, q, moderatorID, projectID, startTime, endTime, timezone)
	if err != nil {
		return fmt.Errorf("upsert moderator time range: %w", err)
	}
	return nil
}

// GetAllModeratorsAvailabilityPerRole returns moderator availability for moderators with role_id=1
// assigned to a project, with end_time >= NOW().
func (r *ProjectRepo) GetAllModeratorsAvailabilityPerRole(ctx context.Context, projectID int64) ([]ModeratorAvailability, error) {
	q := `SELECT moderator_availability.id, moderator_id, client_id, start_time, end_time
	      FROM moderator_availability
	      INNER JOIN user_role ON user_role.user_id = moderator_availability.moderator_id
	      INNER JOIN projects_users ON moderator_availability.moderator_id = projects_users.user_id
	      WHERE user_role.role_id = 1 AND end_time >= NOW() AND project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get moderators availability per role: %w", err)
	}
	defer rows.Close()

	var results []ModeratorAvailability
	for rows.Next() {
		var ma ModeratorAvailability
		if err := rows.Scan(&ma.ID, &ma.ModeratorID, &ma.ClientID, &ma.StartTime, &ma.EndTime); err != nil {
			return nil, fmt.Errorf("scan moderator availability per role: %w", err)
		}
		results = append(results, ma)
	}
	return results, rows.Err()
}

// GetModeratorsTimeRangePerProjectMRA returns moderator time ranges for a project.
// Contract-identical with legacy getModeratorsTimeRangePerProject.
func (r *ProjectRepo) GetModeratorsTimeRangePerProjectMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
	q := `SELECT moderator_id, project_id, start_time, end_time, timezone
	      FROM moderator_time_range WHERE project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("get moderators time range per project mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var moderatorID, pID int64
		var startTime, endTime, tz sql.NullString
		if err := rows.Scan(&moderatorID, &pID, &startTime, &endTime, &tz); err != nil {
			return nil, fmt.Errorf("scan moderator time range: %w", err)
		}
		records = append(records, map[string]any{
			"moderator_id": moderatorID,
			"project_id":   pID,
			"start_time":   startTime.String,
			"end_time":     endTime.String,
			"timezone":     tz.String,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}
