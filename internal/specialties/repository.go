package specialties

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Kind separates what a trappist treats from how they treat it, so both
// catalogues share one table and one admin screen.
type Kind string

const (
	KindSpecialty Kind = "specialty"
	KindApproach  Kind = "approach"
)

// ErrNotFound is returned when no specialty matches the given uuid.
var ErrNotFound = errors.New("specialty not found")

// Specialty is one admin-managed catalogue entry. Names maps a UI language
// code to its label, e.g. {"en": "Anxiety", "fa": "اضطراب"}.
type Specialty struct {
	UUID      string            `db:"uuid"`
	Kind      Kind              `db:"kind"`
	Slug      string            `db:"slug"`
	Names     map[string]string `db:"names"`
	SortOrder int32             `db:"sort_order"`
	Active    bool              `db:"active"`
	CreatedAt time.Time         `db:"created_at"`
	UpdatedAt time.Time         `db:"updated_at"`
}

type Repository interface {
	Create(ctx context.Context, s *Specialty) error
	Update(ctx context.Context, s *Specialty) error
	GetByUUID(ctx context.Context, uuid string) (*Specialty, error)
	Delete(ctx context.Context, uuid string) error
	// List returns entries ordered by kind, sort_order, slug. An empty kind
	// means every kind; inactive entries are left out unless includeInactive.
	List(ctx context.Context, kind Kind, includeInactive bool) ([]*Specialty, error)
	// SlugExists reports whether another entry (not excludeUUID) of the same
	// kind already uses slug.
	SlugExists(ctx context.Context, kind Kind, slug, excludeUUID string) (bool, error)
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

const selectColumns = `SELECT uuid, kind, slug, names, sort_order, active, created_at, updated_at FROM specialties`

func (repo *repository) Create(ctx context.Context, s *Specialty) error {
	names, err := json.Marshal(s.Names)
	if err != nil {
		return err
	}
	query := `INSERT INTO specialties (uuid, kind, slug, names, sort_order, active) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = repo.db.ExecContext(ctx, query, s.UUID, string(s.Kind), s.Slug, names, s.SortOrder, s.Active)
	return err
}

func (repo *repository) Update(ctx context.Context, s *Specialty) error {
	names, err := json.Marshal(s.Names)
	if err != nil {
		return err
	}
	query := `UPDATE specialties SET kind=?, slug=?, names=?, sort_order=?, active=? WHERE uuid=?`
	_, err = repo.db.ExecContext(ctx, query, string(s.Kind), s.Slug, names, s.SortOrder, s.Active, s.UUID)
	return err
}

func (repo *repository) GetByUUID(ctx context.Context, uuid string) (*Specialty, error) {
	row := repo.db.QueryRowContext(ctx, selectColumns+` WHERE uuid = ?`, uuid)
	return scanSpecialty(row)
}

// Delete removes the entry outright. Trappist profiles that still reference
// the uuid simply stop resolving it; use Active=false to hide an entry while
// keeping it on existing profiles.
func (repo *repository) Delete(ctx context.Context, uuid string) error {
	_, err := repo.db.ExecContext(ctx, `DELETE FROM specialties WHERE uuid = ?`, uuid)
	return err
}

func (repo *repository) List(ctx context.Context, kind Kind, includeInactive bool) ([]*Specialty, error) {
	query := selectColumns + ` WHERE (? = '' OR kind = ?) AND (? OR active = 1) ORDER BY kind, sort_order, slug`
	rows, err := repo.db.QueryContext(ctx, query, string(kind), string(kind), includeInactive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]*Specialty, 0)
	for rows.Next() {
		s, err := scanSpecialty(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (repo *repository) SlugExists(ctx context.Context, kind Kind, slug, excludeUUID string) (bool, error) {
	var count int
	query := `SELECT COUNT(1) FROM specialties WHERE kind = ? AND slug = ? AND uuid <> ?`
	if err := repo.db.QueryRowContext(ctx, query, string(kind), slug, excludeUUID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSpecialty(row rowScanner) (*Specialty, error) {
	var s Specialty
	var kind string
	var names []byte
	err := row.Scan(&s.UUID, &kind, &s.Slug, &names, &s.SortOrder, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.Kind = Kind(kind)
	if err := json.Unmarshal(names, &s.Names); err != nil {
		return nil, err
	}
	return &s, nil
}
