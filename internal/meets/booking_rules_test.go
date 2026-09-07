package meets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckMinHoursBetweenBookings(t *testing.T) {
	ctx := context.Background()
	meet := &Meet{
		ParticipantUuids: []string{"p1"},
		Start:            time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}

	t.Run("disabled when minHours is zero", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				t.Fatal("must not query when disabled")
				return nil, nil
			},
		}
		err := checkMinHoursBetweenBookings(ctx, repo, meet, &MeetSettings{MinHoursBetweenBookings: 0}, "")
		assert.NoError(t, err)
	})

	t.Run("no participants means no check needed", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				t.Fatal("must not query with no participants")
				return nil, nil
			},
		}
		err := checkMinHoursBetweenBookings(ctx, repo, &Meet{Start: meet.Start}, &MeetSettings{MinHoursBetweenBookings: 2}, "")
		assert.NoError(t, err)
	})

	t.Run("violated when an active booking overlaps the window", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				return []*Meet{{UUID: "other", ParticipantUuids: []string{"p1"}}}, nil
			},
		}
		err := checkMinHoursBetweenBookings(ctx, repo, meet, &MeetSettings{MinHoursBetweenBookings: 2}, "")
		assert.EqualError(t, err, "minimum hours between bookings violated")
	})

	t.Run("not violated when the only overlapping booking was cancelled", func(t *testing.T) {
		cancelled := `{"participants":[{"uuid":"p1","status":"cancelled"}]}`
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				return []*Meet{{UUID: "other", ParticipantUuids: []string{"p1"}, Settings: &cancelled}}, nil
			},
		}
		err := checkMinHoursBetweenBookings(ctx, repo, meet, &MeetSettings{MinHoursBetweenBookings: 2}, "")
		assert.NoError(t, err)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				return nil, errors.New("db down")
			},
		}
		err := checkMinHoursBetweenBookings(ctx, repo, meet, &MeetSettings{MinHoursBetweenBookings: 2}, "")
		assert.EqualError(t, err, "db down")
	})
}

func TestCheckPreventMultipleUpcomingBookings(t *testing.T) {
	ctx := context.Background()
	meet := &Meet{
		ParticipantUuids: []string{"p1"},
		Start:            time.Now().Add(24 * time.Hour),
	}

	t.Run("disabled by default", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				t.Fatal("must not query when disabled")
				return nil, nil
			},
		}
		err := checkPreventMultipleUpcomingBookings(ctx, repo, meet, &MeetSettings{}, "")
		assert.NoError(t, err)
	})

	t.Run("violated when participant has an active upcoming booking", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				assert.NotNil(t, from)
				assert.Nil(t, to)
				return []*Meet{{UUID: "other", ParticipantUuids: []string{"p1"}}}, nil
			},
		}
		err := checkPreventMultipleUpcomingBookings(ctx, repo, meet, &MeetSettings{PreventMultipleUpcomingBookings: true}, "")
		assert.EqualError(t, err, "participant already has an upcoming appointment")
	})

	t.Run("not violated when the only upcoming booking was cancelled", func(t *testing.T) {
		cancelled := `{"participants":[{"uuid":"p1","status":"cancelled"}]}`
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				return []*Meet{{UUID: "other", ParticipantUuids: []string{"p1"}, Settings: &cancelled}}, nil
			},
		}
		err := checkPreventMultipleUpcomingBookings(ctx, repo, meet, &MeetSettings{PreventMultipleUpcomingBookings: true}, "")
		assert.NoError(t, err)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := &MockRepository{
			FindParticipantBookingsFunc: func(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*Meet, error) {
				return nil, errors.New("db down")
			},
		}
		err := checkPreventMultipleUpcomingBookings(ctx, repo, meet, &MeetSettings{PreventMultipleUpcomingBookings: true}, "")
		assert.EqualError(t, err, "db down")
	})
}

func TestHasActiveConflict(t *testing.T) {
	cancelled := `{"participants":[{"uuid":"p1","status":"cancelled"}]}`
	bookings := []*Meet{
		{UUID: "m1", ParticipantUuids: []string{"p1"}, Settings: &cancelled},
		{UUID: "m2", ParticipantUuids: []string{"p2"}},
	}

	assert.False(t, hasActiveConflict(bookings, []string{"p1"}))
	assert.True(t, hasActiveConflict(bookings, []string{"p2"}))
	assert.True(t, hasActiveConflict(bookings, []string{"p1", "p2"}))
	assert.False(t, hasActiveConflict(nil, []string{"p1"}))
}
