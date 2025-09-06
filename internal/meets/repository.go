package meets

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Meet struct {
	ID           string    `json:"id"`
	UUID         string    `json:"uuid"`
	Title        string    `json:"title"`
	OrganizerID  string    `json:"organizer_id"`
	Participants []string  `json:"participants"`
	Start        time.Time `json:"start_time"`
	End          time.Time `json:"end_time"`
	Description  string    `json:"description"`
	Color        string    `json:"color"`
}

type Repository interface {
	Create(meet *Meet) error
	GetByID(id string) (*Meet, error)
	Update(meet *Meet) error
	Delete(id string) error
	GetAllByOrganizerId(organizerId string) ([]*Meet, error)
	HasConflict(organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
}

// HasConflict checks if there is an overlapping appointment for the organizer and period
func (repo *repository) HasConflict(organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
	query := `SELECT COUNT(1) FROM meets WHERE organizer_id = ? AND start_time < ? AND end_time > ?`
	args := []interface{}{organizerId, end, start}
	if len(excludeUUID) > 0 && excludeUUID[0] != "" {
		query += " AND uuid != ?"
		args = append(args, excludeUUID[0])
	}
	var count int
	err := repo.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repository{
		db: db,
	}
}

func (repo *repository) Create(meet *Meet) error {
	participantsJSON, err := json.Marshal(meet.Participants)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	// Ensure times are stored in UTC
	startUTC := meet.Start.UTC()
	endUTC := meet.End.UTC()

	query := `INSERT INTO meets (uuid, title, organizer_id, participants, start_time, end_time, description, color) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := repo.db.Exec(query, meet.UUID, meet.Title, meet.OrganizerID, string(participantsJSON), startUTC, endUTC, meet.Description, meet.Color)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		meet.ID = fmt.Sprintf("%d", id)
	}

	return nil
}

func (repo *repository) GetByID(id string) (*Meet, error) {
	query := `SELECT id, uuid, title, organizer_id, participants, start_time, end_time, description, color FROM meets WHERE id = ?`
	row := repo.db.QueryRow(query, id)
	var a Meet
	var participantsStr string
	var start, end time.Time
	err := row.Scan(&a.ID, &a.UUID, &a.Title, &a.OrganizerID, &participantsStr, &start, &end, &a.Description, &a.Color)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("meet not found")
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(participantsStr), &a.Participants); err != nil {
		return nil, fmt.Errorf("failed to unmarshal participants: %w", err)
	}
	a.Start = start
	a.End = end
	return &a, nil
}

func (repo *repository) Update(meet *Meet) error {
	participantsJSON, err := json.Marshal(meet.Participants)
	if err != nil {
		return fmt.Errorf("failed to marshal participants: %w", err)
	}

	// Ensure times are stored in UTC
	startUTC := meet.Start.UTC()
	endUTC := meet.End.UTC()

	query := `UPDATE meets SET title=?, organizer_id=?, participants=?, start_time=?, end_time=?, description=?, color=? WHERE id=?`
	_, err = repo.db.Exec(query, meet.Title, meet.OrganizerID, string(participantsJSON), startUTC, endUTC, meet.Description, meet.Color, meet.UUID)

	return err
}

func (repo *repository) Delete(id string) error {
	query := `DELETE FROM meets WHERE id = ?`
	_, err := repo.db.Exec(query, id)
	return err
}

func (repo *repository) GetAllByOrganizerId(organizerId string) ([]*Meet, error) {
	query := `SELECT id, uuid, title, organizer_id, participants, start_time, end_time, description, color FROM meets WHERE organizer_id = ?`
	rows, err := repo.db.Query(query, organizerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Meet
	for rows.Next() {
		var a Meet
		var participantsStr string
		var start, end time.Time
		if err := rows.Scan(&a.ID, &a.UUID, &a.Title, &a.OrganizerID, &participantsStr, &start, &end, &a.Description, &a.Color); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(participantsStr), &a.Participants); err != nil {
			return nil, fmt.Errorf("failed to unmarshal participants: %w", err)
		}
		a.Start = start
		a.End = end
		result = append(result, &a)
	}
	return result, nil
}
