package meets

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/salahfarzin/logger"
	"go.uber.org/zap"
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

// ListSchedulingInput holds all filter parameters for the scheduling list.
type ListSchedulingInput struct {
	// AllowedClinics are the organizer UUIDs this caller is allowed to see.
	// An empty slice means the caller has no allowed clinics — return empty immediately.
	// Ignored when Unrestricted is true.
	AllowedClinics []string
	// Unrestricted, when true, bypasses AllowedClinics scoping entirely (every
	// organizer, real clinic or not) - the handler sets this only for callers it has
	// already verified hold RoleSuperAdmin. Clinic (below) still narrows results to
	// one specific organizer even when Unrestricted is set.
	Unrestricted bool
	// Clinic, when non-empty, restricts to a single clinic within AllowedClinics
	// (or any organizer at all, when Unrestricted is set).
	Clinic string
	// ParticipantUuid, when set, restricts results to meets with this participant.
	ParticipantUuid string
	// Time range filter.
	From *time.Time
	To   *time.Time
	// Pagination.
	Page     int
	PageSize int
	// Sort. SortBy is validated against a fixed column allow-list by
	// buildScopedQueryOptions; SortDir is "asc" or "desc".
	SortBy  string
	SortDir string
	// UseCursor selects keyset pagination (QueryMeetsCursor) instead of the
	// Page/PageSize OFFSET path. Cursor is the opaque token from a prior
	// response's NextCursor; empty on the first page.
	UseCursor bool
	Cursor    string
}

// ListSchedulingResult is the paginated result of ListScheduling.
type ListSchedulingResult struct {
	Meets    []*Meet
	Total    int
	Page     int
	PageSize int
	// NextCursor and HasMore are only populated when UseCursor was requested.
	NextCursor string
	HasMore    bool
}

type Service interface {
	Create(ctx context.Context, meet *Meet) (*Meet, error)
	Update(ctx context.Context, meet *Meet) (*Meet, error)
	GetByUUID(ctx context.Context, uuid string) (*Meet, error)
	QueryMeets(ctx context.Context, opts *MeetQueryOptions) ([]*Meet, error)
	GetAvailability(ctx context.Context, organizerId string, from, to time.Time, priceUUID *string) (map[string]DateSlot, error)
	ParseStartAndEndTimes(start, end string) (time.Time, time.Time, error)
	Delete(ctx context.Context, uuid string) error
	ListScheduling(ctx context.Context, in *ListSchedulingInput) (ListSchedulingResult, error)
}

// TemplateMaterializer is the narrow slice of availabilitytemplates.Service
// that GetAvailability/Delete need. Defined here (not imported from
// internal/availabilitytemplates) so meets has no dependency on that package —
// availabilitytemplates depends on meets, not the other way around.
type TemplateMaterializer interface {
	Materialize(ctx context.Context, organizerUuid string, from, to time.Time) error
	RecordSkip(ctx context.Context, templateUuid string, occurrenceDate time.Time) error
}

type service struct {
	repo      Repository
	templates TemplateMaterializer
}

func NewService(repo Repository, templates TemplateMaterializer) Service {
	return &service{repo: repo, templates: templates}
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

	if err := validateBookingRules(ctx, s.repo, meet, ""); err != nil {
		return nil, err
	}

	meetUUID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	meet.UUID = meetUUID.String()
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

	if err := validateBookingRules(ctx, s.repo, meet, meet.UUID); err != nil {
		return nil, err
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

// Delete implements Service. If the meet was materialized from an availability
// template, the occurrence is recorded as explicitly skipped first, so a later
// GetAvailability call for the same organizer/window never re-creates it.
func (s *service) Delete(ctx context.Context, meetUUID string) error {
	if s.templates != nil {
		if m, err := s.repo.GetByUUID(ctx, meetUUID); err == nil && m.TemplateUuid != nil && *m.TemplateUuid != "" {
			if err := s.templates.RecordSkip(ctx, *m.TemplateUuid, m.Start); err != nil {
				logger.FromContext(ctx).Warn("failed to record availability template skip", zap.Error(err), zap.String("meet_uuid", meetUUID))
			}
		}
	}
	return s.repo.Delete(ctx, meetUUID)
}

// GetAvailability returns available datetimes for a user between from and to, optionally filtered by price_uuid
func (s *service) GetAvailability(ctx context.Context, organizerId string, from, to time.Time, priceUUID *string) (map[string]DateSlot, error) {
	if s.templates != nil {
		if err := s.templates.Materialize(ctx, organizerId, from, to); err != nil {
			logger.FromContext(ctx).Warn("failed to materialize availability templates", zap.Error(err), zap.String("organizer_uuid", organizerId))
		}
	}

	opts := &MeetQueryOptions{
		OrganizerUuids: []string{organizerId},
		From:           &from,
		To:             &to,
		OnlyAvailable:  func(b bool) *bool { return &b }(true),
		PriceUuid:      priceUUID,
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

// ListScheduling returns a paginated scheduling list scoped to the allowed clinics.
func (s *service) ListScheduling(ctx context.Context, in *ListSchedulingInput) (ListSchedulingResult, error) {
	empty := ListSchedulingResult{Meets: []*Meet{}}

	// 1. Empty allowed clinics → empty result (caller has no scope), unless the
	// handler already verified this caller is unrestricted (RoleSuperAdmin).
	if !in.Unrestricted && len(in.AllowedClinics) == 0 {
		return empty, nil
	}

	// 2. ParticipantUuid, when set, is an exact-match filter.
	var participantUuids []string
	if in.ParticipantUuid != "" {
		participantUuids = []string{in.ParticipantUuid}
	}

	// 3. Build query options scoped to allowed clinics.
	opts, clinicDenied := buildScopedQueryOptions(in, participantUuids)
	if clinicDenied {
		// FINDING 1 fix: an out-of-scope Clinic is an access violation — return
		// empty without hitting the repo.
		return ListSchedulingResult{Page: in.Page, PageSize: in.PageSize}, nil
	}

	// 4. Query repository — keyset path when requested, OFFSET path otherwise.
	var rows []*Meet
	var total int
	var nextCursor string
	var hasMore bool
	var err error
	if opts.UseCursor {
		rows, total, nextCursor, hasMore, err = s.repo.QueryMeetsCursor(ctx, opts)
	} else {
		rows, total, err = s.repo.QueryMeets(ctx, opts)
	}
	if err != nil {
		return empty, err
	}
	if len(rows) == 0 {
		return ListSchedulingResult{
			Meets: []*Meet{}, Total: total, Page: in.Page, PageSize: in.PageSize,
			NextCursor: nextCursor, HasMore: hasMore,
		}, nil
	}

	return ListSchedulingResult{
		Meets:      rows,
		Total:      total,
		Page:       in.Page,
		PageSize:   in.PageSize,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// buildScopedQueryOptions builds MeetQueryOptions scoped to the caller's allowed
// clinics. clinicDenied is true when in.Clinic is set but not a member of
// in.AllowedClinics, meaning the caller asked for a clinic outside their scope.
func buildScopedQueryOptions(in *ListSchedulingInput, participantUuids []string) (opts *MeetQueryOptions, clinicDenied bool) {
	opts = &MeetQueryOptions{
		OrganizerUuids:   in.AllowedClinics,
		Unrestricted:     in.Unrestricted,
		ParticipantUuids: participantUuids,
		From:             in.From,
		To:               in.To,
		Page:             in.Page,
		PageSize:         in.PageSize,
		SortBy:           in.SortBy,
		SortDir:          in.SortDir,
		UseCursor:        in.UseCursor,
		Cursor:           in.Cursor,
	}
	if in.Clinic != "" {
		// Unrestricted callers (handler already verified RoleSuperAdmin) may narrow
		// to any organizer at all - only non-unrestricted callers are checked
		// against their own AllowedClinics.
		if !in.Unrestricted && !slices.Contains(in.AllowedClinics, in.Clinic) {
			return nil, true
		}
		opts.OrganizerUuids = []string{in.Clinic}
		opts.Unrestricted = false
	}
	return opts, false
}
