package iris

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ProjectRepository defines the contract for ProjectRepo.
type ProjectRepository interface {
	List(ctx context.Context, page, pageSize int, statusID *int, search string) ([]ProjectListRow, int, error)
	GetByID(ctx context.Context, id int64) (*Project, error)
	Create(ctx context.Context, p *Project) (int64, error)
	Update(ctx context.Context, id int64, fields map[string]any) error
}

// Compile-time interface satisfaction check.
var _ ProjectRepository = (*ProjectRepo)(nil)

// Project maps to the IRIS `project` table (21 columns verified).
type Project struct {
	ID                          int64          `json:"id"`
	Name                        string         `json:"name"`
	Description                 sql.NullString `json:"description"`
	SubscriptionID              int64          `json:"subscriptionId"`
	CreatedOn                   time.Time      `json:"createdOn"`
	Budget                      sql.NullString `json:"budget"`
	CreatedBy                   sql.NullInt64  `json:"createdBy"`
	IsPrivate                   bool           `json:"isPrivate"`
	QualModeratorID             sql.NullInt64  `json:"qualModeratorId"`
	ModifiedOn                  sql.NullTime   `json:"modifiedOn"`
	ModifiedBy                  sql.NullInt64  `json:"modifiedBy"`
	ProjectTypeID               int            `json:"projectTypeId"`
	SalesforceProjectID         sql.NullString `json:"salesforceProjectId"`
	SalesforceSurveyFolderID    sql.NullString `json:"salesforceSurveyFolderId"`
	CompletedOn                 sql.NullTime   `json:"completedOn"`
	ProjectStatusID             int            `json:"projectStatusId"`
	FinalizedOn                 sql.NullTime   `json:"finalizedOn"`
	IsArchived                  bool           `json:"isArchived"`
	ArchivedOn                  sql.NullTime   `json:"archivedOn"`
	ArchivedBy                  sql.NullInt64  `json:"archivedBy"`
	IsSalesforceProjectModified bool           `json:"isSalesforceProjectModified"`
}

// ProjectListRow is a flattened row for list queries that JOINs status and subscription.
type ProjectListRow struct {
	ID                  int64          `json:"id"`
	Name                string         `json:"name"`
	Description         sql.NullString `json:"description"`
	SubscriptionID      int64          `json:"subscriptionId"`
	SubscriptionCompany sql.NullString `json:"subscriptionCompany"`
	ProjectStatusID     int            `json:"projectStatusId"`
	ProjectStatusName   string         `json:"projectStatusName"`
	ProjectTypeID       int            `json:"projectTypeId"`
	SalesforceProjectID sql.NullString `json:"salesforceProjectId"`
	IsArchived          bool           `json:"isArchived"`
	CreatedOn           time.Time      `json:"createdOn"`
	ModifiedOn          sql.NullTime   `json:"modifiedOn"`
}

// ProjectRepo provides CRUD for the IRIS project table.
type ProjectRepo struct {
	rw *sql.DB // read-write
	ro *sql.DB // read-only replica
}

func NewProjectRepo(rw, ro *sql.DB) *ProjectRepo {
	if ro == nil {
		ro = rw
	}
	return &ProjectRepo{rw: rw, ro: ro}
}

const projectListQuery = `
SELECT p.id, p.name, p.description, p.subscription_id,
       s.company AS subscription_company,
       p.project_status_id, ps.name AS project_status_name,
       p.project_type_id, p.salesforce_project_id,
       p.is_archived, p.created_on, p.modified_on
FROM project p
JOIN project_status ps ON ps.id = p.project_status_id
LEFT JOIN subscription s ON s.id = p.subscription_id
WHERE p.project_type_id = 2
`

// List returns qual projects from IRIS with pagination, optional status filter and search.
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

	// Count first
	countQ := "SELECT COUNT(*) FROM project p WHERE p.project_type_id = 2" + where
	var total int
	if err := r.ro.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count iris projects: %w", err)
	}

	// Paginated list
	offset := (page - 1) * pageSize
	listQ := projectListQuery + where + " ORDER BY p.created_on DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	rows, err := r.ro.QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list iris projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectListRow
	for rows.Next() {
		var p ProjectListRow
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.SubscriptionID,
			&p.SubscriptionCompany,
			&p.ProjectStatusID, &p.ProjectStatusName,
			&p.ProjectTypeID, &p.SalesforceProjectID,
			&p.IsArchived, &p.CreatedOn, &p.ModifiedOn,
		); err != nil {
			return nil, 0, fmt.Errorf("scan iris project row: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate iris project rows: %w", err)
	}

	slog.Debug("iris projects listed", "count", len(projects), "total", total, "page", page)
	return projects, total, nil
}

// GetByID returns a single project by ID.
func (r *ProjectRepo) GetByID(ctx context.Context, id int64) (*Project, error) {
	q := `SELECT id, name, description, subscription_id, created_on, budget, created_by,
	             is_private, qual_moderator_id, modified_on, modified_by,
	             project_type_id, salesforce_project_id, salesforce_survey_folder_id,
	             completed_on, project_status_id, finalized_on,
	             is_archived, archived_on, archived_by, is_salesforce_project_modified
	      FROM project WHERE id = ?`

	var p Project
	err := r.ro.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.SubscriptionID, &p.CreatedOn, &p.Budget, &p.CreatedBy,
		&p.IsPrivate, &p.QualModeratorID, &p.ModifiedOn, &p.ModifiedBy,
		&p.ProjectTypeID, &p.SalesforceProjectID, &p.SalesforceSurveyFolderID,
		&p.CompletedOn, &p.ProjectStatusID, &p.FinalizedOn,
		&p.IsArchived, &p.ArchivedOn, &p.ArchivedBy, &p.IsSalesforceProjectModified,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get iris project %d: %w", id, err)
	}
	return &p, nil
}

// Create inserts a new project and returns the generated ID.
func (r *ProjectRepo) Create(ctx context.Context, p *Project) (int64, error) {
	q := `INSERT INTO project (name, description, subscription_id, created_on, budget, created_by,
	                           is_private, qual_moderator_id, project_type_id,
	                           salesforce_project_id, project_status_id)
	      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.rw.ExecContext(ctx, q,
		p.Name, p.Description, p.SubscriptionID, time.Now().UTC(), p.Budget, p.CreatedBy,
		p.IsPrivate, p.QualModeratorID, 2, // project_type_id = 2 (Qual)
		p.SalesforceProjectID, p.ProjectStatusID,
	)
	if err != nil {
		return 0, fmt.Errorf("create iris project: %w", err)
	}
	return res.LastInsertId()
}

// Update modifies mutable project fields.
func (r *ProjectRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	// Whitelist allowed fields
	allowed := map[string]bool{
		"name": true, "description": true, "budget": true,
		"is_private": true, "qual_moderator_id": true,
		"project_status_id": true, "salesforce_project_id": true,
		"salesforce_survey_folder_id": true, "is_archived": true,
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
	_, err := r.rw.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update iris project %d: %w", id, err)
	}
	return nil
}
