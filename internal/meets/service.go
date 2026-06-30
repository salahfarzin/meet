package meets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type DateSlot struct {
	Title string
	Times []TimeSlot
}
type TimeSlot struct {
	Uuid     string
	Start    string
	End      string
	Duration string
}

type Service interface {
	Create(ctx context.Context, meet *Meet) (*Meet, error)
	Update(ctx context.Context, meet *Meet) (*Meet, error)
	GetByID(ctx context.Context, id string) (*Meet, error)
	GetByUUID(ctx context.Context, uuid string) (*Meet, error)
	QueryMeets(ctx context.Context, opts *MeetQueryOptions) ([]*Meet, error)
	GetAvailability(ctx context.Context, organizerId string, from, to time.Time, priceUUID *string) (map[string]DateSlot, error)
	ParseStartAndEndTimes(start, end string) (time.Time, time.Time, error)
	Delete(ctx context.Context, uuid string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// GetByID implements Service.
func (s *service) GetByID(ctx context.Context, id string) (*Meet, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByUUID implements Service.
func (s *service) GetByUUID(ctx context.Context, meetUUID string) (*Meet, error) {
	return s.repo.GetByUUID(ctx, meetUUID)
}

func (s *service) Create(ctx context.Context, meet *Meet) (*Meet, error) {
	// Check for conflicts for this organizer and period
	hasConflict, err := s.repo.HasConflict(ctx, meet.OrganizerUuid, meet.Start, meet.End)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.New("appointment conflict for this organizer and period")
	}

	meet.UUID = uuid.New().String()
	// Type, OldPrice, Discount, Price are already set in meet
	if err := s.repo.Create(ctx, meet); err != nil {
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
	hasConflict, err := s.repo.HasConflict(ctx, meet.OrganizerUuid, meet.Start, meet.End, meet.UUID)
	if err != nil {
		return nil, err
	}
	if hasConflict {
		return nil, errors.New("appointment conflict for this organizer and period")
	}

	// Type, OldPrice, Discount, Price are already set in meet
	if err := s.repo.Update(ctx, meet); err != nil {
		return nil, err
	}
	return meet, nil
}

// ParseStartAndEndTimes parses start and end time strings in RFC3339 format and returns time.Time values or an error.
func (s *service) ParseStartAndEndTimes(start, end string) (startTime, endTime time.Time, err error) {
	startTime, err = time.Parse(time.RFC3339, start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start time format")
	}
	endTime, err = time.Parse(time.RFC3339, end)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end time format")
	}
	return startTime, endTime, nil
}

// QueryMeets implements Service.
func (s *service) QueryMeets(ctx context.Context, opts *MeetQueryOptions) ([]*Meet, error) {
	meets, _, err := s.repo.QueryMeets(ctx, opts)
	return meets, err
}

// Delete implements Service.
func (s *service) Delete(ctx context.Context, meetUUID string) error {
	return s.repo.Delete(ctx, meetUUID)
}

// GetAvailability returns available datetimes for a user between from and to, optionally filtered by price_uuid
func (s *service) GetAvailability(ctx context.Context, organizerId string, from, to time.Time, priceUUID *string) (map[string]DateSlot, error) {
	opts := &MeetQueryOptions{
		OrganizerUuid: organizerId,
		From:          &from,
		To:            &to,
		OnlyAvailable: func(b bool) *bool { return &b }(true),
		PriceUuid:     priceUUID,
	}
	meets, _, err := s.repo.QueryMeets(ctx, opts)
	if err != nil {
		return nil, err
	}
	dates := make(map[string]DateSlot)
	for _, m := range meets {
		date := m.Start.Format("2006-01-02")
		startStr := m.Start.Format("15:04")
		endStr := m.End.Format("15:04")
		duration := m.End.Sub(m.Start)
		slot := TimeSlot{
			Uuid:     m.UUID,
			Start:    startStr,
			End:      endStr,
			Duration: fmt.Sprintf("%dm", int(duration.Minutes())),
		}
		ds, exists := dates[date]
		if !exists {
			ds = DateSlot{Title: m.Title}
		}
		ds.Times = append(ds.Times, slot)
		dates[date] = ds
	}
	// Sort slots ascending by start time for each date
	for date := range dates {
		slots := dates[date].Times
		sort.Slice(slots, func(i, j int) bool { return slots[i].Start < slots[j].Start })
		ds := dates[date]
		ds.Times = slots
		dates[date] = ds
	}
	return dates, nil
}
