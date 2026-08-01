package meets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrVersionConflict is returned by Update when the row's version no longer
// matches meet.Version — someone else updated it since the caller last read it.
var ErrVersionConflict = errors.New("meet was modified concurrently")

const sqlAndStartTimeBefore = " AND start_time < ?"

type Meet struct {
	UUID             string     `json:"uuid" db:"uuid"`
	Title            string     `json:"title" db:"title"`
	OrganizerUuid    string     `json:"organizer_uuid" db:"organizer_uuid"`
	CreatorUuid      string     `json:"creator_uuid" db:"creator_uuid"`
	PriceUuid        *string    `json:"price_uuid" db:"price_uuid"`
	Type             int32      `json:"type" db:"type"`
	Start            time.Time  `json:"start_time" db:"start_time"`
	End              time.Time  `json:"end_time" db:"end_time"`
	Description      string     `json:"description" db:"description"`
	Color            string     `json:"color" db:"color"`
	ParticipantUuids []string   `json:"participant_uuids" db:"participant_uuids"`
	BookedAt         *time.Time `json:"booked_at" db:"booked_at"`
	Settings         *string    `json:"settings" db:"settings"`
	// TemplateUuid, when set, marks this meet as materialized from an
	// AvailabilityTemplate occurrence (see internal/availabilitytemplates).
	TemplateUuid *string   `json:"template_uuid,omitempty" db:"template_uuid"`
	Version      int32     `json:"version" db:"version"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	// Display-only fields: populated from identity service, never persisted.
	FirstName    string `json:"first_name,omitempty" db:"-"`
	LastName     string `json:"last_name,omitempty" db:"-"`
	NationalCode string `json:"national_code,omitempty" db:"-"`
	Mobile       string `json:"mobile,omitempty" db:"-"`
	ClinicName   string `json:"clinic_name,omitempty" db:"-"`
}

type Repository interface {
	GenerateAvailableSlots(ctx context.Context, organizerID string, from time.Time, to time.Time, priceUUID *string) ([]*Meet, error)
	Create(ctx context.Context, meet *Meet) error
	GetByUUID(ctx context.Context, uuid string) (*Meet, error)
	Update(ctx context.Context, meet *Meet) error
	Delete(ctx context.Context, uuid string) error
	// QueryMeets: pass nil for no filter; returns rows, total count, error
	QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error)
	HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
	// FindParticipantBookings returns meets where any of participantUuids is a
	// participant, with start_time after from (if set) and before to (if set),
	// excluding excludeUUID. Used by booking-restriction rules (service.go); callers
	// filter for cancelled participants themselves via hasActiveBooking, since that
	// needs the settings JSON this can't filter on in SQL.
	FindParticipantBookings(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error)
}
type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{
		db: db,
	}
}

type MeetQueryOptions struct {
	OrganizerUuids []string
	// Unrestricted, when true, drops the organizer_uuid IN (...) filter entirely
	// (every organizer, real clinic or not) - set only for callers the handler layer
	// has already verified hold RoleSuperAdmin. OrganizerUuids is ignored in this case.
	Unrestricted     bool
	ParticipantUuids []string
	From             *time.Time
	To               *time.Time
	OnlyAvailable    *bool
	PriceUuid        *string
	Page             int
	PageSize         int
}

// HasConflict checks if there is an overlapping appointment for the organizer and period
func (repo *repository) HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
	query := `SELECT COUNT(1) FROM meets WHERE organizer_uuid = ? AND start_time < ? AND end_time > ?`
	args := []any{organizerId, end, start}
	if len(excludeUUID) > 0 && excludeUUID[0] != "" {
		query += " AND uuid != ?"
		args = append(args, excludeUUID[0])
	}
	var count int
	err := repo.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (repo *repository) FindParticipantBookings(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
	if len(participantUuids) == 0 {
		return nil, nil
	}

	query, args, err := buildParticipantBookingsQuery(participantUuids, from, to, excludeUUID)
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanParticipantBookings(rows)
}

// buildParticipantBookingsQuery builds the SQL and bound args for
// FindParticipantBookings: participantUuids is required (checked by the caller),
// from/to/excludeUUID are each optional filters.
func buildParticipantBookingsQuery(participantUuids []string, from, to *time.Time, excludeUUID string) (query string, args []any, err error) {
	participantsJSON, err := json.Marshal(participantUuids)
	if err != nil {
		return "", nil, err
	}

	query = `SELECT uuid, participant_uuids, settings FROM meets WHERE JSON_OVERLAPS(participant_uuids, ?)`
	args = []any{string(participantsJSON)}
	if from != nil {
		query += " AND start_time > ?"
		args = append(args, *from)
	}
	if to != nil {
		query += sqlAndStartTimeBefore
		args = append(args, *to)
	}
	if excludeUUID != "" {
		query += " AND uuid != ?"
		args = append(args, excludeUUID)
	}
	return query, args, nil
}

// scanParticipantBookings scans the rows produced by FindParticipantBookings's query.
func scanParticipantBookings(rows *sql.Rows) ([]*Meet, error) {
	var result []*Meet
	for rows.Next() {
		var m Meet
		var participantsStr string
		var settings sql.NullString
		if err := rows.Scan(&m.UUID, &participantsStr, &settings); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(participantsStr), &m.ParticipantUuids); err != nil {
			return nil, fmt.Errorf("failed to unmarshal participants: %w", err)
		}
		if settings.Valid {
			m.Settings = &settings.String
		}
		result = append(result, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (repo *repository) Create(ctx context.Context, meet *Meet) error {
	participantsJSON, err := json.Marshal(meet.ParticipantUuids)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	// Ensure times are stored in UTC
	startUTC := meet.Start.UTC()
	endUTC := meet.End.UTC()

	meet.Version = 1
	// created_at is set here (rather than left to the column's DB default) so the
	// Meet returned from Create() carries the exact value that was stored, instead
	// of the zero time.Time - handler.Create formats meet.CreatedAt straight into
	// CreateResponse without a re-fetch.
	meet.CreatedAt = time.Now().UTC()
	query := `INSERT INTO meets (uuid, title, organizer_uuid, creator_uuid, participant_uuids, start_time, end_time, description, color, type, price_uuid, booked_at, settings, template_uuid, version, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = repo.db.ExecContext(ctx, query, meet.UUID, meet.Title, meet.OrganizerUuid, meet.CreatorUuid, string(participantsJSON), startUTC, endUTC, meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, meet.Settings, meet.TemplateUuid, meet.Version, meet.CreatedAt)
	return err
}

func (repo *repository) GetByUUID(ctx context.Context, uuid string) (*Meet, error) {
	query := `SELECT uuid, title, organizer_uuid, creator_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at, settings, template_uuid, version, created_at FROM meets WHERE uuid = ?`
	return repo.scanOneMeet(ctx, query, uuid)
}

func (repo *repository) scanOneMeet(ctx context.Context, query, arg string) (*Meet, error) {
	row := repo.db.QueryRowContext(ctx, query, arg)
	var a Meet
	var creatorUuid sql.NullString
	var participantsStr string
	var start, end time.Time
	err := row.Scan(&a.UUID, &a.Title, &a.OrganizerUuid, &creatorUuid, &a.PriceUuid, &participantsStr, &start, &end, &a.Description, &a.Color, &a.Type, &a.BookedAt, &a.Settings, &a.TemplateUuid, &a.Version, &a.CreatedAt)
	a.CreatorUuid = creatorUuid.String
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("meet not found")
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(participantsStr), &a.ParticipantUuids); err != nil {
		return nil, fmt.Errorf("failed to unmarshal participants: %w", err)
	}
	a.Start = start
	a.End = end
	return &a, nil
}

// Update is a compare-and-swap: it only writes if the row's current version
// still matches meet.Version, then advances the version. If another update
// landed first (version moved), 0 rows are affected and ErrVersionConflict is
// returned so the caller can re-read and retry instead of clobbering it.
func (repo *repository) Update(ctx context.Context, meet *Meet) error {
	participantsJSON, err := json.Marshal(meet.ParticipantUuids)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	// Ensure times are stored in UTC
	startUTC := meet.Start.UTC()
	endUTC := meet.End.UTC()

	// creator_uuid is intentionally excluded from this SET list - it's set once on
	// Create and never reassigned.
	query := `UPDATE meets SET title=?, organizer_uuid=?, participant_uuids=?, start_time=?, end_time=?, description=?, color=?, type=?, price_uuid=?, booked_at=?, settings=?, version=version+1 WHERE uuid=? AND version=?`
	res, err := repo.db.ExecContext(ctx, query, meet.Title, meet.OrganizerUuid, string(participantsJSON), startUTC, endUTC, meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, meet.Settings, meet.UUID, meet.Version)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		exists, existsErr := repo.exists(ctx, meet.UUID)
		if existsErr != nil {
			return existsErr
		}
		if !exists {
			return fmt.Errorf("meet not found")
		}
		return ErrVersionConflict
	}

	meet.Version++
	return nil
}

func (repo *repository) exists(ctx context.Context, uuid string) (bool, error) {
	var count int
	err := repo.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM meets WHERE uuid = ?`, uuid).Scan(&count)
	return count > 0, err
}

func (repo *repository) Delete(ctx context.Context, uuid string) error {
	query := `DELETE FROM meets WHERE uuid = ?`
	_, err := repo.db.ExecContext(ctx, query, uuid)
	return err
}

func (repo *repository) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
	// Availability queries always target exactly one organizer - GenerateAvailableSlots
	// has no notion of a multi-clinic union.
	if options != nil && options.OnlyAvailable != nil && *options.OnlyAvailable {
		if len(options.OrganizerUuids) != 1 {
			return nil, 0, fmt.Errorf("exactly one OrganizerUuid is required for availability queries")
		}
		meets, err := repo.handleAvailabilityQuery(ctx, options)
		return meets, len(meets), err
	}

	// Unrestricted (every organizer, no IN (...) filter) or an explicit OrganizerUuids
	// list drive the paginated COUNT + SELECT path.
	if options == nil || (!options.Unrestricted && len(options.OrganizerUuids) == 0) {
		return nil, 0, fmt.Errorf("OrganizerUuids is required")
	}

	return repo.queryMeetsPaginated(ctx, options)
}

// queryMeetsPaginated handles the new OrganizerUuids path with COUNT + paginated SELECT.
func (repo *repository) queryMeetsPaginated(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
	page := options.Page
	pageSize := options.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	whereClause, args, err := buildPaginatedWhereClause(options)
	if err != nil {
		return nil, 0, err
	}

	// COUNT query
	countQuery := "SELECT COUNT(*) FROM meets WHERE " + whereClause
	var total int
	if err := repo.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Paginated SELECT. whereClause is built from fixed fragments with ? placeholders only; no
	// value is ever interpolated into the query string itself.
	selectQuery := "SELECT uuid, organizer_uuid, creator_uuid, price_uuid, participant_uuids, type, title, start_time, end_time, color, description, booked_at, settings, version, created_at FROM meets WHERE " + //nolint:gosec // G202: placeholders only, no interpolated values
		whereClause + " ORDER BY start_time LIMIT ? OFFSET ?"
	selectArgs := make([]any, 0, len(args)+2)
	selectArgs = append(selectArgs, args...)
	selectArgs = append(selectArgs, pageSize, offset)

	rows, err := repo.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result, err := scanPaginatedMeets(rows)
	if err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

// buildPaginatedWhereClause builds the WHERE clause and its bound args for the
// OrganizerUuids (multi-organizer, paginated) query path. Unrestricted skips the
// organizer_uuid filter entirely, so the clause always starts with something -
// "1=1" keeps the AND-chaining below unconditional.
func buildPaginatedWhereClause(options *MeetQueryOptions) (whereClause string, args []any, err error) {
	if options.Unrestricted {
		whereClause = "1=1"
		args = []any{}
	} else {
		inPlaceholders := make([]string, len(options.OrganizerUuids))
		args = make([]any, len(options.OrganizerUuids))
		for i, u := range options.OrganizerUuids {
			inPlaceholders[i] = "?"
			args[i] = u
		}
		whereClause = "organizer_uuid IN (" + strings.Join(inPlaceholders, ",") + ")"
	}

	if options.From != nil {
		whereClause += " AND start_time >= ?"
		args = append(args, *options.From)
	}
	if options.To != nil {
		whereClause += sqlAndStartTimeBefore
		args = append(args, *options.To)
	}
	if len(options.ParticipantUuids) > 0 {
		participantJSON, err := json.Marshal(options.ParticipantUuids)
		if err != nil {
			return "", nil, fmt.Errorf("failed to marshal participant uuids: %w", err)
		}
		whereClause += " AND JSON_OVERLAPS(participant_uuids, ?)"
		args = append(args, string(participantJSON))
	}

	return whereClause, args, nil
}

// scanPaginatedMeets scans the rows produced by the paginated SELECT in queryMeetsPaginated.
func scanPaginatedMeets(rows *sql.Rows) ([]*Meet, error) {
	result := make([]*Meet, 0)
	for rows.Next() {
		var m Meet
		var creatorUuid sql.NullString
		var participantsStr string
		var start, end time.Time
		if err := rows.Scan(&m.UUID, &m.OrganizerUuid, &creatorUuid, &m.PriceUuid, &participantsStr, &m.Type, &m.Title, &start, &end, &m.Color, &m.Description, &m.BookedAt, &m.Settings, &m.Version, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.CreatorUuid = creatorUuid.String
		if err := json.Unmarshal([]byte(participantsStr), &m.ParticipantUuids); err != nil {
			return nil, fmt.Errorf("failed to unmarshal participants: %w", err)
		}
		m.Start = start
		m.End = end
		result = append(result, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// handleAvailabilityQuery handles the availability-specific query logic
func (repo *repository) handleAvailabilityQuery(ctx context.Context, options *MeetQueryOptions) ([]*Meet, error) {
	start := time.Now().UTC()
	end := start.AddDate(0, 0, 6)

	if options.From != nil {
		start = *options.From
	}
	if options.To != nil {
		end = *options.To
	}

	return repo.GenerateAvailableSlots(ctx, options.OrganizerUuids[0], start, end, options.PriceUuid)
}

// GenerateAvailableSlots returns all available slots for an organizer between from and to, optionally filtered by price_uuid
func (repo *repository) GenerateAvailableSlots(ctx context.Context, organizerID string, from, to time.Time, priceUUID *string) ([]*Meet, error) {
	result := make([]*Meet, 0)
	query := `SELECT uuid, title, start_time, end_time FROM meets WHERE organizer_uuid = ? AND start_time BETWEEN ? AND ? AND booked_at IS NULL`
	args := []any{organizerID, from, to}

	if priceUUID != nil && *priceUUID != "" {
		query += " AND price_uuid = ?"
		args = append(args, *priceUUID)
	}

	query += " ORDER BY start_time ASC"

	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var uuid, title string
		var start, end time.Time
		if err := rows.Scan(&uuid, &title, &start, &end); err != nil {
			return nil, err
		}
		result = append(result, &Meet{
			UUID:          uuid,
			Title:         title,
			OrganizerUuid: organizerID,
			Start:         start,
			End:           end,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
