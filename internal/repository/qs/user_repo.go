package qs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// User maps to the QS `user` table (11 columns verified).
type User struct {
	ID                      int64          `json:"id"`
	FirstName               sql.NullString `json:"firstName"`
	LastName                sql.NullString `json:"lastName"`
	Email                   sql.NullString `json:"email"`
	Deleted                 int            `json:"deleted"`
	CognitoUserID           sql.NullInt64  `json:"cognitoUserId"`
	TermsAccepted           int            `json:"termsAccepted"`
	ModifiedOn              time.Time      `json:"modifiedOn"`
	ModeratorBuffer         sql.NullInt64  `json:"moderatorBuffer"`
	ModeratorBufferModified sql.NullTime   `json:"moderatorBufferModifiedOn"`
	TimeZone                sql.NullString `json:"timeZone"`
}

// UserWithRoles is a User enriched with their role IDs (1=Moderator, 2=Manager, 3=Admin).
type UserWithRoles struct {
	User
	RoleIDs []int `json:"roleIds"`
}

// UserListRow is a flattened row for list queries with aggregated roles.
type UserListRow struct {
	ID            int64          `json:"id"`
	FirstName     sql.NullString `json:"firstName"`
	LastName      sql.NullString `json:"lastName"`
	Email         sql.NullString `json:"email"`
	Deleted       int            `json:"deleted"`
	TimeZone      sql.NullString `json:"timeZone"`
	ModifiedOn    time.Time      `json:"modifiedOn"`
	RoleIDs       string         `json:"roleIds"` // comma-separated from GROUP_CONCAT
}

// ModeratorAvailability maps to the QS `moderator_availability` table (6 columns verified).
type ModeratorAvailability struct {
	ID          int64     `json:"id"`
	ModeratorID int64     `json:"moderatorId"`
	ClientID    int64     `json:"clientId"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	ModifiedOn  time.Time `json:"modifiedOn"`
}

// UserRepo provides CRUD for the QS user and user_role tables.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

const qsUserListQuery = `
SELECT u.id, u.first_name, u.last_name, u.email, u.deleted, u.time_zone, u.modified_on,
       COALESCE(GROUP_CONCAT(ur.role_id ORDER BY ur.role_id), '') AS role_ids
FROM user u
LEFT JOIN user_role ur ON ur.user_id = u.id
WHERE u.deleted = 0
`

// List returns QS users with pagination and optional filters.
func (r *UserRepo) List(ctx context.Context, page, pageSize int, roleID *int, search string) ([]UserListRow, int, error) {
	where := ""
	args := []any{}

	if search != "" {
		where += " AND (u.first_name LIKE ? OR u.last_name LIKE ? OR u.email LIKE ?)"
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}

	// Count
	countQ := "SELECT COUNT(DISTINCT u.id) FROM user u LEFT JOIN user_role ur ON ur.user_id = u.id WHERE u.deleted = 0" + where
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	if roleID != nil {
		countQ += " AND ur.role_id = ?"
		countArgs = append(countArgs, *roleID)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count qs users: %w", err)
	}

	// Build list query
	listWhere := where
	if roleID != nil {
		listWhere += " AND ur.role_id = ?"
		args = append(args, *roleID)
	}

	offset := (page - 1) * pageSize
	listQ := qsUserListQuery + listWhere + " GROUP BY u.id ORDER BY u.modified_on DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list qs users: %w", err)
	}
	defer rows.Close()

	var users []UserListRow
	for rows.Next() {
		var u UserListRow
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted, &u.TimeZone, &u.ModifiedOn, &u.RoleIDs); err != nil {
			return nil, 0, fmt.Errorf("scan qs user row: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// GetByID returns a single QS user with their roles.
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*UserWithRoles, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.email, u.deleted, u.cognito_user_id,
	             u.terms_accepted, u.modified_on, u.moderator_buffer, u.moderator_buffer_modified_on, u.time_zone
	      FROM user u WHERE u.id = ?`
	var u UserWithRoles
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted, &u.CognitoUserID,
		&u.TermsAccepted, &u.ModifiedOn, &u.ModeratorBuffer, &u.ModeratorBufferModified, &u.TimeZone,
	)
	if err != nil {
		return nil, fmt.Errorf("get qs user %d: %w", id, err)
	}

	// Fetch roles
	roleRows, err := r.db.QueryContext(ctx, "SELECT role_id FROM user_role WHERE user_id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("get qs user roles %d: %w", id, err)
	}
	defer roleRows.Close()
	for roleRows.Next() {
		var rid int
		if err := roleRows.Scan(&rid); err != nil {
			return nil, fmt.Errorf("scan qs role: %w", err)
		}
		u.RoleIDs = append(u.RoleIDs, rid)
	}
	return &u, roleRows.Err()
}

// GetByEmail returns a QS user by email.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*UserWithRoles, error) {
	q := `SELECT id FROM user WHERE email = ? AND deleted = 0 LIMIT 1`
	var id int64
	if err := r.db.QueryRowContext(ctx, q, email).Scan(&id); err != nil {
		return nil, fmt.Errorf("get qs user by email %s: %w", email, err)
	}
	return r.GetByID(ctx, id)
}

// GetModerators returns all users with role_id=1 (moderator).
func (r *UserRepo) GetModerators(ctx context.Context) ([]UserListRow, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.email, u.deleted, u.time_zone, u.modified_on,
	             COALESCE(GROUP_CONCAT(ur2.role_id ORDER BY ur2.role_id), '') AS role_ids
	      FROM user u
	      JOIN user_role ur ON ur.user_id = u.id AND ur.role_id = 1
	      LEFT JOIN user_role ur2 ON ur2.user_id = u.id
	      WHERE u.deleted = 0
	      GROUP BY u.id
	      ORDER BY u.first_name, u.last_name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get qs moderators: %w", err)
	}
	defer rows.Close()

	var mods []UserListRow
	for rows.Next() {
		var u UserListRow
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted, &u.TimeZone, &u.ModifiedOn, &u.RoleIDs); err != nil {
			return nil, fmt.Errorf("scan qs moderator row: %w", err)
		}
		mods = append(mods, u)
	}
	return mods, rows.Err()
}

// ListModeratorAvailability returns availability slots for a moderator.
func (r *UserRepo) ListModeratorAvailability(ctx context.Context, moderatorID int64, clientID *int64, startDate, endDate string) ([]ModeratorAvailability, error) {
	q := `SELECT id, moderator_id, client_id, start_time, end_time, modified_on
	      FROM moderator_availability WHERE moderator_id = ?`
	args := []any{moderatorID}

	if clientID != nil {
		q += " AND client_id = ?"
		args = append(args, *clientID)
	}
	if startDate != "" {
		q += " AND start_time >= ?"
		args = append(args, startDate)
	}
	if endDate != "" {
		q += " AND end_time <= ?"
		args = append(args, endDate+" 23:59:59")
	}
	q += " ORDER BY start_time"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list qs availability: %w", err)
	}
	defer rows.Close()

	var avails []ModeratorAvailability
	for rows.Next() {
		var a ModeratorAvailability
		if err := rows.Scan(&a.ID, &a.ModeratorID, &a.ClientID, &a.StartTime, &a.EndTime, &a.ModifiedOn); err != nil {
			return nil, fmt.Errorf("scan qs availability row: %w", err)
		}
		avails = append(avails, a)
	}
	return avails, rows.Err()
}

// CreateModeratorAvailability inserts a new availability slot.
func (r *UserRepo) CreateModeratorAvailability(ctx context.Context, moderatorID, clientID int64, startTime, endTime time.Time) (*ModeratorAvailability, error) {
	q := `INSERT INTO moderator_availability (moderator_id, client_id, start_time, end_time) VALUES (?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, q, moderatorID, clientID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("create qs availability: %w", err)
	}
	id, _ := res.LastInsertId()
	slog.InfoContext(ctx, "created QS moderator availability", "id", id, "moderatorId", moderatorID)
	return &ModeratorAvailability{
		ID:          id,
		ModeratorID: moderatorID,
		ClientID:    clientID,
		StartTime:   startTime,
		EndTime:     endTime,
		ModifiedOn:  time.Now(),
	}, nil
}

// DeleteModeratorAvailability removes an availability slot.
func (r *UserRepo) DeleteModeratorAvailability(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM moderator_availability WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete qs availability %d: %w", id, err)
	}
	slog.InfoContext(ctx, "deleted QS moderator availability", "id", id)
	return nil
}

// Update updates a QS user's basic fields.
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
	args = append(args, id)
	q := "UPDATE user SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update qs user %d: %w", id, err)
	}
	slog.InfoContext(ctx, "updated QS user", "id", id)
	return nil
}

// UpdateTimeZone updates only the time_zone field for a user.
func (r *UserRepo) UpdateTimeZone(ctx context.Context, userID int64, timeZone string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET time_zone = ? WHERE id = ?", timeZone, userID)
	if err != nil {
		return fmt.Errorf("update timezone for user %d: %w", userID, err)
	}
	slog.InfoContext(ctx, "updated QS user timezone", "id", userID, "timeZone", timeZone)
	return nil
}

// UpdateModeratorBuffer updates a moderator's buffer setting.
func (r *UserRepo) UpdateModeratorBuffer(ctx context.Context, id int64, buffer int) error {
	q := "UPDATE user SET moderator_buffer = ?, moderator_buffer_modified_on = NOW() WHERE id = ?"
	_, err := r.db.ExecContext(ctx, q, buffer, id)
	if err != nil {
		return fmt.Errorf("update qs moderator buffer %d: %w", id, err)
	}
	slog.InfoContext(ctx, "updated QS moderator buffer", "id", id, "buffer", buffer)
	return nil
}

// Create inserts a new user and assigns the given role IDs.
func (r *UserRepo) Create(ctx context.Context, firstName, lastName, email, timeZone string, roleIDs []int) (int64, error) {
	q := `INSERT INTO user (first_name, last_name, email, deleted, terms_accepted, time_zone) VALUES (?, ?, ?, 0, 0, ?)`
	res, err := r.db.ExecContext(ctx, q, firstName, lastName, email, timeZone)
	if err != nil {
		return 0, fmt.Errorf("create qs user: %w", err)
	}
	uid, _ := res.LastInsertId()
	for _, rid := range roleIDs {
		_, err := r.db.ExecContext(ctx, "INSERT INTO user_role (user_id, role_id) VALUES (?, ?)", uid, rid)
		if err != nil {
			slog.WarnContext(ctx, "failed to assign role", "userId", uid, "roleId", rid, "error", err)
		}
	}
	slog.InfoContext(ctx, "created QS user", "id", uid, "email", email, "roles", roleIDs)
	return uid, nil
}

// SoftDelete marks a user as deleted.
func (r *UserRepo) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET deleted = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("soft-delete qs user %d: %w", id, err)
	}
	slog.InfoContext(ctx, "soft-deleted QS user", "id", id)
	return nil
}

// AddRoles adds role IDs to a user (skips duplicates).
func (r *UserRepo) AddRoles(ctx context.Context, userID int64, roleIDs []int) error {
	for _, rid := range roleIDs {
		_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO user_role (user_id, role_id) VALUES (?, ?)", userID, rid)
		if err != nil {
			return fmt.Errorf("add role %d to user %d: %w", rid, userID, err)
		}
	}
	slog.InfoContext(ctx, "added roles to QS user", "userId", userID, "roles", roleIDs)
	return nil
}

// DeleteRoles removes role IDs from a user.
func (r *UserRepo) DeleteRoles(ctx context.Context, userID int64, roleIDs []int) error {
	for _, rid := range roleIDs {
		_, err := r.db.ExecContext(ctx, "DELETE FROM user_role WHERE user_id = ? AND role_id = ?", userID, rid)
		if err != nil {
			return fmt.Errorf("delete role %d from user %d: %w", rid, userID, err)
		}
	}
	slog.InfoContext(ctx, "deleted roles from QS user", "userId", userID, "roles", roleIDs)
	return nil
}

// GetRoles returns role IDs for a user.
func (r *UserRepo) GetRoles(ctx context.Context, userID int64) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT role_id FROM user_role WHERE user_id = ?", userID)
	if err != nil {
		return nil, fmt.Errorf("get roles for user %d: %w", userID, err)
	}
	defer rows.Close()
	var roles []int
	for rows.Next() {
		var rid int
		if err := rows.Scan(&rid); err != nil {
			return nil, err
		}
		roles = append(roles, rid)
	}
	return roles, rows.Err()
}

// GetUserCommPreference returns communication preference for a user from user_communication_preferences.
func (r *UserRepo) GetUserCommPreference(ctx context.Context, userID int64) ([]map[string]any, error) {
	q := `SELECT allow_contact_by_email FROM user_communication_preferences WHERE user_id = ?`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get comm pref for user %d: %w", userID, err)
	}
	defer rows.Close()
	var records []map[string]any
	for rows.Next() {
		var allowContact sql.NullInt64
		if err := rows.Scan(&allowContact); err != nil {
			return nil, err
		}
		var val any
		if allowContact.Valid {
			val = allowContact.Int64
		}
		records = append(records, map[string]any{"allow_contact_by_email": val})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// UpdateUserCommPreference performs the legacy 4-query transaction on user_communication_preferences.
// Uses cognito_user_id as the lookup key (matching legacy QS Tool behavior).
func (r *UserRepo) UpdateUserCommPreference(ctx context.Context, cognitoUserID string, pmUserID string, allowContactByEmail int) ([]map[string]any, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. SET created_by where null
	res1, err := tx.ExecContext(ctx,
		"UPDATE user_communication_preferences SET created_by = ? WHERE created_by IS NULL AND cognito_user_id = ?",
		pmUserID, cognitoUserID)
	if err != nil {
		return nil, fmt.Errorf("update created_by: %w", err)
	}
	n1, _ := res1.RowsAffected()

	// 2. SET allow_contact_by_email
	res2, err := tx.ExecContext(ctx,
		"UPDATE user_communication_preferences SET allow_contact_by_email = ? WHERE cognito_user_id = ?",
		allowContactByEmail, cognitoUserID)
	if err != nil {
		return nil, fmt.Errorf("update allow_contact: %w", err)
	}
	n2, _ := res2.RowsAffected()

	// 3. SET modified_by
	res3, err := tx.ExecContext(ctx,
		"UPDATE user_communication_preferences SET modified_by = ? WHERE cognito_user_id = ?",
		pmUserID, cognitoUserID)
	if err != nil {
		return nil, fmt.Errorf("update modified_by: %w", err)
	}
	n3, _ := res3.RowsAffected()

	// 4. SELECT *
	rows, err := tx.QueryContext(ctx,
		"SELECT * FROM user_communication_preferences WHERE cognito_user_id = ?", cognitoUserID)
	if err != nil {
		return nil, fmt.Errorf("select comm prefs: %w", err)
	}
	cols, _ := rows.Columns()
	var records []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			rows.Close()
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = vals[i]
		}
		records = append(records, row)
	}
	rows.Close()

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	// Return transaction result array matching legacy data-api-client format
	result := []map[string]any{
		{"numberOfRecordsUpdated": n1},
		{"numberOfRecordsUpdated": n2},
		{"numberOfRecordsUpdated": n3},
	}
	if len(records) > 0 {
		result = append(result, map[string]any{"records": records})
	} else {
		result = append(result, map[string]any{"records": []map[string]any{}})
	}
	return result, nil
}

// AcceptTerms sets terms_accepted = 1 for a QS user.
func (r *UserRepo) AcceptTerms(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET terms_accepted = 1 WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("accept terms user %d: %w", userID, err)
	}
	slog.InfoContext(ctx, "accepted terms for QS user", "id", userID)
	return nil
}

// ListByRole returns QS users filtered by role.
func (r *UserRepo) ListByRole(ctx context.Context, roleID int, page, pageSize int) ([]UserListRow, int, error) {
	return r.List(ctx, page, pageSize, &roleID, "")
}

// UpdateAvailability updates a moderator availability slot.
func (r *UserRepo) UpdateModeratorAvailability(ctx context.Context, id int64, startTime, endTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE moderator_availability SET start_time = ?, end_time = ?, modified_on = NOW() WHERE id = ?",
		startTime, endTime, id)
	if err != nil {
		return fmt.Errorf("update qs moderator availability %d: %w", id, err)
	}
	return nil
}

// GetByEmailIncludeDeleted returns a QS user by email regardless of deleted status.
func (r *UserRepo) GetByEmailIncludeDeleted(ctx context.Context, email string) (*UserWithRoles, error) {
	q := `SELECT id, first_name, last_name, email, deleted, cognito_user_id, terms_accepted,
	             modified_on, moderator_buffer, moderator_buffer_modified_on, time_zone
	      FROM user WHERE email = ? LIMIT 1`
	var u User
	err := r.db.QueryRowContext(ctx, q, email).Scan(
		&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Deleted,
		&u.CognitoUserID, &u.TermsAccepted, &u.ModifiedOn,
		&u.ModeratorBuffer, &u.ModeratorBufferModified, &u.TimeZone,
	)
	if err != nil {
		return nil, fmt.Errorf("get qs user by email (include deleted) %s: %w", email, err)
	}
	roles, _ := r.GetRoles(ctx, u.ID)
	return &UserWithRoles{User: u, RoleIDs: roles}, nil
}

// RestoreByEmail sets deleted = 0 for a user found by email.
func (r *UserRepo) RestoreByEmail(ctx context.Context, email string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE user SET deleted = 0 WHERE email = ?", email)
	if err != nil {
		return fmt.Errorf("restore qs user by email %s: %w", email, err)
	}
	slog.InfoContext(ctx, "restored QS user", "email", email)
	return nil
}

// AddUserClient inserts a user_client association.
func (r *UserRepo) AddUserClient(ctx context.Context, userID, clientID int64) error {
	_, err := r.db.ExecContext(ctx, "INSERT IGNORE INTO user_client (user_id, client_id) VALUES (?, ?)", userID, clientID)
	if err != nil {
		return fmt.Errorf("add client %d to user %d: %w", clientID, userID, err)
	}
	return nil
}

// CreateUserCommPrefs inserts initial communication preferences for a user.
func (r *UserRepo) CreateUserCommPrefs(ctx context.Context, userID int64, email, cognitoUserID string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT IGNORE INTO user_communication_preferences (user_id, email, cognito_user_id) VALUES (?, ?, ?)",
		userID, email, cognitoUserID)
	if err != nil {
		return fmt.Errorf("create comm prefs for user %d: %w", userID, err)
	}
	return nil
}

// GetEmailByCognitoID returns the email for a user looked up by cognito_user_id.
func (r *UserRepo) GetEmailByCognitoID(ctx context.Context, cognitoID string) (string, error) {
	var email string
	err := r.db.QueryRowContext(ctx,
		"SELECT email FROM user WHERE cognito_user_id = ? LIMIT 1", cognitoID).Scan(&email)
	if err != nil {
		return "", fmt.Errorf("get email by cognito id %s: %w", cognitoID, err)
	}
	return email, nil
}

// CheckUserIsQsToolAndI2 checks if a user exists in QS and has I2 (IRIS) cross-reference.
func (r *UserRepo) CheckUserIsQsToolAndI2(ctx context.Context, email string) (map[string]any, error) {
	u, err := r.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return map[string]any{"isQsTool": false, "isI2": false, "exists": false}, nil
	}
	return map[string]any{
		"isQsTool": true,
		"isI2":     u.CognitoUserID.Valid,
		"exists":   true,
		"userId":   u.ID,
	}, nil
}

// GetAllUsersAdmin returns all users with comm prefs using the legacy JOIN query.
// Optional cognitoUserId filter narrows to a single user.
func (r *UserRepo) GetAllUsersAdmin(ctx context.Context, cognitoUserID string) ([]map[string]any, error) {
	q := `SELECT *
	FROM   (SELECT myusers.first_name                   AS firstName,
	               myusers.last_name                    AS lastName,
	               myusers.email                        AS email,
	               userpref.modified_date               AS modifiedDate,
	               userpref.allow_contact_by_email,
	               myusers.cognito_user_id              AS cognitoUserId,
	               (SELECT Concat(u.first_name, ' ', u.last_name) AS byWho
	                FROM   user u
	                WHERE  u.cognito_user_id = userpref.modified_by) AS byWho,
	               (SELECT user_role.role_id AS roleId
	                FROM   user_role
	                WHERE  user_role.user_id = myusers.id
	                ORDER  BY modified_date DESC
	                LIMIT  1)                           AS userRole
	        FROM   user myusers
	               INNER JOIN user_communication_preferences userpref
	                       ON myusers.id = userpref.user_id
	        WHERE  myusers.cognito_user_id IS NOT NULL) AS usersList
	WHERE  usersList.userRole IS NOT NULL`

	var args []any
	if cognitoUserID != "" {
		q += " AND usersList.cognitoUserId = ?"
		args = append(args, cognitoUserID)
	}
	q += " ORDER BY Concat(firstName, ' ', lastName)"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get all users admin: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var firstName, lastName, email, cogID sql.NullString
		var modifiedDate sql.NullString
		var allowContact sql.NullInt64
		var byWho sql.NullString
		var userRole sql.NullInt64
		if err := rows.Scan(&firstName, &lastName, &email, &modifiedDate,
			&allowContact, &cogID, &byWho, &userRole); err != nil {
			return nil, err
		}
		rec := map[string]any{
			"firstName":             firstName.String,
			"lastName":              lastName.String,
			"email":                 email.String,
			"cognitoUserId":         cogID.String,
		}
		if modifiedDate.Valid {
			rec["modifiedDate"] = modifiedDate.String
		} else {
			rec["modifiedDate"] = nil
		}
		if allowContact.Valid {
			rec["allow_contact_by_email"] = allowContact.Int64
		} else {
			rec["allow_contact_by_email"] = nil
		}
		if byWho.Valid {
			rec["byWho"] = byWho.String
		} else {
			rec["byWho"] = nil
		}
		if userRole.Valid {
			rec["userRole"] = userRole.Int64
		} else {
			rec["userRole"] = nil
		}
		records = append(records, rec)
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// GetAllModeratorsListMRA returns all moderators for a client with interview count,
// matching legacy getAllModeratorsList SQL exactly.
func (r *UserRepo) GetAllModeratorsListMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT
		user.first_name AS firstName,
		user.last_name AS lastName,
		user.id AS id,
		COUNT(DISTINCT moderator_time_slot.id) AS interviewCount
	FROM user
	INNER JOIN user_client ON user.id = user_client.user_id
	INNER JOIN user_role ON user.id = user_role.user_id
	LEFT OUTER JOIN (
		SELECT moderator_time_slot.*
		FROM moderator_time_slot
		INNER JOIN time_slot ON time_slot.id = moderator_time_slot.time_slot_id
			AND time_slot.status_id = 2
			AND time_slot.is_invalid = FALSE
	) moderator_time_slot ON user.id = moderator_time_slot.moderator_id
	WHERE user_client.client_id = ?
		AND user_role.role_id = 1
		AND user.deleted = 0
	GROUP BY user.id
	ORDER BY firstName, lastName ASC`

	rows, err := r.db.QueryContext(ctx, q, clientID)
	if err != nil {
		return nil, fmt.Errorf("get all moderators list: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var firstName, lastName string
		var id, interviewCount int64
		if err := rows.Scan(&firstName, &lastName, &id, &interviewCount); err != nil {
			return nil, fmt.Errorf("scan moderator row: %w", err)
		}
		records = append(records, map[string]any{
			"firstName":      firstName,
			"lastName":       lastName,
			"id":             id,
			"interviewCount": interviewCount,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// ──────────────────────────────────────────────
// MRA #50 — UpdateModeratorMRA repo methods
// ──────────────────────────────────────────────

// FetchUserInfoByUserIdMRA returns moderatorBuffer and clientId for a user.
func (r *UserRepo) FetchUserInfoByUserIdMRA(ctx context.Context, userID int64) (int, int64, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.time_zone, ur.modified_on,
		u.email, r.id as roles, u.moderator_buffer as moderatorBuffer,
		u.moderator_buffer_modified_on as moderatorBufferModifiedOn, c.client_id as clientId
		FROM user u LEFT OUTER JOIN user_role ur ON (u.id = ur.user_id)
		LEFT OUTER JOIN role r ON (ur.role_id = r.id)
		LEFT OUTER JOIN user_client c ON (c.user_id = u.id)
		WHERE u.id = ?
		ORDER BY ur.modified_on DESC
		LIMIT 1`
	var (
		id                        int64
		firstName, lastName       sql.NullString
		timeZone                  sql.NullString
		modifiedOn                sql.NullTime
		email                     sql.NullString
		roles                     sql.NullInt64
		moderatorBuffer           sql.NullInt64
		moderatorBufferModifiedOn sql.NullTime
		clientID                  sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&id, &firstName, &lastName, &timeZone, &modifiedOn,
		&email, &roles, &moderatorBuffer, &moderatorBufferModifiedOn, &clientID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch user info by id mra %d: %w", userID, err)
	}
	buf := 0
	if moderatorBuffer.Valid {
		buf = int(moderatorBuffer.Int64)
	}
	cid := int64(0)
	if clientID.Valid {
		cid = clientID.Int64
	}
	return buf, cid, nil
}

// UpdateModeratorBufferMRA updates the moderator_buffer for a user.
func (r *UserRepo) UpdateModeratorBufferMRA(ctx context.Context, userID int64, buffer int) error {
	q := `UPDATE user SET moderator_buffer = ?, moderator_buffer_modified_on = now() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, q, buffer, userID)
	if err != nil {
		return fmt.Errorf("update moderator buffer mra %d: %w", userID, err)
	}
	return nil
}

// ModeratorTimeslotMRA represents a future scheduled interview for a moderator.
type ModeratorTimeslotMRA struct {
	TimeSlotID     int64          `json:"timeSlotId"`
	StartTime      time.Time      `json:"startTime"`
	EndTime        time.Time      `json:"endTime"`
	Completed      int            `json:"completed"`
	Duration       sql.NullInt64  `json:"duration"`
	ImportedOverlap string        `json:"importedOverLap"`
	IntervieweeID  sql.NullInt64  `json:"intervieweeId"`
	ProjectID      int64          `json:"projectId"`
	ProjectName    sql.NullString `json:"projectName"`
}

// GetFutureModeratorTimeslotsMRA returns future scheduled interviews for a moderator+client.
func (r *UserRepo) GetFutureModeratorTimeslotsMRA(ctx context.Context, moderatorID, clientID int64) ([]ModeratorTimeslotMRA, error) {
	q := `SELECT t.id AS timeSlotId, t.start_time AS startTime, t.end_time AS endTime,
		t.status_id AS completed, t.duration,
		(CASE WHEN t.has_imported_overlap IS NULL THEN '0' ELSE t.has_imported_overlap END) AS importedOverLap,
		ad.responder_id AS intervieweeId, t.project_id AS projectId, p.name AS projectName
		FROM time_slot t INNER JOIN project p ON p.id = t.project_id
		INNER JOIN answer_details ad ON ad.time_slot_id = t.id
		WHERE t.id IN (SELECT time_slot_id FROM moderator_time_slot WHERE moderator_id = ?)
		AND t.status_id IN (2, 7, 8, 9) AND p.client_id = ? AND t.start_time > now()`
	rows, err := r.db.QueryContext(ctx, q, moderatorID, clientID)
	if err != nil {
		return nil, fmt.Errorf("get future moderator timeslots mra: %w", err)
	}
	defer rows.Close()
	var records []ModeratorTimeslotMRA
	for rows.Next() {
		var ts ModeratorTimeslotMRA
		if err := rows.Scan(&ts.TimeSlotID, &ts.StartTime, &ts.EndTime, &ts.Completed,
			&ts.Duration, &ts.ImportedOverlap, &ts.IntervieweeID, &ts.ProjectID, &ts.ProjectName); err != nil {
			return nil, fmt.Errorf("scan future moderator timeslot mra: %w", err)
		}
		records = append(records, ts)
	}
	return records, rows.Err()
}

// AvailWithProximityMRA represents an availability near a scheduled interview.
type AvailWithProximityMRA struct {
	ID        int64     `json:"id"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

// GetFutureAvailsWithProximityMRA returns manual availabilities near future interviews (60min proximity).
func (r *UserRepo) GetFutureAvailsWithProximityMRA(ctx context.Context, moderatorID, clientID int64) ([]AvailWithProximityMRA, error) {
	q := `SELECT DISTINCT ma.id AS id, ma.start_time AS startTime, ma.end_time AS endTime
		FROM moderator_availability ma
		INNER JOIN user u ON u.id = ma.moderator_id
		INNER JOIN (
			SELECT ts.start_time AS startTime, ts.end_time AS endTime
			FROM moderator_time_slot mts INNER JOIN time_slot ts ON mts.time_slot_id = ts.id
			WHERE mts.moderator_id = ? AND ts.start_time > now() AND ts.status_id IN (2, 7, 8, 9)
		) moderator_future_interviews
		ON DATE_ADD(ma.end_time, INTERVAL 60 MINUTE) >= moderator_future_interviews.startTime
		AND DATE_SUB(ma.start_time, INTERVAL 60 MINUTE) <= moderator_future_interviews.endTime
		WHERE ma.moderator_id = ? AND ma.client_id = ? AND ma.start_time > now()`
	rows, err := r.db.QueryContext(ctx, q, moderatorID, moderatorID, clientID)
	if err != nil {
		return nil, fmt.Errorf("get future avails with proximity mra: %w", err)
	}
	defer rows.Close()
	var records []AvailWithProximityMRA
	for rows.Next() {
		var a AvailWithProximityMRA
		if err := rows.Scan(&a.ID, &a.StartTime, &a.EndTime); err != nil {
			return nil, fmt.Errorf("scan avail with proximity mra: %w", err)
		}
		records = append(records, a)
	}
	return records, rows.Err()
}

// GetFutureImportedAvailsWithProximityMRA returns imported availabilities near future interviews (60min proximity).
func (r *UserRepo) GetFutureImportedAvailsWithProximityMRA(ctx context.Context, moderatorID, clientID int64) ([]AvailWithProximityMRA, error) {
	q := `SELECT DISTINCT ma.id AS id, ma.start_time AS startTime, ma.end_time AS endTime
		FROM imported_moderator_availability ma
		INNER JOIN user u ON u.id = ma.moderator_id
		INNER JOIN (
			SELECT ts.start_time AS startTime, ts.end_time AS endTime
			FROM moderator_time_slot mts INNER JOIN time_slot ts ON mts.time_slot_id = ts.id
			WHERE mts.moderator_id = ? AND ts.start_time > now() AND ts.status_id IN (2, 7, 8, 9)
		) moderator_future_interviews
		ON DATE_ADD(ma.end_time, INTERVAL 60 MINUTE) >= moderator_future_interviews.startTime
		AND DATE_SUB(ma.start_time, INTERVAL 60 MINUTE) <= moderator_future_interviews.endTime
		WHERE ma.moderator_id = ? AND ma.client_id = ? AND ma.start_time > now()`
	rows, err := r.db.QueryContext(ctx, q, moderatorID, moderatorID, clientID)
	if err != nil {
		return nil, fmt.Errorf("get future imported avails with proximity mra: %w", err)
	}
	defer rows.Close()
	var records []AvailWithProximityMRA
	for rows.Next() {
		var a AvailWithProximityMRA
		if err := rows.Scan(&a.ID, &a.StartTime, &a.EndTime); err != nil {
			return nil, fmt.Errorf("scan imported avail with proximity mra: %w", err)
		}
		records = append(records, a)
	}
	return records, rows.Err()
}

// AvailLengthMRA holds start, end, and length in minutes for an availability.
type AvailLengthMRA struct {
	StartTime time.Time
	EndTime   time.Time
	Length    int
}

// GetModeratorAvailabilityLengthMRA returns start_time, end_time, and length in minutes.
func (r *UserRepo) GetModeratorAvailabilityLengthMRA(ctx context.Context, availID int64) (*AvailLengthMRA, error) {
	q := `SELECT start_time, end_time, TIMESTAMPDIFF(MINUTE, start_time, end_time) AS av_length
		FROM moderator_availability WHERE id = ?`
	var a AvailLengthMRA
	err := r.db.QueryRowContext(ctx, q, availID).Scan(&a.StartTime, &a.EndTime, &a.Length)
	if err != nil {
		return nil, fmt.Errorf("get moderator availability length mra %d: %w", availID, err)
	}
	return &a, nil
}

// GetImportedModeratorAvailabilityLengthMRA returns start_time, end_time, and length for imported avail.
func (r *UserRepo) GetImportedModeratorAvailabilityLengthMRA(ctx context.Context, availID int64) (*AvailLengthMRA, error) {
	q := `SELECT start_time, end_time, TIMESTAMPDIFF(MINUTE, start_time, end_time) AS av_length
		FROM imported_moderator_availability WHERE id = ?`
	var a AvailLengthMRA
	err := r.db.QueryRowContext(ctx, q, availID).Scan(&a.StartTime, &a.EndTime, &a.Length)
	if err != nil {
		return nil, fmt.Errorf("get imported moderator availability length mra %d: %w", availID, err)
	}
	return &a, nil
}

// DeleteModeratorAvailabilityByIdMRA deletes from moderator_availability by id.
func (r *UserRepo) DeleteModeratorAvailabilityByIdMRA(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM moderator_availability WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete moderator availability mra %d: %w", id, err)
	}
	return nil
}

// DeleteImportedModeratorAvailabilityByIdMRA deletes from imported_moderator_availability by id.
func (r *UserRepo) DeleteImportedModeratorAvailabilityByIdMRA(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM imported_moderator_availability WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete imported moderator availability mra %d: %w", id, err)
	}
	return nil
}

// UpdateModeratorAvailabilityStartTimeMRA updates start_time for a moderator_availability.
func (r *UserRepo) UpdateModeratorAvailabilityStartTimeMRA(ctx context.Context, id int64, startTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE moderator_availability SET start_time = ? WHERE id = ?", startTime, id)
	if err != nil {
		return fmt.Errorf("update moderator availability start time mra %d: %w", id, err)
	}
	return nil
}

// UpdateModeratorAvailabilityEndTimeMRA updates end_time for a moderator_availability.
func (r *UserRepo) UpdateModeratorAvailabilityEndTimeMRA(ctx context.Context, id int64, endTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE moderator_availability SET end_time = ? WHERE id = ?", endTime, id)
	if err != nil {
		return fmt.Errorf("update moderator availability end time mra %d: %w", id, err)
	}
	return nil
}

// UpdateImportedModeratorAvailabilityStartTimeMRA updates start_time for imported_moderator_availability.
func (r *UserRepo) UpdateImportedModeratorAvailabilityStartTimeMRA(ctx context.Context, id int64, startTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE imported_moderator_availability SET start_time = ? WHERE id = ?", startTime, id)
	if err != nil {
		return fmt.Errorf("update imported moderator availability start time mra %d: %w", id, err)
	}
	return nil
}

// UpdateImportedModeratorAvailabilityEndTimeMRA updates end_time for imported_moderator_availability.
func (r *UserRepo) UpdateImportedModeratorAvailabilityEndTimeMRA(ctx context.Context, id int64, endTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE imported_moderator_availability SET end_time = ? WHERE id = ?", endTime, id)
	if err != nil {
		return fmt.Errorf("update imported moderator availability end time mra %d: %w", id, err)
	}
	return nil
}

// CleanUpAvailabilitiesByModeratorIdMRA deletes availabilities where start_time >= end_time.
func (r *UserRepo) CleanUpAvailabilitiesByModeratorIdMRA(ctx context.Context, moderatorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM imported_moderator_availability WHERE start_time >= end_time AND moderator_id = ?", moderatorID)
	if err != nil {
		return fmt.Errorf("cleanup imported availabilities mra %d: %w", moderatorID, err)
	}
	_, err = r.db.ExecContext(ctx,
		"DELETE FROM moderator_availability WHERE start_time >= end_time AND moderator_id = ?", moderatorID)
	if err != nil {
		return fmt.Errorf("cleanup availabilities mra %d: %w", moderatorID, err)
	}
	return nil
}

// RemoveNestedAvailabilitiesMRA removes availabilities that are fully contained within another.
func (r *UserRepo) RemoveNestedAvailabilitiesMRA(ctx context.Context, moderatorID int64) error {
	q1 := `DELETE FROM moderator_availability WHERE id IN (
		SELECT innerTable.id FROM (
			SELECT ma1.id FROM moderator_availability ma1 JOIN moderator_availability ma2
			ON ma1.start_time >= ma2.start_time AND ma1.end_time <= ma2.end_time
				AND ma1.end_time <= ma2.end_time AND ma1.start_time <= ma2.end_time
				AND ma1.end_time <= ma2.end_time AND ma1.moderator_id = ma2.moderator_id
			WHERE ma1.id != ma2.id AND ma1.moderator_id = ?
		) innerTable
	) AND moderator_id = ?`
	_, err := r.db.ExecContext(ctx, q1, moderatorID, moderatorID)
	if err != nil {
		return fmt.Errorf("remove nested availabilities mra %d: %w", moderatorID, err)
	}

	q2 := `DELETE FROM imported_moderator_availability WHERE id IN (
		SELECT innerTable.id FROM (
			SELECT ma1.id FROM imported_moderator_availability ma1 JOIN imported_moderator_availability ma2
			ON ma1.start_time >= ma2.start_time AND ma1.end_time <= ma2.end_time
				AND ma1.end_time <= ma2.end_time AND ma1.start_time <= ma2.end_time
				AND ma1.end_time <= ma2.end_time AND ma1.moderator_id = ma2.moderator_id
			WHERE ma1.id != ma2.id AND ma1.moderator_id = ?
		) innerTable
	) AND moderator_id = ?`
	_, err = r.db.ExecContext(ctx, q2, moderatorID, moderatorID)
	if err != nil {
		return fmt.Errorf("remove nested imported availabilities mra %d: %w", moderatorID, err)
	}
	return nil
}

// GetModeratorBufferMRA returns the moderator_buffer for a user (default 15 if NULL).
func (r *UserRepo) GetModeratorBufferMRA(ctx context.Context, moderatorID int64) (int, error) {
	q := `SELECT COALESCE(moderator_buffer, 15) FROM user WHERE id = ?`
	var buffer int
	if err := r.db.QueryRowContext(ctx, q, moderatorID).Scan(&buffer); err != nil {
		return 15, fmt.Errorf("get moderator buffer: %w", err)
	}
	return buffer, nil
}

// GetModExternalCalendarStatusMRA returns the external calendar import status for a moderator.
func (r *UserRepo) GetModExternalCalendarStatusMRA(ctx context.Context, moderatorID int64) (string, error) {
	q := `SELECT COALESCE(status, '') FROM moderator_external_calendar WHERE moderator_id = ? ORDER BY id DESC LIMIT 1`
	var status string
	err := r.db.QueryRowContext(ctx, q, moderatorID).Scan(&status)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get external calendar status: %w", err)
	}
	return status, nil
}

// IsValidAvailabilityMRA checks if a new availability conflicts with existing timeslots,
// matching legacy isValidAvailability SQL exactly.
func (r *UserRepo) IsValidAvailabilityMRA(ctx context.Context, moderatorID int64, startTime, endTime string, buffer int) (int64, error) {
	q := `SELECT count(time_slot.id) AS count FROM time_slot
		INNER JOIN moderator_time_slot ON time_slot.id = moderator_time_slot.time_slot_id
		WHERE moderator_time_slot.moderator_id = ?
		AND time_slot.status_id IN (2, 7, 8, 9)
		AND time_slot.is_invalid = false
		AND (
			DATE_SUB(time_slot.start_time, INTERVAL ? MINUTE) <= ? AND DATE_ADD(time_slot.end_time, INTERVAL ? MINUTE) >= ?
			OR (
				(time_slot.start_time >= ? AND time_slot.end_time <= ?)
				OR (time_slot.start_time > ? AND time_slot.end_time <= ?)
				OR (time_slot.start_time >= ? AND time_slot.end_time < ?)
				OR (time_slot.start_time > ? AND DATE_SUB(time_slot.end_time, INTERVAL ? MINUTE) <= ?)
				OR (DATE_ADD(time_slot.start_time, INTERVAL ? MINUTE) >= ? AND time_slot.end_time < ?)
				OR (DATE_SUB(time_slot.start_time, INTERVAL ? MINUTE) <= ? AND ? = time_slot.start_time)
				OR (DATE_ADD(time_slot.end_time, INTERVAL ? MINUTE) >= ? AND time_slot.end_time = ?)
				OR (DATE_SUB(time_slot.start_time, INTERVAL ? MINUTE) < ? AND ? < DATE_SUB(time_slot.start_time, INTERVAL ? MINUTE))
				OR (DATE_ADD(time_slot.end_time, INTERVAL ? MINUTE) > ? AND ? > DATE_ADD(time_slot.start_time, INTERVAL ? MINUTE))
			)
		)`
	var count int64
	err := r.db.QueryRowContext(ctx, q,
		moderatorID,
		buffer, startTime, buffer, endTime,
		startTime, endTime,
		startTime, endTime,
		startTime, endTime,
		startTime, buffer, endTime,
		buffer, startTime, endTime,
		buffer, endTime, endTime,
		buffer, startTime, startTime,
		buffer, endTime, startTime, buffer,
		buffer, startTime, endTime, buffer,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("check availability validity: %w", err)
	}
	return count, nil
}

// OverlappingAvailabilitiesMRA returns overlapping availabilities for a moderator,
// matching legacy overLappingAvailabilities SQL.
func (r *UserRepo) OverlappingAvailabilitiesMRA(ctx context.Context, moderatorID int64, startTime, endTime string) ([]map[string]any, error) {
	q := `SELECT id, moderator_id AS moderatorId, client_id AS clientId, start_time AS startTime, end_time AS endTime
		FROM moderator_availability
		WHERE moderator_availability.moderator_id = ?
		AND (
			(moderator_availability.start_time >= ? AND moderator_availability.start_time <= ?)
			OR (moderator_availability.end_time >= ? AND moderator_availability.end_time <= ?)
			OR (moderator_availability.start_time <= ? AND moderator_availability.end_time >= ?)
		)`
	rows, err := r.db.QueryContext(ctx, q, moderatorID, startTime, endTime, startTime, endTime, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("get overlapping availabilities: %w", err)
	}
	defer rows.Close()
	var records []map[string]any
	for rows.Next() {
		var id, modID, clientID int64
		var st, et string
		if err := rows.Scan(&id, &modID, &clientID, &st, &et); err != nil {
			return nil, fmt.Errorf("scan overlapping availability: %w", err)
		}
		records = append(records, map[string]any{
			"id": id, "moderatorId": modID, "clientId": clientID, "startTime": st, "endTime": et,
		})
	}
	return records, rows.Err()
}

// GetAllModeratorAvailabilityMRA returns all manual availabilities for a moderator+client.
func (r *UserRepo) GetAllModeratorAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	q := `SELECT id, moderator_id AS moderatorId, client_id AS clientId, start_time AS startTime, end_time AS endTime
		FROM moderator_availability
		WHERE moderator_id = ? AND client_id = ?
		ORDER BY start_time ASC`
	rows, err := r.db.QueryContext(ctx, q, moderatorID, clientID)
	if err != nil {
		return nil, fmt.Errorf("get all moderator availability: %w", err)
	}
	defer rows.Close()
	var records []map[string]any
	for rows.Next() {
		var id, modID, cID int64
		var st, et string
		if err := rows.Scan(&id, &modID, &cID, &st, &et); err != nil {
			return nil, fmt.Errorf("scan moderator availability: %w", err)
		}
		records = append(records, map[string]any{
			"id": id, "moderatorId": modID, "clientId": cID, "startTime": st, "endTime": et, "isImported": false,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// GetNonOverlappingManualAvailabilityMRA returns manual availabilities with no overlap with imported.
func (r *UserRepo) GetNonOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT ma.id,
        ma.moderator_id AS moderatorId,
        first_name      AS firstName,
        last_name       AS lastName,
        ma.client_id    AS clientId,
        ma.start_time   AS startTime,
        ma.end_time     AS endTime
FROM   moderator_availability ma
INNER JOIN user ON user.id = ma.moderator_id
AND ma.id NOT IN (SELECT DISTINCT ma.id
              FROM   imported_moderator_availability ima
                     INNER JOIN moderator_availability ma
                             ON ma.moderator_id = ima.moderator_id
                                AND ( (ima.start_time > ma.start_time and ima.end_time > ma.start_time AND ima.end_time <= ma.end_time)
                                    OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time >= ma.start_time)
                                    OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time > ma.start_time) )
                                AND ma.id NOT IN (SELECT ma.id FROM moderator_availability ma
                     INNER JOIN imported_moderator_availability ima ON ima.moderator_id = ma.moderator_id
                     WHERE ima.end_time = ma.end_time AND ima.start_time = ma.start_time AND ma.moderator_id = ?)
              WHERE  ima.moderator_id = ? AND ima.client_id = ?)
AND ma.id NOT IN (SELECT ma.id FROM imported_moderator_availability ima
     INNER JOIN moderator_availability ma ON ma.moderator_id = ima.moderator_id
     WHERE ma.end_time = ima.end_time AND ma.start_time = ima.start_time AND ima.moderator_id = ?)
WHERE  ma.moderator_id = ? AND ma.client_id = ?
ORDER  BY ma.start_time`
	return r.scanAvailabilityRows(ctx, q, moderatorID, moderatorID, clientID, moderatorID, moderatorID, clientID)
}

// GetNonOverlappingImportedAvailabilityMRA returns imported availabilities that exactly match manual ones.
func (r *UserRepo) GetNonOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT ima.id,
        ima.moderator_id AS moderatorId,
        first_name       AS firstName,
        last_name        AS lastName,
        ima.client_id    AS clientId,
        ima.start_time   AS startTime,
        ima.end_time     AS endTime
FROM   imported_moderator_availability ima
INNER JOIN user ON user.id = ima.moderator_id
WHERE  ima.moderator_id = ? AND ima.client_id = ?
AND ima.id NOT IN (SELECT DISTINCT ima.id
              FROM   imported_moderator_availability ima
                     INNER JOIN moderator_availability ma
                             ON ma.moderator_id = ima.moderator_id
                                AND ( (ima.start_time > ma.start_time and ima.end_time > ma.start_time AND ima.end_time <= ma.end_time)
                                    OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time >= ma.start_time)
                                    OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time > ma.start_time) )
                                AND ma.id NOT IN (SELECT ma.id FROM moderator_availability ma
                     INNER JOIN imported_moderator_availability ima ON ima.moderator_id = ma.moderator_id
                     WHERE ima.end_time = ma.end_time AND ima.start_time = ma.start_time AND ma.moderator_id = ?)
              WHERE  ima.moderator_id = ? AND ima.client_id = ?)
AND ima.id IN (SELECT DISTINCT ima.id FROM imported_moderator_availability ima
       INNER JOIN moderator_availability ma ON ma.moderator_id = ima.moderator_id
       AND ima.end_time = ma.end_time AND ima.start_time = ma.start_time AND ma.moderator_id = ?)
ORDER  BY ima.start_time`
	return r.scanAvailabilityRows(ctx, q, moderatorID, clientID, moderatorID, moderatorID, clientID, moderatorID)
}

// GetAllOverlappingManualAvailabilityMRA returns manual availabilities that overlap with imported (excluding exact matches).
func (r *UserRepo) GetAllOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT ma.id,
        ma.moderator_id AS moderatorId,
        first_name      AS firstName,
        last_name       AS lastName,
        ma.client_id    AS clientId,
        ma.start_time   AS startTime,
        ma.end_time     AS endTime
FROM   moderator_availability ma
INNER JOIN user ON user.id = ma.moderator_id
INNER JOIN imported_moderator_availability ima ON ima.moderator_id = ma.moderator_id
AND ma.id IN (SELECT DISTINCT ma.id
              FROM   imported_moderator_availability ima
                     LEFT JOIN moderator_availability ma
                             ON ma.moderator_id = ima.moderator_id
                                AND ( (ima.start_time > ma.start_time and ima.end_time > ma.start_time AND ima.end_time <= ma.end_time)
                                    OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time >= ma.start_time)
                                    OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time > ma.start_time) )
                                AND ma.id NOT IN (SELECT ma.id FROM moderator_availability ma
                     INNER JOIN imported_moderator_availability ima ON ima.moderator_id = ma.moderator_id
                     WHERE ima.end_time = ma.end_time AND ima.start_time = ma.start_time AND ma.moderator_id = ?)
              WHERE  ima.moderator_id = ? AND ima.client_id = ?)
AND ma.id NOT IN (SELECT ma.id FROM imported_moderator_availability ima
     INNER JOIN moderator_availability ma ON ma.moderator_id = ima.moderator_id
     WHERE ma.end_time = ima.end_time AND ma.start_time = ima.start_time AND ima.moderator_id = ?)
WHERE  ma.moderator_id = ? AND ma.client_id = ?
ORDER  BY ma.start_time`
	return r.scanAvailabilityRows(ctx, q, moderatorID, moderatorID, clientID, moderatorID, moderatorID, clientID)
}

// GetAllOverlappingImportedAvailabilityMRA returns imported availabilities that overlap with manual (excluding exact matches).
func (r *UserRepo) GetAllOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	q := `SELECT DISTINCT ima.id,
        ima.moderator_id AS moderatorId,
        first_name       AS firstName,
        last_name        AS lastName,
        ima.client_id    AS clientId,
        ima.start_time   AS startTime,
        ima.end_time     AS endTime
FROM   imported_moderator_availability ima
INNER JOIN user ON user.id = ima.moderator_id
INNER JOIN moderator_availability ma ON ma.moderator_id = ima.moderator_id
   AND ( (ima.start_time > ma.start_time and ima.end_time > ma.start_time AND ima.end_time <= ma.end_time)
       OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time >= ma.start_time)
       OR (ima.start_time < ma.end_time AND ima.end_time < ma.end_time and ima.start_time > ma.start_time) )
   AND ma.id NOT IN (SELECT ma.id FROM moderator_availability ma
        INNER JOIN imported_moderator_availability ima ON ima.moderator_id = ma.moderator_id
        WHERE ima.end_time = ma.end_time AND ima.start_time = ma.start_time AND ma.moderator_id = ?)
WHERE  ima.moderator_id = ? AND ima.client_id = ?
ORDER  BY ima.start_time`
	return r.scanAvailabilityRows(ctx, q, moderatorID, moderatorID, clientID)
}

// scanAvailabilityRows is a helper to scan availability result rows into []map[string]any.
func (r *UserRepo) scanAvailabilityRows(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []map[string]any
	for rows.Next() {
		var id, modID, clientID int64
		var firstName, lastName, st, et string
		if err := rows.Scan(&id, &modID, &firstName, &lastName, &clientID, &st, &et); err != nil {
			return nil, err
		}
		records = append(records, map[string]any{
			"id": id, "moderatorId": modID, "firstName": firstName, "lastName": lastName,
			"clientId": clientID, "startTime": st, "endTime": et,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// FindModeratorAvailabilityByIdMRA finds a single moderator availability by ID.
func (r *UserRepo) FindModeratorAvailabilityByIdMRA(ctx context.Context, id int64) (map[string]any, error) {
	q := `SELECT id, moderator_id AS moderatorId, client_id AS clientId, start_time AS startTime, end_time AS endTime
		FROM moderator_availability WHERE id = ?`
	var avID, modID, clientID int64
	var st, et string
	err := r.db.QueryRowContext(ctx, q, id).Scan(&avID, &modID, &clientID, &st, &et)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find moderator availability by id: %w", err)
	}
	return map[string]any{
		"id": avID, "moderatorId": modID, "clientId": clientID,
		"startTime": st, "endTime": et, "moderator_id": modID, "client_id": clientID,
	}, nil
}

// GetModeratorsInfoByAvailabilityIdMRA returns moderator info (with buffer) by availability ID.
func (r *UserRepo) GetModeratorsInfoByAvailabilityIdMRA(ctx context.Context, availabilityID int64) (map[string]any, error) {
	q := `SELECT moderator_id AS id, first_name AS firstName, last_name AS lastName, email,
		COALESCE(moderator_buffer, 15) AS moderatorBuffer
		FROM moderator_availability
		INNER JOIN user ON user.id = moderator_availability.moderator_id
		WHERE moderator_availability.id = ?`
	var id int64
	var firstName, lastName, email string
	var buffer int
	err := r.db.QueryRowContext(ctx, q, availabilityID).Scan(&id, &firstName, &lastName, &email, &buffer)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get moderator info by availability id: %w", err)
	}
	return map[string]any{
		"id": id, "firstName": firstName, "lastName": lastName,
		"email": email, "moderatorBuffer": buffer,
	}, nil
}

// GetAllModeratorAvailabilityWithUserMRA returns all moderator availabilities with user info (firstName, lastName).
func (r *UserRepo) GetAllModeratorAvailabilityWithUserMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
	q := `SELECT ma.id, moderator_id AS moderatorId, first_name AS firstName, last_name AS lastName,
		client_id AS clientId, start_time AS startTime, end_time AS endTime
		FROM moderator_availability ma
		INNER JOIN user ON user.id = ma.moderator_id
		WHERE moderator_id = ? AND client_id = ?`
	return r.scanAvailabilityRows(ctx, q, moderatorID, clientID)
}

// GetOverlappingImportedAvailabilityMRA returns imported availabilities overlapping a given time range.
func (r *UserRepo) GetOverlappingImportedAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) ([]map[string]any, error) {
	q := `SELECT ima.id, moderator_id AS moderatorId, first_name AS firstName, last_name AS lastName,
		client_id AS clientId, start_time AS startTime, end_time AS endTime
		FROM imported_moderator_availability ima
		INNER JOIN user ON user.id = ima.moderator_id
		WHERE moderator_id = ? AND client_id = ?
		AND ((start_time >= ? AND start_time < ?)
			OR (end_time > ? AND end_time <= ?)
			OR (start_time < ? AND end_time > ?))
		ORDER BY start_time`
	return r.scanAvailabilityRows(ctx, q, moderatorID, clientID, startTime, endTime, startTime, endTime, startTime, endTime)
}

// AddModeratorAvailabilityFromImportedMRA creates a manual availability record from imported availability data.
func (r *UserRepo) AddModeratorAvailabilityFromImportedMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) error {
	q := `INSERT INTO moderator_availability (moderator_id, client_id, start_time, end_time) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, moderatorID, clientID, startTime, endTime)
	return err
}

// GetAllModeratorsAvailabilityPerClientMRA returns moderator availabilities for a client+project.
// Contract-identical with legacy getAllModeratorsAvailabilityPerClient.
func (r *UserRepo) GetAllModeratorsAvailabilityPerClientMRA(ctx context.Context, clientID, projectID int64) ([]map[string]any, error) {
	q := `SELECT moderator_availability.id, moderator_id AS moderatorId,
	      client_id AS clientId, start_time AS startTime, end_time AS endTime
	      FROM moderator_availability
	      INNER JOIN projects_users ON moderator_availability.moderator_id = projects_users.user_id
	      WHERE client_id = ? AND project_id = ?`
	rows, err := r.db.QueryContext(ctx, q, clientID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get all moderators availability per client mra: %w", err)
	}
	defer rows.Close()

	var records []map[string]any
	for rows.Next() {
		var id, modID, cID int64
		var st, et string
		if err := rows.Scan(&id, &modID, &cID, &st, &et); err != nil {
			return nil, fmt.Errorf("scan moderator availability mra: %w", err)
		}
		records = append(records, map[string]any{
			"id":          id,
			"moderatorId": modID,
			"clientId":    cID,
			"startTime":   st,
			"endTime":     et,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// GetModeratorsListMRA returns moderators for a project matching legacy getModeratorsList query.
func (r *UserRepo) GetModeratorsListMRA(ctx context.Context, projectID int64) ([]map[string]any, error) {
q := `SELECT DISTINCT user.first_name AS firstName, user.last_name AS lastName, user.id AS id,
user_client.client_id AS clientId,
COUNT(DISTINCT moderator_time_slot.id) AS interviewCount,
COALESCE(mtr.start_time, '') AS startTime,
COALESCE(mtr.end_time, '') AS endTime,
COALESCE(mtr.timezone, '') AS timezone
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
return nil, fmt.Errorf("get moderators list mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var id, clientID int64
var interviewCount int
var firstName, lastName, startTime, endTime, timezone string
if err := rows.Scan(&firstName, &lastName, &id, &clientID, &interviewCount, &startTime, &endTime, &timezone); err != nil {
return nil, fmt.Errorf("scan moderator list mra: %w", err)
}
records = append(records, map[string]any{
"firstName": firstName, "lastName": lastName, "id": id,
"clientId": clientID, "interviewCount": interviewCount,
"startTime": startTime, "endTime": endTime, "timezone": timezone,
})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}

// GetAllProjectManagersListMRA returns project managers matching legacy SQL (client_id=1 hardcoded in legacy).
func (r *UserRepo) GetAllProjectManagersListMRA(ctx context.Context) ([]map[string]any, error) {
q := `SELECT DISTINCT user.first_name AS firstName, user.last_name AS lastName, user.id AS id
FROM user
INNER JOIN user_client ON user.id = user_client.user_id
INNER JOIN user_role ON user.id = user_role.user_id
WHERE user_client.client_id = 1 AND user_role.role_id IN (2, 3) AND user.deleted = 0
GROUP BY user.id
ORDER BY firstName, lastName ASC`
rows, err := r.db.QueryContext(ctx, q)
if err != nil {
return nil, fmt.Errorf("get all project managers list mra: %w", err)
}
defer rows.Close()
var records []map[string]any
for rows.Next() {
var id int64
var firstName, lastName string
if err := rows.Scan(&firstName, &lastName, &id); err != nil {
return nil, err
}
records = append(records, map[string]any{"firstName": firstName, "lastName": lastName, "id": id})
}
if records == nil {
records = []map[string]any{}
}
return records, rows.Err()
}

// ──────────────────────────────────────────────
// MRA #56 — PM Availabilities
// ──────────────────────────────────────────────

const pmAvailBaseQueryMRA = `SELECT moderator_availability.id, moderator_id AS moderatorId,
client_id AS clientId, start_time AS startTime, end_time AS endTime,
first_name AS firstName, last_name AS lastName
FROM moderator_availability
INNER JOIN user ON user.id = moderator_id
WHERE client_id = ?`

func (r *UserRepo) scanPMAvailRows(rows *sql.Rows) ([]map[string]any, error) {
	defer rows.Close()
	var records []map[string]any
	for rows.Next() {
		var id, modID, cID int64
		var st, et, firstName, lastName string
		if err := rows.Scan(&id, &modID, &cID, &st, &et, &firstName, &lastName); err != nil {
			return nil, fmt.Errorf("scan pm avail mra: %w", err)
		}
		records = append(records, map[string]any{
			"id": id, "moderatorId": modID, "clientId": cID,
			"startTime": st, "endTime": et,
			"firstName": firstName, "lastName": lastName,
		})
	}
	if records == nil {
		records = []map[string]any{}
	}
	return records, rows.Err()
}

// GetAllModeratorsAvailabilityForPMMRA returns all moderator availabilities for a client.
func (r *UserRepo) GetAllModeratorsAvailabilityForPMMRA(ctx context.Context, clientID int64) ([]map[string]any, error) {
	rows, err := r.db.QueryContext(ctx, pmAvailBaseQueryMRA, clientID)
	if err != nil {
		return nil, fmt.Errorf("get all moderators availability for pm mra: %w", err)
	}
	return r.scanPMAvailRows(rows)
}

// GetAllModeratorsAvailabilityForPMWithProjectFilterMRA returns availabilities excluding specified projects.
func (r *UserRepo) GetAllModeratorsAvailabilityForPMWithProjectFilterMRA(ctx context.Context, clientID int64, projectIDs []int64) ([]map[string]any, error) {
	if len(projectIDs) == 0 {
		return r.GetAllModeratorsAvailabilityForPMMRA(ctx, clientID)
	}
	placeholders := make([]string, len(projectIDs))
	args := []any{clientID}
	for i, id := range projectIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `SELECT DISTINCT moderator_availability.id, moderator_id AS moderatorId,
client_id AS clientId, start_time AS startTime, end_time AS endTime,
first_name AS firstName, last_name AS lastName
FROM moderator_availability
INNER JOIN user ON user.id = moderator_id
INNER JOIN projects_users pu ON pu.user_id = moderator_id
WHERE client_id = ? AND pu.project_id NOT IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get moderators availability with project filter mra: %w", err)
	}
	return r.scanPMAvailRows(rows)
}

// GetAllModeratorsAvailabilityForPMWithModeratorFilterMRA returns availabilities excluding specified moderators.
func (r *UserRepo) GetAllModeratorsAvailabilityForPMWithModeratorFilterMRA(ctx context.Context, clientID int64, moderatorIDs []int64) ([]map[string]any, error) {
	if len(moderatorIDs) == 0 {
		return r.GetAllModeratorsAvailabilityForPMMRA(ctx, clientID)
	}
	placeholders := make([]string, len(moderatorIDs))
	args := []any{clientID}
	for i, id := range moderatorIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := pmAvailBaseQueryMRA + " AND moderator_id NOT IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get moderators availability with moderator filter mra: %w", err)
	}
	return r.scanPMAvailRows(rows)
}

// UpdateModExternalCalendarUrlMRA updates the external calendar URL and key for a moderator.
func (r *UserRepo) UpdateModExternalCalendarUrlMRA(ctx context.Context, moderatorID int64, url, key string) error {
const q = `UPDATE moderator_external_calendar SET external_calendar_url = ?, external_calendar_key = ? WHERE moderator_id = ?`
_, err := r.db.ExecContext(ctx, q, url, key, moderatorID)
if err != nil {
return fmt.Errorf("update external calendar url mra: %w", err)
}
return nil
}

// UpdateModExternalCalendarStatusMRA updates the external calendar import status for a moderator.
func (r *UserRepo) UpdateModExternalCalendarStatusMRA(ctx context.Context, moderatorID int64, status string) error {
const q = `UPDATE moderator_external_calendar SET status = ? WHERE moderator_id = ?`
_, err := r.db.ExecContext(ctx, q, status, moderatorID)
if err != nil {
return fmt.Errorf("update external calendar status mra: %w", err)
}
return nil
}

// DeleteImportedModeratorAvailabilityByModeratorMRA deletes imported avails for a moderator+client.
func (r *UserRepo) DeleteImportedModeratorAvailabilityByModeratorMRA(ctx context.Context, moderatorID, clientID int64) error {
const q = `DELETE FROM imported_moderator_availability WHERE moderator_id = ? AND client_id = ?`
_, err := r.db.ExecContext(ctx, q, moderatorID, clientID)
if err != nil {
return fmt.Errorf("delete imported moderator availability mra: %w", err)
}
return nil
}

// GetImportedModeratorAvailabilityMRA returns imported moderator availability for a moderator+client.
func (r *UserRepo) GetImportedModeratorAvailabilityMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
const q = `SELECT id, moderator_id AS moderatorId, client_id AS clientId,
start_time AS startTime, end_time AS endTime
FROM imported_moderator_availability
WHERE moderator_id = ? AND client_id = ?
ORDER BY start_time`
rows, err := r.db.QueryContext(ctx, q, moderatorID, clientID)
if err != nil {
return nil, fmt.Errorf("get imported moderator availability mra: %w", err)
}
defer rows.Close()

var result []map[string]any
for rows.Next() {
var id, modID, cID int64
var startTime, endTime string
if err := rows.Scan(&id, &modID, &cID, &startTime, &endTime); err != nil {
return nil, fmt.Errorf("scan imported moderator availability: %w", err)
}
result = append(result, map[string]any{
"id":          id,
"moderatorId": modID,
"clientId":    cID,
"startTime":   startTime,
"endTime":     endTime,
})
}
if result == nil {
result = []map[string]any{}
}
return result, rows.Err()
}

// GetModeratorExternalCalendarMRA returns all rows from moderator_external_calendar for a moderator.
// Legacy: SELECT * FROM moderator_external_calendar WHERE moderator_id = ?
func (r *UserRepo) GetModeratorExternalCalendarMRA(ctx context.Context, moderatorID int64) ([]map[string]any, error) {
	const q = `SELECT * FROM moderator_external_calendar WHERE moderator_id = ?`
	rows, err := r.db.QueryContext(ctx, q, moderatorID)
	if err != nil {
		return nil, fmt.Errorf("get moderator external calendar mra: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan moderator external calendar: %w", err)
		}
		rec := make(map[string]any, len(cols))
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				rec[col] = string(b)
			} else {
				rec[col] = val
			}
		}
		result = append(result, rec)
	}
	if result == nil {
		result = []map[string]any{}
	}
	return result, rows.Err()
}

// GetRunningImportProcessCountMRA counts running import processes across all moderators.
// Legacy: SELECT count(*) as RunningProcesses FROM moderator_external_calendar WHERE status = "In Progress"
// Note: Legacy does NOT filter by moderator_id despite receiving it.
func (r *UserRepo) GetRunningImportProcessCountMRA(ctx context.Context) (int64, error) {
	const q = `SELECT COUNT(*) AS RunningProcesses FROM moderator_external_calendar WHERE status = 'In Progress'`
	var count int64
	err := r.db.QueryRowContext(ctx, q).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get running import process count mra: %w", err)
	}
	return count, nil
}

// DeleteExternalCalStatusMRA deletes the external calendar status for a moderator.
// Legacy: DELETE FROM moderator_external_calendar WHERE moderator_id = ?
func (r *UserRepo) DeleteExternalCalStatusMRA(ctx context.Context, moderatorID int64) error {
	const q = `DELETE FROM moderator_external_calendar WHERE moderator_id = ?`
	_, err := r.db.ExecContext(ctx, q, moderatorID)
	if err != nil {
		return fmt.Errorf("delete external cal status mra: %w", err)
	}
	return nil
}

// GetImportedModeratorAvailabilityListMRA returns imported avails with user info for a moderator+client.
func (r *UserRepo) GetImportedModeratorAvailabilityListMRA(ctx context.Context, moderatorID, clientID int64) ([]map[string]any, error) {
const q = `SELECT ma.id, moderator_id AS moderatorId, first_name AS firstName, last_name AS lastName,
client_id AS clientId, start_time AS startTime, end_time AS endTime
FROM imported_moderator_availability ma
INNER JOIN user ON user.id = ma.moderator_id
WHERE moderator_id = ? AND client_id = ?`
rows, err := r.db.QueryContext(ctx, q, moderatorID, clientID)
if err != nil {
return nil, fmt.Errorf("get imported moderator availability list mra: %w", err)
}
defer rows.Close()
var result []map[string]any
for rows.Next() {
var id, modID int64
var firstName, lastName string
var cID int64
var startTime, endTime string
if err := rows.Scan(&id, &modID, &firstName, &lastName, &cID, &startTime, &endTime); err != nil {
return nil, err
}
result = append(result, map[string]any{
"id": id, "moderatorId": modID, "firstName": firstName, "lastName": lastName,
"clientId": cID, "startTime": startTime, "endTime": endTime,
})
}
if result == nil {
result = []map[string]any{}
}
return result, rows.Err()
}

func (r *UserRepo) GetOverlappingManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) ([]map[string]any, error) {
const q = `SELECT ima.id, moderator_id AS moderatorId, first_name AS firstName, last_name AS lastName,
client_id AS clientId, start_time AS startTime, end_time AS endTime
FROM moderator_availability ima
INNER JOIN user ON user.id = ima.moderator_id
WHERE moderator_id = ? AND client_id = ?
AND ((start_time >= ? AND start_time < ?)
OR (end_time > ? AND end_time <= ?)
OR (start_time < ? AND end_time > ?))
ORDER BY start_time`
rows, err := r.db.QueryContext(ctx, q, moderatorID, clientID,
startTime, endTime, startTime, endTime, startTime, endTime)
if err != nil {
return nil, fmt.Errorf("get overlapping manual availability mra: %w", err)
}
defer rows.Close()
var result []map[string]any
for rows.Next() {
var id, modID int64
var firstName, lastName string
var cID int64
var st, et string
if err := rows.Scan(&id, &modID, &firstName, &lastName, &cID, &st, &et); err != nil {
return nil, err
}
result = append(result, map[string]any{
"id": id, "moderatorId": modID, "firstName": firstName, "lastName": lastName,
"clientId": cID, "startTime": st, "endTime": et,
})
}
if result == nil {
result = []map[string]any{}
}
return result, rows.Err()
}


func (r *UserRepo) DeleteImportedAvailByModAndIdMRA(ctx context.Context, moderatorID, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM imported_moderator_availability WHERE moderator_id = ? AND id = ?`, moderatorID, id)
	if err != nil {
		return fmt.Errorf("delete imported avail by mod and id mra: %w", err)
	}
	return nil
}
func (r *UserRepo) DeleteManualAvailByModAndIdMRA(ctx context.Context, moderatorID, id int64) error {
_, err := r.db.ExecContext(ctx, `DELETE FROM moderator_availability WHERE moderator_id = ? AND id = ?`, moderatorID, id)
if err != nil {
return fmt.Errorf("delete manual moderator availability by id mra: %w", err)
}
return nil
}

func (r *UserRepo) AddManualAvailabilityMRA(ctx context.Context, moderatorID, clientID int64, startTime, endTime string) error {
_, err := r.db.ExecContext(ctx,
`INSERT INTO moderator_availability (moderator_id, client_id, start_time, end_time) VALUES (?, ?, ?, ?)`,
moderatorID, clientID, startTime, endTime)
if err != nil {
return fmt.Errorf("add manual availability mra: %w", err)
}
return nil
}
