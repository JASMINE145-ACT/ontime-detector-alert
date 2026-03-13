package alerts

import (
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type Repository interface {
	Create(alert *Alert) error
	Delete(id string) error
	ListByUser(userID string) ([]Alert, error)
	ListActive() ([]Alert, error)
	UpdateNotificationState(id string, triggeredAt, lastNotifiedAt *time.Time) error
	Close() error
}

type sqliteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(path string) (Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &sqliteRepository{db: db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS alerts (
    id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    direction TEXT NOT NULL,
    threshold REAL NOT NULL,
    user_id TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    triggered_at TIMESTAMP NULL,
    cooldown_seconds INTEGER NOT NULL DEFAULT 0,
    last_notified_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
`
	_, err := db.Exec(schema)
	return err
}

func (r *sqliteRepository) Create(a *Alert) error {
	now := time.Now().UTC()
	if a.ID == "" {
		a.ID = generateID()
	}
	a.CreatedAt = now
	a.UpdatedAt = now
	if _, err := r.db.Exec(
		`INSERT INTO alerts (id, symbol, direction, threshold, user_id, active, triggered_at, cooldown_seconds, last_notified_at, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Symbol, string(a.Direction), a.Threshold, a.UserID, boolToInt(a.Active),
		a.TriggeredAt, a.CooldownSeconds, a.LastNotifiedAt, a.CreatedAt, a.UpdatedAt,
	); err != nil {
		return err
	}
	return nil
}

func (r *sqliteRepository) Delete(id string) error {
	res, err := r.db.Exec(`DELETE FROM alerts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *sqliteRepository) ListByUser(userID string) ([]Alert, error) {
	rows, err := r.db.Query(`SELECT id, symbol, direction, threshold, user_id, active, triggered_at, cooldown_seconds, last_notified_at, created_at, updated_at FROM alerts WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		var direction string
		var activeInt int
		if err := rows.Scan(&a.ID, &a.Symbol, &direction, &a.Threshold, &a.UserID, &activeInt, &a.TriggeredAt, &a.CooldownSeconds, &a.LastNotifiedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Direction = Direction(direction)
		a.Active = activeInt == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *sqliteRepository) ListActive() ([]Alert, error) {
	rows, err := r.db.Query(`SELECT id, symbol, direction, threshold, user_id, active, triggered_at, cooldown_seconds, last_notified_at, created_at, updated_at FROM alerts WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		var direction string
		var activeInt int
		if err := rows.Scan(&a.ID, &a.Symbol, &direction, &a.Threshold, &a.UserID, &activeInt, &a.TriggeredAt, &a.CooldownSeconds, &a.LastNotifiedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Direction = Direction(direction)
		a.Active = activeInt == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *sqliteRepository) UpdateNotificationState(id string, triggeredAt, lastNotifiedAt *time.Time) error {
	if triggeredAt == nil && lastNotifiedAt == nil {
		return errors.New("nothing to update")
	}
	now := time.Now().UTC()
	_, err := r.db.Exec(`
UPDATE alerts
SET triggered_at = COALESCE(?, triggered_at),
    last_notified_at = COALESCE(?, last_notified_at),
    updated_at = ?
WHERE id = ?`,
		triggeredAt, lastNotifiedAt, now, id,
	)
	return err
}

func (r *sqliteRepository) Close() error {
	return r.db.Close()
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

