package qs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Respondent maps to the QS `responder` table (10 columns verified).
type Respondent struct {
	ID                  int64          `json:"id"`
	FirstName           string         `json:"firstName"`
	LastName            string         `json:"lastName"`
	Title               sql.NullString `json:"title"`
	ExternalResponderID sql.NullString `json:"externalResponderId"`
	SessKey             sql.NullString `json:"sessKey"`
	TimeZone            sql.NullString `json:"timeZone"`
	TimeZoneAbbr        sql.NullString `json:"timeZoneAbbr"`
	LanguageCountry     sql.NullString `json:"languageCountry"`
	ModifiedOn          time.Time      `json:"modifiedOn"`
}

// RespondentListRow includes respondent + contact info.
type RespondentListRow struct {
	ID                  int64          `json:"id"`
	FirstName           string         `json:"firstName"`
	LastName            string         `json:"lastName"`
	Title               sql.NullString `json:"title"`
	ExternalResponderID sql.NullString `json:"externalResponderId"`
	TimeZone            sql.NullString `json:"timeZone"`
	Email               sql.NullString `json:"email"`
	Phone               sql.NullString `json:"phone"`
	ModifiedOn          time.Time      `json:"modifiedOn"`
}

// RespondentCommunicationAddress maps to responder_communication_address.
type RespondentCommunicationAddress struct {
	ID              int64     `json:"id"`
	ResponderID     int64     `json:"responderId"`
	TransportTypeID int       `json:"transportTypeId"` // 1=email, 2=sms, etc.
	Address         string    `json:"address"`
	Contactable     bool      `json:"contactable"`
	CreatedOn       time.Time `json:"createdOn"`
	OptedOut        bool      `json:"optedOut"`
}

// RespondentRepo handles QS respondent (participant) CRUD.
type RespondentRepo struct {
	db *sql.DB
}

// NewRespondentRepo creates a new QS respondent repository.
func NewRespondentRepo(db *sql.DB) *RespondentRepo {
	return &RespondentRepo{db: db}
}

// List returns respondents with optional search, plus email from communication_address.
func (repo *RespondentRepo) List(ctx context.Context, page, pageSize int, search string) ([]RespondentListRow, int, error) {
	var conditions []string
	var args []any

	if search != "" {
		conditions = append(conditions, "(r.first_name LIKE ? OR r.last_name LIKE ? OR rca_email.address LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count
	countQ := `SELECT COUNT(DISTINCT r.id) FROM responder r
		LEFT JOIN responder_communication_address rca_email ON rca_email.responder_id = r.id AND rca_email.transport_type_id = 1
		` + where
	var total int
	if err := repo.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count respondents: %w", err)
	}

	query := `SELECT r.id, r.first_name, r.last_name, r.title, r.external_responder_id,
		r.time_zone,
		rca_email.address AS email,
		rca_phone.address AS phone,
		r.modified_on
		FROM responder r
		LEFT JOIN responder_communication_address rca_email ON rca_email.responder_id = r.id AND rca_email.transport_type_id = 1
		LEFT JOIN responder_communication_address rca_phone ON rca_phone.responder_id = r.id AND rca_phone.transport_type_id = 2
		` + where + `
		ORDER BY r.modified_on DESC
		LIMIT ? OFFSET ?`
	listArgs := append(args, pageSize, (page-1)*pageSize)

	rows, err := repo.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list respondents: %w", err)
	}
	defer rows.Close()

	var result []RespondentListRow
	for rows.Next() {
		var r RespondentListRow
		if err := rows.Scan(
			&r.ID, &r.FirstName, &r.LastName, &r.Title, &r.ExternalResponderID,
			&r.TimeZone, &r.Email, &r.Phone, &r.ModifiedOn,
		); err != nil {
			return nil, 0, fmt.Errorf("scan respondent: %w", err)
		}
		result = append(result, r)
	}
	return result, total, rows.Err()
}

// GetByID returns a single respondent with communication addresses.
func (repo *RespondentRepo) GetByID(ctx context.Context, id int64) (*Respondent, error) {
	const q = `SELECT id, first_name, last_name, title, external_responder_id,
		sess_key, time_zone, time_zone_abbr, language_country, modified_on
		FROM responder WHERE id = ?`

	var r Respondent
	err := repo.db.QueryRowContext(ctx, q, id).Scan(
		&r.ID, &r.FirstName, &r.LastName, &r.Title, &r.ExternalResponderID,
		&r.SessKey, &r.TimeZone, &r.TimeZoneAbbr, &r.LanguageCountry, &r.ModifiedOn,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get respondent %d: %w", id, err)
	}
	return &r, nil
}

// GetCommunicationAddresses returns contact info for a respondent.
func (repo *RespondentRepo) GetCommunicationAddresses(ctx context.Context, responderID int64) ([]RespondentCommunicationAddress, error) {
	const q = `SELECT id, responder_id, transport_type_id, address, contactable, created_on, opted_out
		FROM responder_communication_address WHERE responder_id = ?`
	rows, err := repo.db.QueryContext(ctx, q, responderID)
	if err != nil {
		return nil, fmt.Errorf("get comm addresses: %w", err)
	}
	defer rows.Close()

	var result []RespondentCommunicationAddress
	for rows.Next() {
		var a RespondentCommunicationAddress
		if err := rows.Scan(&a.ID, &a.ResponderID, &a.TransportTypeID, &a.Address, &a.Contactable, &a.CreatedOn, &a.OptedOut); err != nil {
			return nil, fmt.Errorf("scan comm address: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// Create inserts a new respondent.
func (repo *RespondentRepo) Create(ctx context.Context, r *Respondent) (int64, error) {
	const q = `INSERT INTO responder
		(first_name, last_name, title, external_responder_id, sess_key, time_zone, time_zone_abbr, language_country)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := repo.db.ExecContext(ctx, q,
		r.FirstName, r.LastName, r.Title, r.ExternalResponderID,
		r.SessKey, r.TimeZone, r.TimeZoneAbbr, r.LanguageCountry)
	if err != nil {
		return 0, fmt.Errorf("create respondent: %w", err)
	}
	return res.LastInsertId()
}

// CreateCommunicationAddress adds a contact address for a respondent.
func (repo *RespondentRepo) CreateCommunicationAddress(ctx context.Context, responderID int64, transportTypeID int, address string) (int64, error) {
	const q = `INSERT INTO responder_communication_address (responder_id, transport_type_id, address, contactable)
		VALUES (?, ?, ?, 1)`
	res, err := repo.db.ExecContext(ctx, q, responderID, transportTypeID, address)
	if err != nil {
		return 0, fmt.Errorf("create comm address: %w", err)
	}
	return res.LastInsertId()
}

// Update modifies respondent fields dynamically.
func (repo *RespondentRepo) Update(ctx context.Context, id int64, fields map[string]any) error {
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
	q := "UPDATE responder SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	_, err := repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update respondent %d: %w", id, err)
	}
	return nil
}

// GetRespondentByExternalIdMRA returns responder ID by external_responder_id and project.
// Contract-identical with legacy getRespondentByExternalResponderId.
func (repo *RespondentRepo) GetRespondentByExternalIdMRA(ctx context.Context, externalResponderID string, projectID int64) ([]map[string]any, error) {
	q := `SELECT responder.id AS responderId FROM responder
	      INNER JOIN responder_communication_address resCommAddress
	        ON responder.id = resCommAddress.responder_id
	      INNER JOIN project_responder_comm_address projectResComm
	        ON projectResComm.responder_comm_address_id = resCommAddress.id
	      WHERE responder.external_responder_id = ? AND projectResComm.project_id = ?`
	rows, err := repo.db.QueryContext(ctx, q, externalResponderID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get respondent by external id mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var responderID int64
		if err := rows.Scan(&responderID); err != nil {
			return nil, fmt.Errorf("scan respondent by external id: %w", err)
		}
		records = append(records, map[string]any{"responderId": responderID})
	}
	return records, rows.Err()
}

// GetRescheduleTokenMRA returns the reschedule token for a project+responder.
func (repo *RespondentRepo) GetRescheduleTokenMRA(ctx context.Context, projectID, responderID int64) (string, error) {
	q := `SELECT reschedule_token FROM project_responder_comm_address
	      WHERE project_id = ? AND responder_id = ?`
	var token sql.NullString
	err := repo.db.QueryRowContext(ctx, q, projectID, responderID).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get reschedule token mra: %w", err)
	}
	return token.String, nil
}

// GetParticipantIdMRA returns external responder IDs for a timeslot.
func (r *RespondentRepo) GetParticipantIdMRA(ctx context.Context, timeslotID int64) ([]map[string]any, error) {
q := `SELECT external_responder_id AS externalResponderId
FROM responder
INNER JOIN conference_invitation_responder_time_slot
ON conference_invitation_responder_time_slot.responder_id = responder.id
WHERE time_slot_id = ?`
rows, err := r.db.QueryContext(ctx, q, timeslotID)
if err != nil {
return nil, fmt.Errorf("get participant id mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var extID string
if err := rows.Scan(&extID); err != nil {
return nil, err
}
records = append(records, map[string]any{"externalResponderId": extID})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}
