package availabilitytemplates

import (
	"context"
	"database/sql"
	"fmt"
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
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) Create(ctx context.Context, t *Template) error {
	query := `INSERT INTO availability_templates (uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := repo.db.ExecContext(ctx, query, t.UUID, t.OrganizerUuid, t.Weekday, t.StartTime, t.EndTime, t.PriceUuid, t.EffectiveFrom, t.EffectiveUntil, t.Active)
	return err
}

func (repo *repository) Update(ctx context.Context, t *Template) error {
	query := `UPDATE availability_templates SET weekday=?, start_time=?, end_time=?, price_uuid=?, effective_from=?, effective_until=?, active=? WHERE uuid=?`
	_, err := repo.db.ExecContext(ctx, query, t.Weekday, t.StartTime, t.EndTime, t.PriceUuid, t.EffectiveFrom, t.EffectiveUntil, t.Active, t.UUID)
	return err
}

func (repo *repository) GetByUUID(ctx context.Context, uuid string) (*Template, error) {
	query := `SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at FROM availability_templates WHERE uuid = ?`
	row := repo.db.QueryRowContext(ctx, query, uuid)
	return scanTemplate(row)
}

func (repo *repository) Delete(ctx context.Context, uuid string) error {
	query := `UPDATE availability_templates SET active = 0 WHERE uuid = ?`
	_, err := repo.db.ExecContext(ctx, query, uuid)
	return err
}

func (repo *repository) ListActiveByOrganizer(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
	query := `SELECT uuid, organizer_uuid, weekday, start_time, end_time, price_uuid, effective_from, effective_until, active, created_at, updated_at
		FROM availability_templates
		WHERE organizer_uuid = ? AND active = 1 AND effective_from <= ? AND (effective_until IS NULL OR effective_until >= ?)`
	rows, err := repo.db.QueryContext(ctx, query, organizerUuid, to, from)
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
	err := repo.db.QueryRowContext(ctx, query, templateUuid, occurrenceDate).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (repo *repository) RecordOccurrence(ctx context.Context, templateUuid, occurrenceDate string, status OccurrenceStatus, meetUuid *string) error {
	query := `INSERT INTO availability_template_occurrences (template_uuid, occurrence_date, status, meet_uuid) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE status = VALUES(status), meet_uuid = VALUES(meet_uuid)`
	_, err := repo.db.ExecContext(ctx, query, templateUuid, occurrenceDate, string(status), meetUuid)
	return err
}
