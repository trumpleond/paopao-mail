package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"paopao-api/internal/model"
)

func nowUTC() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

var (
	ErrNotFound      = errors.New("not found")
	ErrNoAvailable   = errors.New("no available account for platform")
	ErrInvalidInput  = errors.New("invalid input")
	ErrAlreadyExists = errors.New("already exists")
)

// AccountStore persists accounts and platform marks.
type AccountStore struct {
	db *sqlx.DB
}

func NewAccountStore(db *sqlx.DB) *AccountStore {
	return &AccountStore{db: db}
}

// ParseCredentialLine parses "email----password". Returns empty if invalid.
func ParseCredentialLine(line string) (email, password string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	parts := strings.SplitN(line, "----", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	email = strings.TrimSpace(parts[0])
	password = strings.TrimSpace(parts[1])
	if email == "" || password == "" || !strings.Contains(email, "@") {
		return "", "", false
	}
	return email, password, true
}

// ImportLines bulk-inserts credentials. overwrite updates password when email exists.
func (s *AccountStore) ImportLines(text string, overwrite bool) (model.ImportResult, error) {
	var res model.ImportResult
	lines := strings.Split(text, "\n")
	now := nowUTC()

	tx, err := s.db.Beginx()
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	insertStmt, err := tx.Preparex(`
		INSERT INTO accounts (email, password, status, note, created_at, updated_at)
		VALUES (?, ?, 'active', '', ?, ?)
	`)
	if err != nil {
		return res, err
	}
	defer insertStmt.Close()

	updateStmt, err := tx.Preparex(`
		UPDATE accounts SET password = ?, updated_at = ? WHERE email = ?
	`)
	if err != nil {
		return res, err
	}
	defer updateStmt.Close()

	existsStmt, err := tx.Preparex(`SELECT id FROM accounts WHERE email = ?`)
	if err != nil {
		return res, err
	}
	defer existsStmt.Close()

	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		res.Total++
		email, password, ok := ParseCredentialLine(line)
		if !ok {
			res.Invalid++
			continue
		}

		var id int64
		err := existsStmt.Get(&id, email)
		if err == nil {
			if overwrite {
				if _, err := updateStmt.Exec(password, now, email); err != nil {
					return res, err
				}
				res.Updated++
			} else {
				res.Skipped++
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return res, err
		}

		if _, err := insertStmt.Exec(email, password, now, now); err != nil {
			// unique race
			if strings.Contains(err.Error(), "UNIQUE") {
				res.Skipped++
				continue
			}
			return res, err
		}
		res.Inserted++
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// PickRandom returns one active account not marked for platform.
func (s *AccountStore) PickRandom(platform string) (*model.PickResult, error) {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return nil, fmt.Errorf("%w: platform required", ErrInvalidInput)
	}

	var a model.Account
	err := s.db.Get(&a, `
		SELECT a.id, a.email, a.password, a.status, a.note, a.created_at, a.updated_at
		FROM accounts a
		WHERE a.status = 'active'
		  AND NOT EXISTS (
		    SELECT 1 FROM platform_marks m
		    WHERE m.account_id = a.id AND m.platform = ?
		  )
		ORDER BY RANDOM()
		LIMIT 1
	`, platform)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoAvailable
	}
	if err != nil {
		return nil, err
	}
	return &model.PickResult{
		ID:         a.ID,
		Email:      a.Email,
		Password:   a.Password,
		Credential: a.Credential(),
	}, nil
}

// Mark records platform usage for account id. Idempotent.
func (s *AccountStore) Mark(accountID int64, platform string) error {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return fmt.Errorf("%w: platform required", ErrInvalidInput)
	}
	if accountID <= 0 {
		return fmt.Errorf("%w: account id required", ErrInvalidInput)
	}

	var exists int
	if err := s.db.Get(&exists, `SELECT 1 FROM accounts WHERE id = ?`, accountID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	_, err := s.db.Exec(`
		INSERT INTO platform_marks (account_id, platform, marked_at)
		VALUES (?, ?, ?)
		ON CONFLICT(account_id, platform) DO NOTHING
	`, accountID, platform, nowUTC())
	return err
}

// MarkByEmail marks by email address.
func (s *AccountStore) MarkByEmail(email, platform string) error {
	email = strings.TrimSpace(email)
	var id int64
	err := s.db.Get(&id, `SELECT id FROM accounts WHERE email = ?`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.Mark(id, platform)
}

// Unmark removes a platform mark.
func (s *AccountStore) Unmark(accountID int64, platform string) error {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return fmt.Errorf("%w: platform required", ErrInvalidInput)
	}
	res, err := s.db.Exec(`
		DELETE FROM platform_marks WHERE account_id = ? AND platform = ?
	`, accountID, platform)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// still ok if account missing or mark missing — check account
		var exists int
		if err := s.db.Get(&exists, `SELECT 1 FROM accounts WHERE id = ?`, accountID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}
	return nil
}

// GetByID returns account with platforms.
func (s *AccountStore) GetByID(id int64) (*model.AccountDetail, error) {
	var a model.Account
	err := s.db.Get(&a, `
		SELECT id, email, password, status, note, created_at, updated_at
		FROM accounts WHERE id = ?
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	platforms, err := s.listPlatforms(id)
	if err != nil {
		return nil, err
	}
	return &model.AccountDetail{Account: a, Platforms: platforms}, nil
}

// GetByEmail returns account by email.
func (s *AccountStore) GetByEmail(email string) (*model.Account, error) {
	var a model.Account
	err := s.db.Get(&a, `
		SELECT id, email, password, status, note, created_at, updated_at
		FROM accounts WHERE email = ?
	`, strings.TrimSpace(email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *AccountStore) listPlatforms(accountID int64) ([]string, error) {
	var platforms []string
	err := s.db.Select(&platforms, `
		SELECT platform FROM platform_marks WHERE account_id = ? ORDER BY platform
	`, accountID)
	if err != nil {
		return nil, err
	}
	if platforms == nil {
		platforms = []string{}
	}
	return platforms, nil
}

// List filter options.
type ListFilter struct {
	Platform string // if set with Unused, filter by platform mark
	Unused   bool   // only accounts not marked for Platform
	Status   string // active/disabled/empty=all
	Offset   int
	Limit    int
}

// List returns paginated accounts.
func (s *AccountStore) List(f ListFilter) ([]model.Account, int, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	where := []string{"1=1"}
	args := []any{}

	if f.Status != "" {
		where = append(where, "a.status = ?")
		args = append(args, f.Status)
	}
	if f.Platform != "" {
		if f.Unused {
			where = append(where, `NOT EXISTS (
				SELECT 1 FROM platform_marks m
				WHERE m.account_id = a.id AND m.platform = ?
			)`)
		} else {
			where = append(where, `EXISTS (
				SELECT 1 FROM platform_marks m
				WHERE m.account_id = a.id AND m.platform = ?
			)`)
		}
		args = append(args, f.Platform)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := `SELECT COUNT(*) FROM accounts a WHERE ` + whereSQL
	if err := s.db.Get(&total, countSQL, args...); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT a.id, a.email, a.password, a.status, a.note, a.created_at, a.updated_at
		FROM accounts a
		WHERE ` + whereSQL + `
		ORDER BY a.id DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, f.Limit, f.Offset)
	var list []model.Account
	if err := s.db.Select(&list, query, args...); err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []model.Account{}
	}
	return list, total, nil
}

// Disable sets status=disabled and optional note. Idempotent if already disabled.
func (s *AccountStore) Disable(id int64, note string) error {
	if id <= 0 {
		return fmt.Errorf("%w: account id required", ErrInvalidInput)
	}
	cur, err := s.GetByID(id)
	if err != nil {
		return err
	}
	newNote := cur.Note
	if strings.TrimSpace(note) != "" {
		newNote = strings.TrimSpace(note)
	}
	if cur.Status == model.StatusDisabled && (note == "" || cur.Note == newNote) {
		return nil
	}
	_, err = s.db.Exec(`
		UPDATE accounts SET status = ?, note = ?, updated_at = ?
		WHERE id = ?
	`, model.StatusDisabled, newNote, nowUTC(), id)
	return err
}

// DisableByEmail disables an account looked up by email address.
func (s *AccountStore) DisableByEmail(email, note string) (int64, error) {
	a, err := s.GetByEmail(email)
	if err != nil {
		return 0, err
	}
	if err := s.Disable(a.ID, note); err != nil {
		return a.ID, err
	}
	return a.ID, nil
}

// Update partial fields.
func (s *AccountStore) Update(id int64, status *string, note *string, password *string) (*model.AccountDetail, error) {
	cur, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	newStatus := cur.Status
	newNote := cur.Note
	newPassword := cur.Password
	if status != nil {
		st := strings.TrimSpace(*status)
		if st != model.StatusActive && st != model.StatusDisabled {
			return nil, fmt.Errorf("%w: status must be active or disabled", ErrInvalidInput)
		}
		newStatus = st
	}
	if note != nil {
		newNote = *note
	}
	if password != nil && strings.TrimSpace(*password) != "" {
		newPassword = strings.TrimSpace(*password)
	}

	_, err = s.db.Exec(`
		UPDATE accounts SET status = ?, note = ?, password = ?, updated_at = ?
		WHERE id = ?
	`, newStatus, newNote, newPassword, nowUTC(), id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

// Delete removes account and marks (FK cascade).
func (s *AccountStore) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// platform_marks: CASCADE may need PRAGMA; also delete manually for safety
	_, _ = s.db.Exec(`DELETE FROM platform_marks WHERE account_id = ?`, id)
	return nil
}

// Stats returns pool overview.
func (s *AccountStore) Stats() (*model.Stats, error) {
	st := &model.Stats{PlatformMarks: map[string]int{}}
	if err := s.db.Get(&st.Total, `SELECT COUNT(*) FROM accounts`); err != nil {
		return nil, err
	}
	if err := s.db.Get(&st.Active, `SELECT COUNT(*) FROM accounts WHERE status = 'active'`); err != nil {
		return nil, err
	}
	if err := s.db.Get(&st.Disabled, `SELECT COUNT(*) FROM accounts WHERE status = 'disabled'`); err != nil {
		return nil, err
	}

	type row struct {
		Platform string `db:"platform"`
		Cnt      int    `db:"cnt"`
	}
	var rows []row
	if err := s.db.Select(&rows, `
		SELECT platform, COUNT(*) AS cnt FROM platform_marks GROUP BY platform ORDER BY platform
	`); err != nil {
		return nil, err
	}
	for _, r := range rows {
		st.PlatformMarks[r.Platform] = r.Cnt
	}
	return st, nil
}
