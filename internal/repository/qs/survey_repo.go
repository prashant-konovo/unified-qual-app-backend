package qs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SurveyRow represents a survey stored in the QS database.
type SurveyRow struct {
	ID          int64          `json:"id"`
	ProjectID   sql.NullInt64  `json:"projectId"`
	ProjectName sql.NullString `json:"projectName"`
	Title       string         `json:"title"`
	Status      string         `json:"status"` // draft, published
	Questions   string         `json:"questions"` // JSON
	Rules       string         `json:"rules"`     // JSON
	CreatedOn   time.Time      `json:"createdOn"`
	ModifiedOn  time.Time      `json:"modifiedOn"`
}

// SurveyRepo handles survey CRUD against the QS database.
type SurveyRepo struct {
	db *sql.DB
}

// NewSurveyRepo creates a new survey repository.
func NewSurveyRepo(db *sql.DB) *SurveyRepo {
	return &SurveyRepo{db: db}
}

// EnsureTable creates the survey table if it doesn't exist.
func (r *SurveyRepo) EnsureTable(ctx context.Context) error {
	q := `CREATE TABLE IF NOT EXISTS survey (
		id          BIGINT AUTO_INCREMENT PRIMARY KEY,
		project_id  BIGINT NULL,
		title       VARCHAR(500) NOT NULL DEFAULT '',
		status      VARCHAR(50) NOT NULL DEFAULT 'draft',
		questions   JSON,
		rules       JSON,
		created_on  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		modified_on DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_survey_project (project_id),
		INDEX idx_survey_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := r.db.ExecContext(ctx, q)
	if err != nil {
		return err
	}
	// Also create booking_reward table
	q2 := `CREATE TABLE IF NOT EXISTS booking_reward (
		time_slot_id   BIGINT PRIMARY KEY,
		reward_points  INT NOT NULL DEFAULT 0,
		reward_status  VARCHAR(50) NOT NULL DEFAULT 'not_credited',
		modified_on    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err = r.db.ExecContext(ctx, q2)
	return err
}

// List returns all surveys with optional search.
func (r *SurveyRepo) List(ctx context.Context, search string) ([]SurveyRow, error) {
	q := `SELECT s.id, s.project_id, COALESCE(p.name, '') AS project_name,
	       s.title, s.status, COALESCE(s.questions, '[]'), COALESCE(s.rules, '[]'),
	       s.created_on, s.modified_on
	      FROM survey s
	      LEFT JOIN project p ON p.id = s.project_id`
	var args []any
	if search != "" {
		q += " WHERE s.title LIKE ? OR p.name LIKE ?"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}
	q += " ORDER BY s.modified_on DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list surveys: %w", err)
	}
	defer rows.Close()

	var result []SurveyRow
	for rows.Next() {
		var s SurveyRow
		if err := rows.Scan(
			&s.ID, &s.ProjectID, &s.ProjectName,
			&s.Title, &s.Status, &s.Questions, &s.Rules,
			&s.CreatedOn, &s.ModifiedOn,
		); err != nil {
			return nil, fmt.Errorf("scan survey: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetByID returns a single survey.
func (r *SurveyRepo) GetByID(ctx context.Context, id int64) (*SurveyRow, error) {
	q := `SELECT s.id, s.project_id, COALESCE(p.name, '') AS project_name,
	       s.title, s.status, COALESCE(s.questions, '[]'), COALESCE(s.rules, '[]'),
	       s.created_on, s.modified_on
	      FROM survey s
	      LEFT JOIN project p ON p.id = s.project_id
	      WHERE s.id = ?`
	var s SurveyRow
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&s.ID, &s.ProjectID, &s.ProjectName,
		&s.Title, &s.Status, &s.Questions, &s.Rules,
		&s.CreatedOn, &s.ModifiedOn,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get survey %d: %w", id, err)
	}
	return &s, nil
}

// Create inserts a new survey.
func (r *SurveyRepo) Create(ctx context.Context, projectID *int64, title, status string, questions, rules json.RawMessage) (int64, error) {
	q := `INSERT INTO survey (project_id, title, status, questions, rules) VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, q, projectID, title, status, string(questions), string(rules))
	if err != nil {
		return 0, fmt.Errorf("create survey: %w", err)
	}
	return res.LastInsertId()
}

// Update modifies a survey.
func (r *SurveyRepo) Update(ctx context.Context, id int64, title, status string, questions, rules json.RawMessage) error {
	q := `UPDATE survey SET title = ?, status = ?, questions = ?, rules = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, q, title, status, string(questions), string(rules), id)
	if err != nil {
		return fmt.Errorf("update survey %d: %w", id, err)
	}
	return nil
}

// Delete removes a survey.
func (r *SurveyRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM survey WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete survey %d: %w", id, err)
	}
	return nil
}
