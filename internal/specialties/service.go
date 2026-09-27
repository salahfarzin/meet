package specialties

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// slugPattern keeps slugs URL/i18n-key safe: lowercase words joined by - or _.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

const maxSlugLength = 64

// ErrInvalidKind is returned for a kind other than specialty/approach.
var ErrInvalidKind = fmt.Errorf("kind must be %q or %q", KindSpecialty, KindApproach)

type Service interface {
	Create(ctx context.Context, s *Specialty) (*Specialty, error)
	Update(ctx context.Context, s *Specialty) (*Specialty, error)
	Delete(ctx context.Context, uuid string) error
	GetAll(ctx context.Context, kind Kind, includeInactive bool) ([]*Specialty, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (svc *service) Create(ctx context.Context, s *Specialty) (*Specialty, error) {
	normalize(s)
	if err := validate(s); err != nil {
		return nil, err
	}
	if err := svc.checkSlug(ctx, s); err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	s.UUID = id.String()
	// proto3 can't tell an omitted bool from false; new entries start visible
	// and are hidden later through Update.
	s.Active = true
	if err := svc.repo.Create(ctx, s); err != nil {
		return nil, err
	}
	return svc.repo.GetByUUID(ctx, s.UUID)
}

func (svc *service) Update(ctx context.Context, s *Specialty) (*Specialty, error) {
	if s.UUID == "" {
		return nil, errors.New("UUID is required")
	}
	normalize(s)
	if err := validate(s); err != nil {
		return nil, err
	}
	if _, err := svc.repo.GetByUUID(ctx, s.UUID); err != nil {
		return nil, err
	}
	if err := svc.checkSlug(ctx, s); err != nil {
		return nil, err
	}
	if err := svc.repo.Update(ctx, s); err != nil {
		return nil, err
	}
	return svc.repo.GetByUUID(ctx, s.UUID)
}

func (svc *service) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("UUID is required")
	}
	return svc.repo.Delete(ctx, id)
}

func (svc *service) GetAll(ctx context.Context, kind Kind, includeInactive bool) ([]*Specialty, error) {
	if kind != "" && !kind.valid() {
		return nil, ErrInvalidKind
	}
	return svc.repo.List(ctx, kind, includeInactive)
}

func (svc *service) checkSlug(ctx context.Context, s *Specialty) error {
	exists, err := svc.repo.SlugExists(ctx, s.Kind, s.Slug, s.UUID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("slug %q is already used by another %s", s.Slug, s.Kind)
	}
	return nil
}

func (k Kind) valid() bool {
	return k == KindSpecialty || k == KindApproach
}

// normalize trims input and drops blank translations so an empty "fa" field
// in the admin form doesn't shadow the client's fallback language.
func normalize(s *Specialty) {
	s.Slug = strings.ToLower(strings.TrimSpace(s.Slug))
	names := make(map[string]string, len(s.Names))
	for lang, name := range s.Names {
		lang = strings.ToLower(strings.TrimSpace(lang))
		name = strings.TrimSpace(name)
		if lang != "" && name != "" {
			names[lang] = name
		}
	}
	s.Names = names
}

func validate(s *Specialty) error {
	if !s.Kind.valid() {
		return ErrInvalidKind
	}
	if len(s.Slug) > maxSlugLength || !slugPattern.MatchString(s.Slug) {
		return errors.New("slug must be lowercase letters and digits joined by - or _ (max 64)")
	}
	if len(s.Names) == 0 {
		return errors.New("at least one name translation is required")
	}
	return nil
}
