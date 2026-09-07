package availabilitytemplates

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/salahfarzin/meet/internal/meets"
)

// materializedTitle is the placeholder title given to a Meet row synthesized
// from a template occurrence — same convention as ravanhealth's manually
// created "Available Meet" slots.
const materializedTitle = "Available"

const timeLayout = "15:04:05"
const dateLayout = "2006-01-02"

type Service interface {
	Create(ctx context.Context, t *Template) (*Template, error)
	Update(ctx context.Context, t *Template) (*Template, error)
	GetByUUID(ctx context.Context, uuid string) (*Template, error)
	Delete(ctx context.Context, uuid string) error
	GetAll(ctx context.Context, organizerUuid string) ([]*Template, error)

	// Materialize walks every active template for organizerUuid overlapping
	// [from, to] and creates a real (unbooked) Meet row for every weekly
	// occurrence in that window that hasn't already been materialized or
	// explicitly skipped, and doesn't collide with an existing meet.
	Materialize(ctx context.Context, organizerUuid string, from, to time.Time) error
	// RecordSkip marks one occurrence as explicitly skipped (the trappist
	// deleted that specific materialized meet) so Materialize never
	// re-creates it.
	RecordSkip(ctx context.Context, templateUuid string, occurrenceDate time.Time) error
}

type service struct {
	repo      Repository
	meetsRepo meets.Repository
}

func NewService(repo Repository, meetsRepo meets.Repository) Service {
	return &service{repo: repo, meetsRepo: meetsRepo}
}

func (s *service) Create(ctx context.Context, t *Template) (*Template, error) {
	if err := validate(t); err != nil {
		return nil, err
	}
	if err := s.checkOverlap(ctx, t); err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	t.UUID = id.String()
	t.Active = true
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *service) Update(ctx context.Context, t *Template) (*Template, error) {
	if t.UUID == "" {
		return nil, errors.New("UUID is required")
	}
	if err := validate(t); err != nil {
		return nil, err
	}
	if err := s.checkOverlap(ctx, t); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// checkOverlap rejects a template whose weekday + time-of-day range overlaps
// an existing active template for the same organizer within its effective
// window — e.g. Sun 09:00-13:00 and Sun 09:00-11:00 would otherwise both
// materialize, and GetAvailability silently only ever surfaces one of them.
func (s *service) checkOverlap(ctx context.Context, t *Template) error {
	from := t.EffectiveFrom
	to := from.AddDate(10, 0, 0)
	if t.EffectiveUntil != nil {
		to = *t.EffectiveUntil
	}

	existing, err := s.repo.ListActiveByOrganizer(ctx, t.OrganizerUuid, from, to)
	if err != nil {
		return err
	}

	for _, e := range existing {
		if e.UUID == t.UUID || e.Weekday != t.Weekday {
			continue
		}
		if t.StartTime < e.EndTime && e.StartTime < t.EndTime {
			return fmt.Errorf("overlaps with an existing availability template for this weekday (%s-%s)", e.StartTime[:5], e.EndTime[:5])
		}
	}
	return nil
}

func (s *service) GetByUUID(ctx context.Context, templateUUID string) (*Template, error) {
	return s.repo.GetByUUID(ctx, templateUUID)
}

func (s *service) Delete(ctx context.Context, templateUUID string) error {
	return s.repo.Delete(ctx, templateUUID)
}

func (s *service) GetAll(ctx context.Context, organizerUuid string) ([]*Template, error) {
	// A wide window (10 years back/forward) is enough to surface every
	// template regardless of its effective range for a plain "list mine" call.
	from := time.Now().AddDate(-10, 0, 0)
	to := time.Now().AddDate(10, 0, 0)
	return s.repo.ListActiveByOrganizer(ctx, organizerUuid, from, to)
}

func validate(t *Template) error {
	if t.OrganizerUuid == "" {
		return errors.New("organizer_uuid is required")
	}
	if t.Weekday < 0 || t.Weekday > 6 {
		return errors.New("weekday must be between 0 (Sunday) and 6 (Saturday)")
	}
	if t.StartTime == "" || t.EndTime == "" {
		return errors.New("start_time and end_time are required")
	}
	if t.StartTime >= t.EndTime {
		return errors.New("start_time must be before end_time")
	}
	if t.EffectiveFrom.IsZero() {
		return errors.New("effective_from is required")
	}
	return nil
}

func (s *service) Materialize(ctx context.Context, organizerUuid string, from, to time.Time) error {
	templates, err := s.repo.ListActiveByOrganizer(ctx, organizerUuid, from, to)
	if err != nil {
		return err
	}

	for _, tmpl := range templates {
		if err := s.materializeTemplate(ctx, tmpl, from, to); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) materializeTemplate(ctx context.Context, tmpl *Template, from, to time.Time) error {
	for d := firstOccurrenceOnOrAfter(tmpl.Weekday, from); !d.After(to); d = d.AddDate(0, 0, 7) {
		if d.Before(tmpl.EffectiveFrom) {
			continue
		}
		if tmpl.EffectiveUntil != nil && d.After(*tmpl.EffectiveUntil) {
			break
		}

		if err := s.materializeOccurrence(ctx, tmpl, d); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) materializeOccurrence(ctx context.Context, tmpl *Template, day time.Time) error {
	dateStr := day.Format(dateLayout)

	already, err := s.repo.HasOccurrence(ctx, tmpl.UUID, dateStr)
	if err != nil {
		return err
	}
	if already {
		return nil
	}

	start, err := combineDateAndTime(day, tmpl.StartTime)
	if err != nil {
		return err
	}
	end, err := combineDateAndTime(day, tmpl.EndTime)
	if err != nil {
		return err
	}

	hasConflict, err := s.meetsRepo.HasConflict(ctx, tmpl.OrganizerUuid, start, end)
	if err != nil {
		return err
	}
	if hasConflict {
		// Something already occupies this window (another template occurrence,
		// or a manually created meet). Leave it untracked — not "skipped" — so
		// it's reconsidered on the next call in case the conflict clears.
		return nil
	}

	meetUUID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	meet := &meets.Meet{
		UUID:             meetUUID.String(),
		Title:            materializedTitle,
		OrganizerUuid:    tmpl.OrganizerUuid,
		PriceUuid:        tmpl.PriceUuid,
		Start:            start,
		End:              end,
		ParticipantUuids: []string{},
		TemplateUuid:     &tmpl.UUID,
	}
	if err := s.meetsRepo.Create(ctx, meet); err != nil {
		return err
	}

	return s.repo.RecordOccurrence(ctx, tmpl.UUID, dateStr, OccurrenceMaterialized, &meet.UUID)
}

func (s *service) RecordSkip(ctx context.Context, templateUuid string, occurrenceDate time.Time) error {
	return s.repo.RecordOccurrence(ctx, templateUuid, occurrenceDate.Format(dateLayout), OccurrenceSkipped, nil)
}

// firstOccurrenceOnOrAfter returns the first date on/after `from` (UTC,
// midnight) matching weekday.
func firstOccurrenceOnOrAfter(weekday int32, from time.Time) time.Time {
	d := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	delta := (int(weekday) - int(d.Weekday()) + 7) % 7
	return d.AddDate(0, 0, delta)
}

func combineDateAndTime(day time.Time, hhmmss string) (time.Time, error) {
	t, err := time.Parse(timeLayout, hhmmss)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
}
