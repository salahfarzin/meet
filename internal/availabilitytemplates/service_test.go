package availabilitytemplates

import (
	"context"
	"testing"
	"time"

	"github.com/salahfarzin/meet/internal/meets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a hand-written test double for Repository, following the
// same shape as meets.MockRepository (func-field-per-method, sane defaults).
type MockRepository struct {
	CreateFunc                func(ctx context.Context, t *Template) error
	UpdateFunc                func(ctx context.Context, t *Template) error
	GetByUUIDFunc             func(ctx context.Context, uuid string) (*Template, error)
	DeleteFunc                func(ctx context.Context, uuid string) error
	ListActiveByOrganizerFunc func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error)
	HasOccurrenceFunc         func(ctx context.Context, templateUuid, occurrenceDate string) (bool, error)
	RecordOccurrenceFunc      func(ctx context.Context, templateUuid, occurrenceDate string, status OccurrenceStatus, meetUuid *string) error

	RecordedOccurrences []recordedOccurrence
}

type recordedOccurrence struct {
	TemplateUuid   string
	OccurrenceDate string
	Status         OccurrenceStatus
	MeetUuid       *string
}

func (m *MockRepository) Create(ctx context.Context, t *Template) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, t)
	}
	return nil
}
func (m *MockRepository) Update(ctx context.Context, t *Template) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, t)
	}
	return nil
}
func (m *MockRepository) GetByUUID(ctx context.Context, uuid string) (*Template, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return &Template{UUID: uuid}, nil
}
func (m *MockRepository) Delete(ctx context.Context, uuid string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, uuid)
	}
	return nil
}
func (m *MockRepository) ListActiveByOrganizer(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
	if m.ListActiveByOrganizerFunc != nil {
		return m.ListActiveByOrganizerFunc(ctx, organizerUuid, from, to)
	}
	return nil, nil
}
func (m *MockRepository) HasOccurrence(ctx context.Context, templateUuid, occurrenceDate string) (bool, error) {
	if m.HasOccurrenceFunc != nil {
		return m.HasOccurrenceFunc(ctx, templateUuid, occurrenceDate)
	}
	return false, nil
}
func (m *MockRepository) RecordOccurrence(ctx context.Context, templateUuid, occurrenceDate string, status OccurrenceStatus, meetUuid *string) error {
	m.RecordedOccurrences = append(m.RecordedOccurrences, recordedOccurrence{templateUuid, occurrenceDate, status, meetUuid})
	if m.RecordOccurrenceFunc != nil {
		return m.RecordOccurrenceFunc(ctx, templateUuid, occurrenceDate, status, meetUuid)
	}
	return nil
}

// MockMeetsRepository is a minimal test double for meets.Repository — only
// HasConflict/Create are exercised by Materialize, the rest are unused no-ops
// required to satisfy the interface.
type MockMeetsRepository struct {
	HasConflictFunc func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error)
	CreateFunc      func(ctx context.Context, meet *meets.Meet) error
}

func (m *MockMeetsRepository) GenerateAvailableSlots(ctx context.Context, organizerID string, from, to time.Time, priceUUID *string) ([]*meets.Meet, error) {
	return nil, nil
}
func (m *MockMeetsRepository) Create(ctx context.Context, meet *meets.Meet) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, meet)
	}
	return nil
}
func (m *MockMeetsRepository) GetByID(ctx context.Context, id string) (*meets.Meet, error) {
	return nil, nil
}
func (m *MockMeetsRepository) GetByUUID(ctx context.Context, uuid string) (*meets.Meet, error) {
	return nil, nil
}
func (m *MockMeetsRepository) Update(ctx context.Context, meet *meets.Meet) error { return nil }
func (m *MockMeetsRepository) Delete(ctx context.Context, uuid string) error      { return nil }
func (m *MockMeetsRepository) QueryMeets(ctx context.Context, options *meets.MeetQueryOptions) ([]*meets.Meet, int, error) {
	return nil, 0, nil
}
func (m *MockMeetsRepository) QueryMeetsCursor(ctx context.Context, options *meets.MeetQueryOptions) ([]*meets.Meet, int, string, bool, error) {
	return nil, 0, "", false, nil
}
func (m *MockMeetsRepository) HasConflict(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
	if m.HasConflictFunc != nil {
		return m.HasConflictFunc(ctx, organizerId, start, end, excludeUUID...)
	}
	return false, nil
}
func (m *MockMeetsRepository) FindParticipantBookings(ctx context.Context, participantUuids []string, from, to *time.Time, excludeUUID string) ([]*meets.Meet, error) {
	return nil, nil
}

func TestServiceCreateValidation(t *testing.T) {
	svc := NewService(&MockRepository{}, &MockMeetsRepository{})

	tests := []struct {
		name string
		t    *Template
	}{
		{"missing organizer", &Template{Weekday: 1, StartTime: "09:00:00", EndTime: "10:00:00", EffectiveFrom: time.Now()}},
		{"weekday too low", &Template{OrganizerUuid: "org1", Weekday: -1, StartTime: "09:00:00", EndTime: "10:00:00", EffectiveFrom: time.Now()}},
		{"weekday too high", &Template{OrganizerUuid: "org1", Weekday: 7, StartTime: "09:00:00", EndTime: "10:00:00", EffectiveFrom: time.Now()}},
		{"missing times", &Template{OrganizerUuid: "org1", Weekday: 1, EffectiveFrom: time.Now()}},
		{"start after end", &Template{OrganizerUuid: "org1", Weekday: 1, StartTime: "10:00:00", EndTime: "09:00:00", EffectiveFrom: time.Now()}},
		{"missing effective_from", &Template{OrganizerUuid: "org1", Weekday: 1, StartTime: "09:00:00", EndTime: "10:00:00"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tt.t)
			assert.Error(t, err)
		})
	}
}

func TestServiceCreateSuccess(t *testing.T) {
	repo := &MockRepository{}
	svc := NewService(repo, &MockMeetsRepository{})

	tmpl := &Template{OrganizerUuid: "org1", Weekday: 1, StartTime: "09:00:00", EndTime: "13:00:00", EffectiveFrom: time.Now()}
	created, err := svc.Create(context.Background(), tmpl)
	require.NoError(t, err)
	assert.NotEmpty(t, created.UUID)
	assert.True(t, created.Active)
}

func TestServiceCreateRejectsOverlap(t *testing.T) {
	existingFrom := time.Now()
	repo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			return []*Template{
				{UUID: "existing", OrganizerUuid: "org1", Weekday: 0, StartTime: "09:00:00", EndTime: "13:00:00", EffectiveFrom: existingFrom},
			}, nil
		},
	}
	svc := NewService(repo, &MockMeetsRepository{})

	tests := []struct {
		name      string
		candidate *Template
		wantErr   bool
	}{
		{"overlapping range", &Template{OrganizerUuid: "org1", Weekday: 0, StartTime: "09:00:00", EndTime: "11:00:00", EffectiveFrom: time.Now()}, true},
		{"different weekday, same time", &Template{OrganizerUuid: "org1", Weekday: 1, StartTime: "09:00:00", EndTime: "11:00:00", EffectiveFrom: time.Now()}, false},
		{"adjacent, non-overlapping time", &Template{OrganizerUuid: "org1", Weekday: 0, StartTime: "13:00:00", EndTime: "15:00:00", EffectiveFrom: time.Now()}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tt.candidate)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServiceUpdateAllowsOverlapWithItself(t *testing.T) {
	existingFrom := time.Now()
	repo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			return []*Template{
				{UUID: "self", OrganizerUuid: "org1", Weekday: 0, StartTime: "09:00:00", EndTime: "13:00:00", EffectiveFrom: existingFrom},
			}, nil
		},
	}
	svc := NewService(repo, &MockMeetsRepository{})

	_, err := svc.Update(context.Background(), &Template{UUID: "self", OrganizerUuid: "org1", Weekday: 0, StartTime: "09:00:00", EndTime: "13:00:00", EffectiveFrom: time.Now()})
	assert.NoError(t, err)
}

func TestFirstOccurrenceOnOrAfter(t *testing.T) {
	// 2026-08-01 is a Saturday (weekday 6).
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// Next Monday (weekday 1) on/after Aug 1 2026 is Aug 3.
	got := firstOccurrenceOnOrAfter(1, from)
	assert.Equal(t, time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), got)

	// Asking for Saturday (weekday 6) itself returns the same day.
	got = firstOccurrenceOnOrAfter(6, from)
	assert.Equal(t, from, got)
}

func TestMaterialize_CreatesOccurrenceWithinWindow(t *testing.T) {
	tmplRepo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			return []*Template{{
				UUID:          "tmpl-1",
				OrganizerUuid: "org1",
				Weekday:       1, // Monday
				StartTime:     "09:00:00",
				EndTime:       "13:00:00",
				EffectiveFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			}}, nil
		},
	}

	var created []*meets.Meet
	meetsRepo := &MockMeetsRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, m *meets.Meet) error {
			m.UUID = "generated-meet-uuid"
			created = append(created, m)
			return nil
		},
	}

	svc := NewService(tmplRepo, meetsRepo)

	// Window: one week, Aug 1 (Sat) .. Aug 7 (Fri) 2026 — contains exactly one Monday (Aug 3).
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	err := svc.Materialize(context.Background(), "org1", from, to)
	require.NoError(t, err)

	require.Len(t, created, 1)
	assert.Equal(t, "org1", created[0].OrganizerUuid)
	assert.Equal(t, time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC), created[0].Start)
	assert.Equal(t, time.Date(2026, 8, 3, 13, 0, 0, 0, time.UTC), created[0].End)
	require.NotNil(t, created[0].TemplateUuid)
	assert.Equal(t, "tmpl-1", *created[0].TemplateUuid)

	require.Len(t, tmplRepo.RecordedOccurrences, 1)
	assert.Equal(t, "2026-08-03", tmplRepo.RecordedOccurrences[0].OccurrenceDate)
	assert.Equal(t, OccurrenceMaterialized, tmplRepo.RecordedOccurrences[0].Status)
}

func TestMaterialize_SkipsAlreadyTrackedOccurrence(t *testing.T) {
	tmplRepo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			return []*Template{{
				UUID:          "tmpl-1",
				OrganizerUuid: "org1",
				Weekday:       1,
				StartTime:     "09:00:00",
				EndTime:       "13:00:00",
				EffectiveFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			}}, nil
		},
		HasOccurrenceFunc: func(ctx context.Context, templateUuid, occurrenceDate string) (bool, error) {
			return true, nil // already materialized or explicitly skipped
		},
	}

	createCalled := false
	meetsRepo := &MockMeetsRepository{
		CreateFunc: func(ctx context.Context, m *meets.Meet) error {
			createCalled = true
			return nil
		},
	}

	svc := NewService(tmplRepo, meetsRepo)
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	err := svc.Materialize(context.Background(), "org1", from, to)
	require.NoError(t, err)
	assert.False(t, createCalled, "should not create a meet for an already-tracked occurrence")
	assert.Empty(t, tmplRepo.RecordedOccurrences)
}

func TestMaterialize_SkipsOnConflictWithoutRecording(t *testing.T) {
	tmplRepo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			return []*Template{{
				UUID:          "tmpl-1",
				OrganizerUuid: "org1",
				Weekday:       1,
				StartTime:     "09:00:00",
				EndTime:       "13:00:00",
				EffectiveFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			}}, nil
		},
	}

	createCalled := false
	meetsRepo := &MockMeetsRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return true, nil // something already occupies this window
		},
		CreateFunc: func(ctx context.Context, m *meets.Meet) error {
			createCalled = true
			return nil
		},
	}

	svc := NewService(tmplRepo, meetsRepo)
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)

	err := svc.Materialize(context.Background(), "org1", from, to)
	require.NoError(t, err)
	assert.False(t, createCalled)
	// Not recorded as skipped either — left untracked so a future call reconsiders it.
	assert.Empty(t, tmplRepo.RecordedOccurrences)
}

func TestMaterialize_RespectsEffectiveUntil(t *testing.T) {
	until := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) // exactly the first Monday in window
	tmplRepo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			return []*Template{{
				UUID:           "tmpl-1",
				OrganizerUuid:  "org1",
				Weekday:        1,
				StartTime:      "09:00:00",
				EndTime:        "13:00:00",
				EffectiveFrom:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
				EffectiveUntil: &until,
			}}, nil
		},
	}

	var created []*meets.Meet
	meetsRepo := &MockMeetsRepository{
		HasConflictFunc: func(ctx context.Context, organizerId string, start, end time.Time, excludeUUID ...string) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, m *meets.Meet) error {
			created = append(created, m)
			return nil
		},
	}

	svc := NewService(tmplRepo, meetsRepo)
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// Two-week window: Aug 3 and Aug 10 are both Mondays, but effective_until cuts it off at Aug 3.
	to := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)

	err := svc.Materialize(context.Background(), "org1", from, to)
	require.NoError(t, err)
	require.Len(t, created, 1)
	assert.Equal(t, time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC), created[0].Start)
}

func TestRecordSkip(t *testing.T) {
	tmplRepo := &MockRepository{}
	svc := NewService(tmplRepo, &MockMeetsRepository{})

	err := svc.RecordSkip(context.Background(), "tmpl-1", time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, tmplRepo.RecordedOccurrences, 1)
	assert.Equal(t, "2026-08-03", tmplRepo.RecordedOccurrences[0].OccurrenceDate)
	assert.Equal(t, OccurrenceSkipped, tmplRepo.RecordedOccurrences[0].Status)
	assert.Nil(t, tmplRepo.RecordedOccurrences[0].MeetUuid)
}

func TestGetAllDelegates(t *testing.T) {
	called := false
	tmplRepo := &MockRepository{
		ListActiveByOrganizerFunc: func(ctx context.Context, organizerUuid string, from, to time.Time) ([]*Template, error) {
			called = true
			assert.Equal(t, "org1", organizerUuid)
			return []*Template{{UUID: "tmpl-1"}}, nil
		},
	}
	svc := NewService(tmplRepo, &MockMeetsRepository{})

	result, err := svc.GetAll(context.Background(), "org1")
	require.NoError(t, err)
	assert.True(t, called)
	require.Len(t, result, 1)
}
