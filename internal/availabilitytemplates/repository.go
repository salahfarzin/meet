package availabilitytemplates

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Template is a recurring weekly open-hours rule for an organizer, e.g.
// "every Monday 09:00-13:00, starting 2026-08-01, until changed".
type Template struct {
	UUID           string     `db:"uuid"`
	OrganizerUuid  string     `db:"organizer_uuid"`
	Weekday        int32      `db:"weekday"`    // 0=Sunday..6=Saturday
	StartTime      string     `db:"start_time"` // "HH:MM:SS"
	EndTime        string     `db:"end_time"`   // "HH:MM:SS"
	PriceUuid      *string    `db:"price_uuid"`
	EffectiveFrom  time.Time  `db:"effective_from"`
	EffectiveUntil *time.Time `db:"effective_until"`
	Active         bool       `db:"active"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// OccurrenceStatus tracks what happened to a single weekly occurrence of a
// template, so materialization never silently re-creates a slot the trappist
// explicitly deleted.
type OccurrenceStatus string

const (
	OccurrenceMaterialized OccurrenceStatus = "materialized"
	OccurrenceSkipped      OccurrenceStatus = "skipped"
)

type Repository interface {
	Create(ctx context.Context, t *Template) error
	Update(ctx context.Context, t *Template) error
	GetByUUID(ctx context.Context, uuid string) (*Template, error)
	// Delete deactivates the template (soft delete) — existing materialized/booked
	// meets are untouched, only future materialization stops.
	Delete(ctx context.Context, uuid string) error
	// ListActiveByOrganizer returns active templates for organizerUuid whose
	// effective window overlaps [from, to].
	ListActiveByOrganizer(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error)

	// HasOccurrence reports whether templateUuid already has a tracked
	// occurrence (materialized or skipped) on occurrenceDate ("YYYY-MM-DD").
	HasOccurrence(ctx context.Context, templateUuid, occurrenceDate string) (bool, error)
	// RecordOccurrence tracks the outcome of one weekly occurrence.
	RecordOccurrence(ctx context.Context, templateUuid, occurrenceDate string, status OccurrenceStatus, meetUuid *string) error
	// PurgeUnbookedOccurrences deletes templateUuid's materialized meets starting
	// after `after` that nobody has booked, plus their occurrence records, so the
	// next Materialize rebuilds them from the template's current config. Booked
	// meets and explicit skips are kept. Returns the number of meets removed.
	// Runs in the caller's transaction when called inside WithTx.
	PurgeUnbookedOccurrences(ctx context.Context, templateUuid string, after time.Time) (int64, error)

	// WithTx runs fn against a Repository bound to a single transaction: every
	// write fn makes commits together, or none do. Nested calls join the
	// enclosing transaction.
	WithTx(ctx context.Context, fn func(Repository) error) error
}

// dbtx is the query surface shared by *sql.DB and *sql.Tx.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type repository struct {
	db *sql.DB // nil when the repository is bound to a transaction
	q  dbtx
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db, q: db}
}

func (repo *repository) WithTx(ctx context.Context, fn func(Repository) error) error {
	return repo.withTx(ctx, func(r *repository) error { return fn(r) })
}

func (repo *repository) withTx(ctx context.Context, fn func(*repository) error) error {
	if repo.db == nil {
		return fn(repo)
	}
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// No-op once committed; rolls back on error or panic.
	defer func() { _ = tx.Rollback() }()

	if err := fn(&repository{q: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *repository) Create(ctx context.Context, t *Template) error {
	query := `INSERT INTO availability_templates (uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := repo.q.ExecContext(ctx, query, t.UUID, t.OrganizerUuid, t.Weekday, t.StartTime, t.EndTime, t.PriceUuid, t.EffectiveFrom, t.EffectiveUntil, t.Active)
	return err
}

func (repo *repository) Update(ctx context.Context, t *Template) error {
	query := `UPDATE availability_templates SET weekday=?, start_time=?, end_time=?, price_uuid=?, effective_from=?, effective_until=?, active=? WHERE uuid=?`
	_, err := repo.q.ExecContext(ctx, query, t.Weekday, t.StartTime, t.EndTime, t.PriceUuid, t.EffectiveFrom, t.EffectiveUntil, t.Active, t.UUID)
	return err
}

func (repo *repository) GetByUUID(ctx context.Context, uuid string) (*Template, error) {
	query := `SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at FROM availability_templates WHERE uuid = ?`
	row := repo.q.QueryRowContext(ctx, query, uuid)
	return scanTemplate(row)
}

func (repo *repository) Delete(ctx context.Context, uuid string) error {
	query := `UPDATE availability_templates SET active = 0 WHERE uuid = ?`
	_, err := repo.q.ExecContext(ctx, query, uuid)
	return err
}

func (repo *repository) ListActiveByOrganizer(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
	query := `SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at
		FROM availability_templates
		WHERE organizer_uuid = ? AND active = 1 AND effective_from <= ? AND (effective_until IS NULL OR effective_until >= ?)`
	rows, err := repo.q.QueryContext(ctx, query, organizerUuid, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Template, 0)
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTemplate(row rowScanner) (*Template, error) {
	var t Template
	var effectiveUntil sql.NullTime
	err := row.Scan(&t.UUID, &t.OrganizerUuid, &t.Weekday, &t.StartTime, &t.EndTime, &t.PriceUuid, &t.EffectiveFrom, &effectiveUntil, &t.Active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("availability template not found")
		}
		return nil, err
	}
	if effectiveUntil.Valid {
		t.EffectiveUntil = &effectiveUntil.Time
	}
	return &t, nil
}

func (repo *repository) HasOccurrence(ctx context.Context, templateUuid, occurrenceDate string) (bool, error) {
	var count int
	query := `SELECT COUNT(1) FROM availability_template_occurrences WHERE template_uuid = ? AND occurrence_date = ?`
	err := repo.q.QueryRowContext(ctx, query, templateUuid, occurrenceDate).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (repo *repository) RecordOccurrence(ctx context.Context, templateUuid, occurrenceDate string, status OccurrenceStatus, meetUuid *string) error {
	query := `INSERT INTO availability_template_occurrences (template_uuid, occurrence_date, status, meet_uuid) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE status = VALUES(status), meet_uuid = VALUES(meet_uuid)`
	_, err := repo.q.ExecContext(ctx, query, templateUuid, occurrenceDate, string(status), meetUuid)
	return err
}

func (repo *repository) PurgeUnbookedOccurrences(ctx context.Context, templateUuid string, after time.Time) (purged int64, err error) {
	err = repo.withTx(ctx, func(r *repository) error {
		purged, err = r.purgeUnbookedOccurrences(ctx, templateUuid, after)
		return err
	})
	return purged, err
}

// purgeUnbookedOccurrences must run inside a transaction: the FOR UPDATE lock
// only holds until the enclosing transaction ends.
func (repo *repository) purgeUnbookedOccurrences(ctx context.Context, templateUuid string, after time.Time) (int64, error) {
	meetUuids, err := repo.lockUnbookedMeets(ctx, templateUuid, after)
	if err != nil || len(meetUuids) == 0 {
		return 0, err
	}

	in := "?" + strings.Repeat(",?", len(meetUuids)-1)
	if _, err := repo.q.ExecContext(ctx, "DELETE FROM availability_template_occurrences WHERE meet_uuid IN ("+in+")", meetUuids...); err != nil {
		return 0, err
	}
	res, err := repo.q.ExecContext(ctx, "DELETE FROM meets WHERE uuid IN ("+in+")", meetUuids...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// lockUnbookedMeets selects FOR UPDATE so a concurrent booking (meets.Update's
// version CAS) either lands first and is excluded here, or blocks until commit
// and then fails cleanly with "meet not found".
func (repo *repository) lockUnbookedMeets(ctx context.Context, templateUuid string, after time.Time) ([]any, error) {
	rows, err := repo.q.QueryContext(ctx, `SELECT uuid FROM meets
		WHERE template_uuid = ? AND start_time > ? AND booked_at IS NULL AND JSON_LENGTH(participant_uuids) = 0
		FOR UPDATE`, templateUuid, after.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var uuids []any
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		uuids = append(uuids, u)
	}
	return uuids, rows.Err()
}
