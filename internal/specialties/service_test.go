package specialties

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	created    *Specialty
	updated    *Specialty
	deleted    string
	stored     map[string]*Specialty
	slugTaken  bool
	createErr  error
	updateErr  error
	deleteErr  error
	getErr     error
	listErr    error
	slugErr    error
	listResult []*Specialty
	listKind   Kind
	listAll    bool
}

func newRepoStub() *mockRepo {
	return &mockRepo{stored: map[string]*Specialty{}}
}

func (m *mockRepo) Create(_ context.Context, s *Specialty) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = s
	m.stored[s.UUID] = s
	return nil
}

func (m *mockRepo) Update(_ context.Context, s *Specialty) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = s
	m.stored[s.UUID] = s
	return nil
}

func (m *mockRepo) GetByUUID(_ context.Context, uuid string) (*Specialty, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	s, ok := m.stored[uuid]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *mockRepo) Delete(_ context.Context, uuid string) error {
	m.deleted = uuid
	return m.deleteErr
}

func (m *mockRepo) List(_ context.Context, kind Kind, includeInactive bool) ([]*Specialty, error) {
	m.listKind, m.listAll = kind, includeInactive
	return m.listResult, m.listErr
}

func (m *mockRepo) SlugExists(context.Context, Kind, string, string) (bool, error) {
	return m.slugTaken, m.slugErr
}

func validSpecialty() *Specialty {
	return &Specialty{
		Kind:  KindSpecialty,
		Slug:  "  Anxiety-Disorders ",
		Names: map[string]string{"EN": " Anxiety ", "fa": "  ", "": "ignored"},
	}
}

func TestServiceCreate(t *testing.T) {
	t.Run("normalizes, activates and stores", func(t *testing.T) {
		repo := newRepoStub()
		created, err := NewService(repo).Create(context.Background(), validSpecialty())
		require.NoError(t, err)

		assert.NotEmpty(t, created.UUID)
		assert.True(t, created.Active)
		assert.Equal(t, "anxiety-disorders", created.Slug)
		assert.Equal(t, map[string]string{"en": "Anxiety"}, created.Names)
	})

	t.Run("validation errors", func(t *testing.T) {
		cases := map[string]*Specialty{
			"bad kind":      {Kind: "other", Slug: "x", Names: map[string]string{"en": "X"}},
			"bad slug":      {Kind: KindSpecialty, Slug: "has space", Names: map[string]string{"en": "X"}},
			"empty slug":    {Kind: KindSpecialty, Slug: "", Names: map[string]string{"en": "X"}},
			"long slug":     {Kind: KindSpecialty, Slug: string(make([]byte, 65)), Names: map[string]string{"en": "X"}},
			"no names":      {Kind: KindSpecialty, Slug: "x", Names: map[string]string{"en": " "}},
			"nil names map": {Kind: KindApproach, Slug: "x"},
		}
		for name, s := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := NewService(newRepoStub()).Create(context.Background(), s)
				assert.Error(t, err)
			})
		}
	})

	t.Run("duplicate slug", func(t *testing.T) {
		repo := newRepoStub()
		repo.slugTaken = true
		_, err := NewService(repo).Create(context.Background(), validSpecialty())
		assert.ErrorContains(t, err, "already used")
	})

	t.Run("slug check error", func(t *testing.T) {
		repo := newRepoStub()
		repo.slugErr = errors.New("boom")
		_, err := NewService(repo).Create(context.Background(), validSpecialty())
		assert.EqualError(t, err, "boom")
	})

	t.Run("repo error", func(t *testing.T) {
		repo := newRepoStub()
		repo.createErr = errors.New("boom")
		_, err := NewService(repo).Create(context.Background(), validSpecialty())
		assert.EqualError(t, err, "boom")
	})
}

func TestServiceUpdate(t *testing.T) {
	existing := func(repo *mockRepo) {
		repo.stored["s1"] = &Specialty{UUID: "s1", Kind: KindSpecialty, Slug: "anxiety", Names: map[string]string{"en": "Anxiety"}, Active: true}
	}

	t.Run("updates and keeps the requested active flag", func(t *testing.T) {
		repo := newRepoStub()
		existing(repo)
		s := validSpecialty()
		s.UUID = "s1"
		s.Active = false

		updated, err := NewService(repo).Update(context.Background(), s)
		require.NoError(t, err)
		assert.False(t, updated.Active)
		assert.Equal(t, "anxiety-disorders", repo.updated.Slug)
	})

	t.Run("requires uuid", func(t *testing.T) {
		_, err := NewService(newRepoStub()).Update(context.Background(), validSpecialty())
		assert.EqualError(t, err, "UUID is required")
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := NewService(newRepoStub()).Update(context.Background(), &Specialty{UUID: "s1", Kind: "bad"})
		assert.ErrorIs(t, err, ErrInvalidKind)
	})

	t.Run("not found", func(t *testing.T) {
		s := validSpecialty()
		s.UUID = "missing"
		_, err := NewService(newRepoStub()).Update(context.Background(), s)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("duplicate slug", func(t *testing.T) {
		repo := newRepoStub()
		existing(repo)
		repo.slugTaken = true
		s := validSpecialty()
		s.UUID = "s1"
		_, err := NewService(repo).Update(context.Background(), s)
		assert.ErrorContains(t, err, "already used")
	})

	t.Run("repo error", func(t *testing.T) {
		repo := newRepoStub()
		existing(repo)
		repo.updateErr = errors.New("boom")
		s := validSpecialty()
		s.UUID = "s1"
		_, err := NewService(repo).Update(context.Background(), s)
		assert.EqualError(t, err, "boom")
	})
}

func TestServiceDelete(t *testing.T) {
	repo := newRepoStub()
	svc := NewService(repo)

	assert.EqualError(t, svc.Delete(context.Background(), ""), "UUID is required")
	assert.NoError(t, svc.Delete(context.Background(), "s1"))
	assert.Equal(t, "s1", repo.deleted)
}

func TestServiceGetAll(t *testing.T) {
	t.Run("passes filters through", func(t *testing.T) {
		repo := newRepoStub()
		repo.listResult = []*Specialty{{UUID: "a1"}}
		list, err := NewService(repo).GetAll(context.Background(), KindApproach, true)
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, KindApproach, repo.listKind)
		assert.True(t, repo.listAll)
	})

	t.Run("empty kind means all", func(t *testing.T) {
		repo := newRepoStub()
		_, err := NewService(repo).GetAll(context.Background(), "", false)
		assert.NoError(t, err)
	})

	t.Run("invalid kind", func(t *testing.T) {
		_, err := NewService(newRepoStub()).GetAll(context.Background(), "nope", false)
		assert.ErrorIs(t, err, ErrInvalidKind)
	})
}
