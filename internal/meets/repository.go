package meets

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Meet struct {
	ID               string     `json:"id" db:"id"`
	UUID             string     `json:"uuid" db:"uuid"`
	Title            string     `json:"title" db:"title"`
	OrganizerUuid    string     `json:"organizer_uuid" db:"organizer_uuid"`
	PriceUuid        *string    `json:"price_uuid" db:"price_uuid"`
	Type             int32      `json:"type" db:"type"`
	Start            time.Time  `json:"start_time" db:"start_time"`
	End              time.Time  `json:"end_time" db:"end_time"`
	Description      string     `json:"description" db:"description"`
	Color            string     `json:"color" db:"color"`
	ParticipantUuids []string   `json:"participant_uuids" db:"participant_uuids"`
	BookedAt         *time.Time `json:"booked_at" db:"booked_at"`
}

type Repository interface {
	GenerateAvailableSlots(ctx context.Context, organizerID string, from time.Time, to time.Time, priceUUID *string) ([]*Meet, error)
	Create(ctx context.Context, meet *Meet) error
	GetByID(ctx context.Context, id string) (*Meet, error)
	GetByUUID(ctx context.Context, uuid string) (*Meet, error)
	Update(ctx context.Context, meet *Meet) error
	Delete(ctx context.Context, uuid string) error
	// QueryMeets: pass nil for no filter; returns rows, total count, error
	QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error)
	HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
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
	OrganizerUuid    string
	OrganizerUuids   []string
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

func (repo *repository) Create(ctx context.Context, meet *Meet) error {
	participantsJSON, err := json.Marshal(meet.ParticipantUuids)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	// Ensure times are stored in UTC
	startUTC := meet.Start.UTC()
	endUTC := meet.End.UTC()

	query := `INSERT INTO meets (uuid, title, organizer_uuid, participant_uuids, start_time, end_time, description, color, type, price_uuid, booked_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := repo.db.ExecContext(ctx, query, meet.UUID, meet.Title, meet.OrganizerUuid, string(participantsJSON), startUTC, endUTC, meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		meet.ID = fmt.Sprintf("%d", id)
	}

	return nil
}

func (repo *repository) GetByID(ctx context.Context, id string) (*Meet, error) {
	query := `SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE id = ?`
	row := repo.db.QueryRowContext(ctx, query, id)
	var a Meet
	var participantsStr string
	var start, end time.Time
	err := row.Scan(&a.ID, &a.UUID, &a.Title, &a.OrganizerUuid, &a.PriceUuid, &participantsStr, &start, &end, &a.Description, &a.Color, &a.Type, &a.BookedAt)
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

func (repo *repository) GetByUUID(ctx context.Context, uuid string) (*Meet, error) {
	query := `SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE uuid = ?`
	row := repo.db.QueryRowContext(ctx, query, uuid)
	var a Meet
	var participantsStr string
	var start, end time.Time
	err := row.Scan(&a.ID, &a.UUID, &a.Title, &a.OrganizerUuid, &a.PriceUuid, &participantsStr, &start, &end, &a.Description, &a.Color, &a.Type, &a.BookedAt)
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

func (repo *repository) Update(ctx context.Context, meet *Meet) error {
	participantsJSON, err := json.Marshal(meet.ParticipantUuids)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	// Ensure times are stored in UTC
	startUTC := meet.Start.UTC()
	endUTC := meet.End.UTC()

	query := `UPDATE meets SET title=?, organizer_uuid=?, participant_uuids=?, start_time=?, end_time=?, description=?, color=?, type=?, price_uuid=?, booked_at=? WHERE uuid=?`
	_, err = repo.db.ExecContext(ctx, query, meet.Title, meet.OrganizerUuid, string(participantsJSON), startUTC, endUTC, meet.Description, meet.Color, meet.Type, meet.PriceUuid, meet.BookedAt, meet.UUID)

	return err
}

func (repo *repository) Delete(ctx context.Context, uuid string) error {
	query := `DELETE FROM meets WHERE uuid = ?`
	_, err := repo.db.ExecContext(ctx, query, uuid)
	return err
}

func (repo *repository) QueryMeets(ctx context.Context, options *MeetQueryOptions) ([]*Meet, int, error) {
	// New path: OrganizerUuids (multi-organizer) with pagination
	if options != nil && len(options.OrganizerUuids) > 0 {
		return repo.queryMeetsPaginated(ctx, options)
	}

	if options == nil || options.OrganizerUuid == "" {
		return nil, 0, fmt.Errorf("OrganizerUuid is required")
	}

	// Handle availability query
	if options.OnlyAvailable != nil && *options.OnlyAvailable {
		meets, err := repo.handleAvailabilityQuery(ctx, options)
		return meets, len(meets), err
	}

	// Handle regular query
	query, args := repo.buildQueryAndArgs(options)
	rows, err := repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	meets, err := repo.processRows(rows)
	return meets, len(meets), err
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

	// Build WHERE clause with proper IN placeholders for all organizer UUIDs.
	inPlaceholders := make([]string, len(options.OrganizerUuids))
	args := make([]any, len(options.OrganizerUuids))
	for i, u := range options.OrganizerUuids {
		inPlaceholders[i] = "?"
		args[i] = u
	}
	whereClause := "organizer_uuid IN (" + joinStrings(inPlaceholders, ",") + ")"

	if options.From != nil {
		whereClause += " AND start_time >= ?"
		args = append(args, *options.From)
	}
	if options.To != nil {
		whereClause += " AND start_time < ?"
		args = append(args, *options.To)
	}
	if len(options.ParticipantUuids) > 0 {
		participantJSON, err := json.Marshal(options.ParticipantUuids)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal participant uuids: %w", err)
		}
		whereClause += " AND JSON_OVERLAPS(participant_uuids, ?)"
		args = append(args, string(participantJSON))
	}

	// COUNT query
	countQuery := "SELECT COUNT(*) FROM meets WHERE " + whereClause
	var total int
	if err := repo.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Paginated SELECT
	selectQuery := `SELECT id, uuid, organizer_uuid, price_uuid, participant_uuids, type, title, start_time, end_time, color, description, booked_at FROM meets WHERE ` +
		whereClause + ` ORDER BY start_time LIMIT ? OFFSET ?`
	selectArgs := append(args, pageSize, offset)

	rows, err := repo.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]*Meet, 0)
	for rows.Next() {
		var m Meet
		var participantsStr string
		var start, end time.Time
		if err := rows.Scan(&m.ID, &m.UUID, &m.OrganizerUuid, &m.PriceUuid, &participantsStr, &m.Type, &m.Title, &start, &end, &m.Color, &m.Description, &m.BookedAt); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal([]byte(participantsStr), &m.ParticipantUuids); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal participants: %w", err)
		}
		m.Start = start
		m.End = end
		result = append(result, &m)
	}

	return result, total, nil
}

// joinStrings joins a slice of strings with a separator.
func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// buildQueryAndArgs constructs the SQL query and arguments based on options
func (repo *repository) buildQueryAndArgs(options *MeetQueryOptions) (query string, args []any) {
	query = `SELECT id, uuid, title, organizer_uuid, price_uuid, participant_uuids, start_time, end_time, description, color, type, booked_at FROM meets WHERE organizer_uuid = ?`
	args = []any{options.OrganizerUuid}

	if options.From != nil {
		query += " AND end_time > ?"
		args = append(args, *options.From)
	}
	if options.To != nil {
		query += " AND start_time < ?"
		args = append(args, *options.To)
	}

	return query, args
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

	return repo.GenerateAvailableSlots(ctx, options.OrganizerUuid, start, end, options.PriceUuid)
}

// processRows converts database rows to Meet objects
func (repo *repository) processRows(rows *sql.Rows) ([]*Meet, error) {
	result := make([]*Meet, 0)

	for rows.Next() {
		var a Meet
		var participantsStr string
		var start, end time.Time
		if err := rows.Scan(&a.ID, &a.UUID, &a.Title, &a.OrganizerUuid, &a.PriceUuid, &participantsStr, &start, &end, &a.Description, &a.Color, &a.Type, &a.BookedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(participantsStr), &a.ParticipantUuids); err != nil {
			return nil, fmt.Errorf("failed to unmarshal participants: %w", err)
		}
		a.Start = start
		a.End = end
		result = append(result, &a)
	}

	return result, nil
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
	return result, nil
}
