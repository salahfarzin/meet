package meets

import (
	"context"
	"errors"
	"time"
)

// bookingRule validates a candidate meet against one booking constraint. Returns
// nil when the rule is disabled in settings or isn't violated. New restrictions
// are added here as another function appended to bookingRules, rather than
// growing an if-block in Create/Update.
type bookingRule func(ctx context.Context, repo Repository, meet *Meet, settings *MeetSettings, excludeUUID string) error

var bookingRules = []bookingRule{
	checkMinHoursBetweenBookings,
	checkPreventMultipleUpcomingBookings,
}

// validateBookingRules runs every bookingRule against meet, returning the first
// violation. excludeUUID is the meet being updated (empty on create) so a meet
// never conflicts with itself.
func validateBookingRules(ctx context.Context, repo Repository, meet *Meet, excludeUUID string) error {
	settings := parseSettings(meet.Settings)
	for _, rule := range bookingRules {
		if err := rule(ctx, repo, meet, &settings, excludeUUID); err != nil {
			return err
		}
	}
	return nil
}

// checkMinHoursBetweenBookings enforces MeetSettings.MinHoursBetweenBookings: no
// participant may have another active booking starting within that many hours of
// this meet's start.
func checkMinHoursBetweenBookings(ctx context.Context, repo Repository, meet *Meet, settings *MeetSettings, excludeUUID string) error {
	if settings.MinHoursBetweenBookings <= 0 || len(meet.ParticipantUuids) == 0 {
		return nil
	}
	from := meet.Start.Add(-time.Duration(settings.MinHoursBetweenBookings) * time.Hour)
	to := meet.Start.Add(time.Duration(settings.MinHoursBetweenBookings) * time.Hour)
	bookings, err := repo.FindParticipantBookings(ctx, meet.ParticipantUuids, &from, &to, excludeUUID)
	if err != nil {
		return err
	}
	if hasActiveConflict(bookings, meet.ParticipantUuids) {
		return errors.New("minimum hours between bookings violated")
	}
	return nil
}

// checkPreventMultipleUpcomingBookings enforces MeetSettings.PreventMultipleUpcomingBookings:
// a participant with any other active booking still in the future may not book another.
func checkPreventMultipleUpcomingBookings(ctx context.Context, repo Repository, meet *Meet, settings *MeetSettings, excludeUUID string) error {
	if !settings.PreventMultipleUpcomingBookings || len(meet.ParticipantUuids) == 0 {
		return nil
	}
	now := time.Now().UTC()
	bookings, err := repo.FindParticipantBookings(ctx, meet.ParticipantUuids, &now, nil, excludeUUID)
	if err != nil {
		return err
	}
	if hasActiveConflict(bookings, meet.ParticipantUuids) {
		return errors.New("participant already has an upcoming appointment")
	}
	return nil
}
