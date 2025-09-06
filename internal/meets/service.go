package meets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	Create(ctx context.Context, meet *Meet) (*Meet, error)
	Update(ctx context.Context, meet *Meet) (*Meet, error)
	GetByID(ctx context.Context, id string) (*Meet, error)
	GetAllByOrganizerId(ctx context.Context, organizerId string) ([]*Meet, error)
	ParseStartAndEndTimes(start, end string) (time.Time, time.Time, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// GetAll implements MeetsService.
func (s *service) GetAllByOrganizerId(ctx context.Context, organizerId string) ([]*Meet, error) {
	return s.repo.GetAllByOrganizerId(organizerId)
}

// GetByID implements MeetsService.
func (s *service) GetByID(ctx context.Context, id string) (*Meet, error) {
	return s.repo.GetByID(id)
}

func (s *service) Create(ctx context.Context, meet *Meet) (*Meet, error) {
	// Check for conflicts for this organizer and period
	hasConflict, err := s.repo.HasConflict(meet.OrganizerID, meet.Start, meet.End)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.New("appointment conflict for this organizer and period")
	}

	meet.UUID = uuid.New().String()
	if err := s.repo.Create(meet); err != nil {
		return nil, err
	}
	return meet, nil
}

// Update implements MeetsService.
func (s *service) Update(ctx context.Context, meet *Meet) (*Meet, error) {
	if meet.UUID == "" {
		return nil, errors.New("UUID is required")
	}
	// Check for conflicts for this organizer and period, excluding this meet's UUID
	hasConflict, err := s.repo.HasConflict(meet.OrganizerID, meet.Start, meet.End, meet.UUID)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.New("appointment conflict for this organizer and period")
	}

	if err := s.repo.Update(meet); err != nil {
		return nil, err
	}
	return meet, nil
}

// ParseStartAndEndTimes parses start and end time strings in RFC3339 format and returns time.Time values or an error.
func (s *service) ParseStartAndEndTimes(start, end string) (time.Time, time.Time, error) {
	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time format")
	}
	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time format")
	}
	return startTime, endTime, nil
}
