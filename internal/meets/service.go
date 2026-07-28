package meets

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/salahfarzin/logger"
	"github.com/salahfarzin/meet/internal/identity"
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
	AllowedClinics []string
	// Clinic, when non-empty, restricts to a single clinic within AllowedClinics.
	Clinic string
	// Identity pre-filter fields — if any are set the identity service is queried first.
	FirstName  string
	LastName   string
	NationalID string
	Mobile     string
	// ParticipantUuid, when set, is an exact-match filter — bypasses the identity Search
	// pre-filter entirely (the caller already knows the UUID; this is not a name/mobile lookup).
	ParticipantUuid string
	// Time range filter.
	From *time.Time
	To   *time.Time
	// Pagination.
	Page     int
	PageSize int
}

// ListSchedulingResult is the paginated result of ListScheduling.
type ListSchedulingResult struct {
	Meets    []*Meet
	Total    int
	Page     int
	PageSize int
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
	ListScheduling(ctx context.Context, in *ListSchedulingInput) (ListSchedulingResult, error)
	// ListClinics returns all clinic UUIDs from the identity service.
	// Returns nil, nil if no identity client is wired (caller must apply a fallback).
	ListClinics(ctx context.Context) ([]identity.Clinic, error)
}

type service struct {
	repo     Repository
	identity identity.Client
}

func NewService(repo Repository, id identity.Client) Service {
	return &service{repo: repo, identity: id}
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

	if err := validateBookingRules(ctx, s.repo, meet, ""); err != nil {
		return nil, err
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

// ListClinics delegates to the identity client. Returns nil, nil when no client is configured.
func (s *service) ListClinics(ctx context.Context) ([]identity.Clinic, error) {
	if s.identity == nil {
		return nil, nil
	}
	return s.identity.ListClinics(ctx)
}

// ListScheduling returns a paginated, identity-enriched scheduling list scoped to the allowed clinics.
func (s *service) ListScheduling(ctx context.Context, in *ListSchedulingInput) (ListSchedulingResult, error) {
	empty := ListSchedulingResult{Meets: []*Meet{}}

	// 1. Empty allowed clinics → empty result (caller has no scope).
	if len(in.AllowedClinics) == 0 {
		return empty, nil
	}

	// 2. Identity pre-filter: if any name/id/mobile filter is set, resolve candidate participant UUIDs.
	participantUuids, noMatch, err := s.resolveParticipantFilter(ctx, in)
	if err != nil {
		return empty, err
	}
	if noMatch {
		return empty, nil
	}

	// 3. Build query options scoped to allowed clinics.
	opts, clinicDenied := buildScopedQueryOptions(in, participantUuids)
	if clinicDenied {
		// FINDING 1 fix: an out-of-scope Clinic is an access violation — return
		// empty without hitting the repo.
		return ListSchedulingResult{Page: in.Page, PageSize: in.PageSize}, nil
	}

	// 4. Query repository.
	rows, total, err := s.repo.QueryMeets(ctx, opts)
	if err != nil {
		return empty, err
	}
	if len(rows) == 0 {
		return ListSchedulingResult{Meets: []*Meet{}, Total: total, Page: in.Page, PageSize: in.PageSize}, nil
	}

	// 5-7. Enrich with identity data (single round-trip for both participants and
	// organizers). If the identity service is unreachable, rows are returned
	// un-enriched rather than failing the whole request — the calendar must
	// remain available even when the identity service is down.
	s.enrichWithIdentity(ctx, rows)

	return ListSchedulingResult{
		Meets:    rows,
		Total:    total,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

// resolveParticipantFilter resolves the candidate participant UUIDs for a scheduling
// query. noMatch signals "no meets can possibly match" (empty result, no error) — an
// exhausted identity search or a nil identity client with an active name/id/mobile filter.
func (s *service) resolveParticipantFilter(ctx context.Context, in *ListSchedulingInput) (participantUuids []string, noMatch bool, err error) {
	if in.ParticipantUuid != "" {
		// Exact-match filter — caller already knows the UUID, so skip the identity Search entirely.
		return []string{in.ParticipantUuid}, false, nil
	}

	needsIdentityFilter := in.FirstName != "" || in.LastName != "" || in.NationalID != "" || in.Mobile != ""
	if !needsIdentityFilter {
		return nil, false, nil
	}

	if s.identity == nil {
		// identity client is nil only in unit tests; production wiring always injects a real client.
		// When nil with an active identity filter, return empty rather than erroring.
		return nil, true, nil
	}

	uuids, err := s.identity.Search(ctx, identity.IdentityFilter{
		FirstName:  in.FirstName,
		LastName:   in.LastName,
		NationalID: in.NationalID,
		Mobile:     in.Mobile,
	})
	if err != nil {
		return nil, false, err
	}
	if len(uuids) == 0 {
		// No matching patients → no meets to show.
		return nil, true, nil
	}
	return uuids, false, nil
}

// buildScopedQueryOptions builds MeetQueryOptions scoped to the caller's allowed
// clinics. clinicDenied is true when in.Clinic is set but not a member of
// in.AllowedClinics, meaning the caller asked for a clinic outside their scope.
func buildScopedQueryOptions(in *ListSchedulingInput, participantUuids []string) (opts *MeetQueryOptions, clinicDenied bool) {
	opts = &MeetQueryOptions{
		OrganizerUuids:   in.AllowedClinics,
		ParticipantUuids: participantUuids,
		From:             in.From,
		To:               in.To,
		Page:             in.Page,
		PageSize:         in.PageSize,
	}
	if in.Clinic != "" {
		if !slices.Contains(in.AllowedClinics, in.Clinic) {
			return nil, true
		}
		opts.OrganizerUuids = []string{in.Clinic}
	}
	return opts, false
}

// enrichWithIdentity attaches participant (patient) and organizer (clinic) identity
// fields to rows in place, via a single identity-service round-trip. If the identity
// service is nil, has nothing to fetch, or is unreachable, rows are left un-enriched.
func (s *service) enrichWithIdentity(ctx context.Context, rows []*Meet) {
	if s.identity == nil {
		return
	}

	uuidsToFetch := collectIdentityUUIDs(rows)
	if len(uuidsToFetch) == 0 {
		return
	}

	identityMap, err := s.identity.GetByUUIDs(ctx, uuidsToFetch)
	if err != nil {
		logger.FromContext(ctx).Warn(
			"identity service unavailable; returning meets without enrichment",
			zap.Error(err),
		)
		return
	}

	for _, m := range rows {
		if len(m.ParticipantUuids) > 0 {
			if id, ok := identityMap[m.ParticipantUuids[0]]; ok {
				m.FirstName = id.FirstName
				m.LastName = id.LastName
				m.NationalCode = id.NationalCode
				m.Mobile = id.Mobile
			}
		}
		if m.OrganizerUuid != "" {
			if org, ok := identityMap[m.OrganizerUuid]; ok {
				name := strings.TrimSpace(org.Name)
				if name == "" {
					name = strings.TrimSpace(org.FirstName + " " + org.LastName)
				}
				m.ClinicName = name
			}
		}
	}
}

// collectIdentityUUIDs gathers the distinct participant and organizer UUIDs across
// rows for a single batched identity lookup.
func collectIdentityUUIDs(rows []*Meet) []string {
	uuidSet := make(map[string]struct{}, len(rows)*2)
	for _, m := range rows {
		if len(m.ParticipantUuids) > 0 {
			uuidSet[m.ParticipantUuids[0]] = struct{}{}
		}
		if m.OrganizerUuid != "" {
			uuidSet[m.OrganizerUuid] = struct{}{}
		}
	}
	uuidsToFetch := make([]string, 0, len(uuidSet))
	for u := range uuidSet {
		uuidsToFetch = append(uuidsToFetch, u)
	}
	return uuidsToFetch
}
