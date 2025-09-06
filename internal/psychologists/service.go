package psychologists

import (
	"context"
	"errors"
)

type Psychologist struct {
	ID   string
	Name string
}

type PsychologistRepository interface {
	FindByID(ctx context.Context, id string) (*Psychologist, error)
	Save(ctx context.Context, psychologist *Psychologist) error
}

type PsychologistService struct {
	repo PsychologistRepository
}

func NewPsychologistService(repo PsychologistRepository) *PsychologistService {
	return &PsychologistService{repo: repo}
}

func (s *PsychologistService) GetPsychologist(ctx context.Context, id string) (*Psychologist, error) {
	psychologist, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if psychologist == nil {
		return nil, errors.New("psychologist not found")
	}
	return psychologist, nil
}

func (s *PsychologistService) CreatePsychologist(ctx context.Context, psychologist *Psychologist) error {
	return s.repo.Save(ctx, psychologist)
}