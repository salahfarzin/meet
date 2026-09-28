package specialties

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var specialtyColumns = []string{"uuid", "kind", "slug", "names", "sort_order", "active", "created_at", "updated_at"}

func newMockRepo(t *testing.T) (Repository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewRepository(db), mock
}

func TestRepositoryCreate(t *testing.T) {
	repo, mock := newMockRepo(t)
	s := &Specialty{UUID: "s1", Kind: KindSpecialty, Slug: "anxiety", Names: map[string]string{"en": "Anxiety"}, SortOrder: 2, Active: true}

	mock.ExpectExec(`INSERT INTO specialties \(uuid, kind, slug, names, sort_order, active\) VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WithArgs("s1", "specialty", "anxiety", []byte(`{"en":"Anxiety"}`), int32(2), true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	assert.NoError(t, repo.Create(context.Background(), s))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryUpdate(t *testing.T) {
	repo, mock := newMockRepo(t)
	s := &Specialty{UUID: "s1", Kind: KindApproach, Slug: "cbt", Names: map[string]string{"en": "CBT"}, Active: false}

	mock.ExpectExec(`UPDATE specialties SET kind=\?, slug=\?, names=\?, sort_order=\?, active=\? WHERE uuid=\?`).
		WithArgs("approach", "cbt", []byte(`{"en":"CBT"}`), int32(0), false, "s1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.Update(context.Background(), s))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryGetByUUID(t *testing.T) {
	now := time.Now()

	t.Run("found", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`SELECT uuid, kind, slug, names, sort_order, active, created_at, updated_at FROM specialties WHERE uuid = \?`).
			WithArgs("s1").
			WillReturnRows(sqlmock.NewRows(specialtyColumns).AddRow("s1", "specialty", "anxiety", `{"en":"Anxiety","fa":"اضطراب"}`, 1, true, now, now))

		s, err := repo.GetByUUID(context.Background(), "s1")
		require.NoError(t, err)
		assert.Equal(t, KindSpecialty, s.Kind)
		assert.Equal(t, map[string]string{"en": "Anxiety", "fa": "اضطراب"}, s.Names)
		assert.True(t, s.Active)
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`FROM specialties WHERE uuid`).WillReturnRows(sqlmock.NewRows(specialtyColumns))

		_, err := repo.GetByUUID(context.Background(), "missing")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("corrupt names", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`FROM specialties WHERE uuid`).
			WillReturnRows(sqlmock.NewRows(specialtyColumns).AddRow("s1", "specialty", "x", `not-json`, 0, true, now, now))

		_, err := repo.GetByUUID(context.Background(), "s1")
		assert.Error(t, err)
	})

	t.Run("db error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(`FROM specialties WHERE uuid`).WillReturnError(errors.New("boom"))

		_, err := repo.GetByUUID(context.Background(), "s1")
		assert.EqualError(t, err, "boom")
	})
}

func TestRepositoryDelete(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectExec(`DELETE FROM specialties WHERE uuid = \?`).WithArgs("s1").WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.Delete(context.Background(), "s1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepositoryList(t *testing.T) {
	now := time.Now()
	listQuery := `FROM specialties WHERE \(\? = '' OR kind = \?\) AND \(\? OR active = 1\) ORDER BY kind, sort_order, slug`

	t.Run("filters by kind", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(listQuery).
			WithArgs("approach", "approach", false).
			WillReturnRows(sqlmock.NewRows(specialtyColumns).
				AddRow("a1", "approach", "cbt", `{"en":"CBT"}`, 0, true, now, now).
				AddRow("a2", "approach", "emdr", `{"en":"EMDR"}`, 1, true, now, now))

		list, err := repo.List(context.Background(), KindApproach, false)
		require.NoError(t, err)
		assert.Len(t, list, 2)
		assert.Equal(t, "emdr", list[1].Slug)
	})

	t.Run("empty result is not nil", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(listQuery).WithArgs("", "", true).WillReturnRows(sqlmock.NewRows(specialtyColumns))

		list, err := repo.List(context.Background(), "", true)
		require.NoError(t, err)
		assert.NotNil(t, list)
		assert.Empty(t, list)
	})

	t.Run("query error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(listQuery).WillReturnError(errors.New("boom"))

		_, err := repo.List(context.Background(), "", false)
		assert.Error(t, err)
	})

	t.Run("scan error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(listQuery).
			WillReturnRows(sqlmock.NewRows(specialtyColumns).AddRow("a1", "approach", "cbt", `bad`, 0, true, now, now))

		_, err := repo.List(context.Background(), "", false)
		assert.Error(t, err)
	})
}

func TestRepositorySlugExists(t *testing.T) {
	slugQuery := `SELECT COUNT\(1\) FROM specialties WHERE kind = \? AND slug = \? AND uuid <> \?`

	t.Run("exists", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(slugQuery).WithArgs("specialty", "anxiety", "s1").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		exists, err := repo.SlugExists(context.Background(), KindSpecialty, "anxiety", "s1")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("free", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(slugQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		exists, err := repo.SlugExists(context.Background(), KindSpecialty, "anxiety", "")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("error", func(t *testing.T) {
		repo, mock := newMockRepo(t)
		mock.ExpectQuery(slugQuery).WillReturnError(errors.New("boom"))

		_, err := repo.SlugExists(context.Background(), KindSpecialty, "anxiety", "")
		assert.Error(t, err)
	})
}

func TestRepositoryWritesRejectUnmarshalableNames(t *testing.T) {
	// map[string]string always marshals; this guards the Exec error path instead.
	repo, mock := newMockRepo(t)
	mock.ExpectExec(`INSERT INTO specialties`).WillReturnError(errors.New("duplicate"))
	assert.Error(t, repo.Create(context.Background(), &Specialty{Kind: KindSpecialty, Names: map[string]string{}}))

	mock.ExpectExec(`UPDATE specialties`).WillReturnError(errors.New("boom"))
	assert.Error(t, repo.Update(context.Background(), &Specialty{Kind: KindSpecialty, Names: map[string]string{}}))
}
