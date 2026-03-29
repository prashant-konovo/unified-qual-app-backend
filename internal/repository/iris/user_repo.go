package iris

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ICUser maps to the IRIS `ic_user` table (key columns for admin/moderator use).
type ICUser struct {
	ID               int64          `json:"id"`
	MarketID         int64          `json:"marketId"`
	FirstName        string         `json:"firstName"`
	LastName         string         `json:"lastName"`
	RegistrationDate time.Time      `json:"registrationDate"`
	OptedOut         bool           `json:"optedOut"`
	TimeZone         sql.NullString `json:"timeZone"`
	ResponderTypeID  int            `json:"responderTypeId"`
	ModifiedOn       sql.NullTime   `json:"modifiedOn"`
	LastLogin        sql.NullTime   `json:"lastLogin"`
}

// ICUserListRow is a flattened row for list queries with email and roles.
type ICUserListRow struct {
	ID               int64          `json:"id"`
	FirstName        string         `json:"firstName"`
	LastName         string         `json:"lastName"`
	Email            sql.NullString `json:"email"`
	RegistrationDate time.Time      `json:"registrationDate"`
	OptedOut         bool           `json:"optedOut"`
	TimeZone         sql.NullString `json:"timeZone"`
	ModifiedOn       sql.NullTime   `json:"modifiedOn"`
	LastLogin        sql.NullTime   `json:"lastLogin"`
	RoleIDs          string         `json:"roleIds"` // comma-separated security_role IDs
	RoleNames        string         `json:"roleNames"` // comma-separated role names
}

// ICUserWithRoles enriches a user with their security role IDs and email.
type ICUserWithRoles struct {
	ICUser
	Email     sql.NullString `json:"email"`
	RoleIDs   []int64        `json:"roleIds"`
	RoleNames []string       `json:"roleNames"`
}

// UserRepo provides read operations for the IRIS ic_user + related tables.
// Uses read-replica for list queries, primary for writes.
type UserRepo struct {
	rw *sql.DB
	ro *sql.DB
}

func NewUserRepo(rw, ro *sql.DB) *UserRepo {
	if ro == nil {
		ro = rw
	}
	return &UserRepo{rw: rw, ro: ro}
}

// adminRoleIDs are the security_role IDs relevant for the admin console.
// ADMIN=1, SUPER ADMIN=5, SUB OWNER=10, SUB ADMIN=11, QS MANAGER=15, QS MODERATOR=16, QS ADMIN=17, PANEL ADMIN=21, SHG ADMIN=23
var adminRoleIDs = []int64{1, 5, 10, 11, 15, 16, 17, 21, 23}

const irisUserListQuery = `
SELECT u.id, u.first_name, u.last_name,
       uca.address AS email,
       u.registration_date, u.opted_out, u.time_zone, u.modified_on, u.last_login,
       COALESCE(GROUP_CONCAT(DISTINCT usr.security_role_id ORDER BY usr.security_role_id), '') AS role_ids,
       COALESCE(GROUP_CONCAT(DISTINCT sr.role_name ORDER BY sr.role_name), '') AS role_names
FROM ic_user u
JOIN user_security_role usr ON usr.user_id = u.id
JOIN security_role sr ON sr.id = usr.security_role_id
LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
WHERE usr.security_role_id IN (1,5,10,11,15,16,17,21,23)
`

// List returns IRIS admin/manager/moderator users with pagination and optional filters.
func (r *UserRepo) List(ctx context.Context, page, pageSize int, roleID *int64, search string) ([]ICUserListRow, int, error) {
	where := ""
	args := []any{}

	if roleID != nil {
		where += " AND usr.security_role_id = ?"
		args = append(args, *roleID)
	}
	if search != "" {
		where += " AND (u.first_name LIKE ? OR u.last_name LIKE ? OR uca.address LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}

	// Count distinct users
	countQ := `SELECT COUNT(DISTINCT u.id) FROM ic_user u
		JOIN user_security_role usr ON usr.user_id = u.id
		LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
		WHERE usr.security_role_id IN (1,5,10,11,15,16,17,21,23)` + where
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	if err := r.ro.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count iris users: %w", err)
	}

	// Paginated list
	offset := (page - 1) * pageSize
	listQ := irisUserListQuery + where + " GROUP BY u.id ORDER BY u.last_login DESC, u.modified_on DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.ro.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list iris users: %w", err)
	}
	defer rows.Close()

	var users []ICUserListRow
	for rows.Next() {
		var u ICUserListRow
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email,
			&u.RegistrationDate, &u.OptedOut, &u.TimeZone, &u.ModifiedOn, &u.LastLogin,
			&u.RoleIDs, &u.RoleNames); err != nil {
			return nil, 0, fmt.Errorf("scan iris user row: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// GetByID returns a single IRIS user with their roles and email.
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*ICUserWithRoles, error) {
	q := `SELECT u.id, u.market_id, u.first_name, u.last_name, u.registration_date,
	             u.opted_out, u.time_zone, u.responder_type_id, u.modified_on, u.last_login,
	             uca.address AS email
	      FROM ic_user u
	      LEFT JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
	      WHERE u.id = ?`
	var u ICUserWithRoles
	err := r.ro.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.MarketID, &u.FirstName, &u.LastName, &u.RegistrationDate,
		&u.OptedOut, &u.TimeZone, &u.ResponderTypeID, &u.ModifiedOn, &u.LastLogin,
		&u.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("get iris user %d: %w", id, err)
	}

	// Fetch security roles
	roleRows, err := r.ro.QueryContext(ctx,
		`SELECT usr.security_role_id, sr.role_name
		 FROM user_security_role usr
		 JOIN security_role sr ON sr.id = usr.security_role_id
		 WHERE usr.user_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get iris user roles %d: %w", id, err)
	}
	defer roleRows.Close()
	for roleRows.Next() {
		var rid int64
		var rname string
		if err := roleRows.Scan(&rid, &rname); err != nil {
			return nil, fmt.Errorf("scan iris role: %w", err)
		}
		u.RoleIDs = append(u.RoleIDs, rid)
		u.RoleNames = append(u.RoleNames, rname)
	}
	return &u, roleRows.Err()
}

// GetByEmail returns an IRIS user by email address.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*ICUserWithRoles, error) {
	q := `SELECT u.id FROM ic_user u
	      JOIN user_communication_address uca ON uca.user_id = u.id AND uca.transport_type_id = 1
	      WHERE uca.address = ? LIMIT 1`
	var id int64
	if err := r.ro.QueryRowContext(ctx, q, email).Scan(&id); err != nil {
		return nil, fmt.Errorf("get iris user by email %s: %w", email, err)
	}
	return r.GetByID(ctx, id)
}

// Update updates an IRIS user's basic fields.
func (r *UserRepo) Update(ctx context.Context, id int64, firstName, lastName, timeZone string) error {
	sets := []string{}
	args := []any{}
	if firstName != "" {
		sets = append(sets, "first_name = ?")
		args = append(args, firstName)
	}
	if lastName != "" {
		sets = append(sets, "last_name = ?")
		args = append(args, lastName)
	}
	if timeZone != "" {
		sets = append(sets, "time_zone = ?")
		args = append(args, timeZone)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "modified_on = NOW()")
	args = append(args, id)
	q := "UPDATE ic_user SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	_, err := r.rw.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update iris user %d: %w", id, err)
	}
	slog.InfoContext(ctx, "updated IRIS user", "id", id)
	return nil
}
