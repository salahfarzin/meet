package doctors

import (
	"context"
	"errors"
)

// Doctor represents a doctor entity.
type Doctor struct {
	ID   string
	Name string
	// Add other relevant fields
}

// DoctorRepository defines the methods for interacting with the doctor data store.
type DoctorRepository interface {
	CreateDoctor(ctx context.Context, doctor *Doctor) error
	GetDoctorByID(ctx context.Context, id string) (*Doctor, error)
	UpdateDoctor(ctx context.Context, doctor *Doctor) error
	DeleteDoctor(ctx context.Context, id string) error
}

// DoctorService contains business logic for managing doctors.
type DoctorService struct {
	repo DoctorRepository
}

// NewDoctorService creates a new DoctorService.
func NewDoctorService(repo DoctorRepository) *DoctorService {
	return &DoctorService{repo: repo}
}

// CreateDoctor creates a new doctor.
func (s *DoctorService) CreateDoctor(ctx context.Context, doctor *Doctor) error {
	if doctor == nil {
		return errors.New("doctor cannot be nil")
	}
	return s.repo.CreateDoctor(ctx, doctor)
}

// GetDoctorByID retrieves a doctor by ID.
func (s *DoctorService) GetDoctorByID(ctx context.Context, id string) (*Doctor, error) {
	return s.repo.GetDoctorByID(ctx, id)
}

// UpdateDoctor updates an existing doctor.
func (s *DoctorService) UpdateDoctor(ctx context.Context, doctor *Doctor) error {
	if doctor == nil {
		return errors.New("doctor cannot be nil")
	}
	return s.repo.UpdateDoctor(ctx, doctor)
}

// DeleteDoctor deletes a doctor by ID.
func (s *DoctorService) DeleteDoctor(ctx context.Context, id string) error {
	return s.repo.DeleteDoctor(ctx, id)
}