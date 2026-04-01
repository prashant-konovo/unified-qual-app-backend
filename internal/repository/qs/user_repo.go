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
