package model

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Account is a mailbox credential in the pool.
// Timestamps are TEXT (SQLite datetime) to avoid driver scan issues.
type Account struct {
	ID        int64  `db:"id" json:"id"`
	Email     string `db:"email" json:"email"`
	Password  string `db:"password" json:"password"`
	Status    string `db:"status" json:"status"`
	Note      string `db:"note" json:"note"`
	CreatedAt string `db:"created_at" json:"created_at"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
}

// Credential returns email----password for upstream APIs.
func (a Account) Credential() string {
	return a.Email + "----" + a.Password
}

// AccountDetail includes marked platforms.
type AccountDetail struct {
	Account
	Platforms []string `json:"platforms"`
}

// PlatformMark records that an account was used on a platform.
type PlatformMark struct {
	ID        int64  `db:"id" json:"id"`
	AccountID int64  `db:"account_id" json:"account_id"`
	Platform  string `db:"platform" json:"platform"`
	MarkedAt  string `db:"marked_at" json:"marked_at"`
}

// ImportResult is the outcome of a batch import.
type ImportResult struct {
	Total    int `json:"total"`
	Inserted int `json:"inserted"`
	Skipped  int `json:"skipped"`
	Invalid  int `json:"invalid"`
	Updated  int `json:"updated"`
}

// PickResult is returned when picking an account for a platform.
type PickResult struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Credential string `json:"credential"`
}

// Stats is pool overview.
type Stats struct {
	Total         int            `json:"total"`
	Active        int            `json:"active"`
	Disabled      int            `json:"disabled"`
	PlatformMarks map[string]int `json:"platform_marks"`
}
