package appointments

import (
	"context"
	"errors"
)

type AppointmentRepository interface {
	Create(ctx context.Context, appointment *Appointment) error
	Update(ctx context.Context, appointment *Appointment) error
	GetByID(ctx context.Context, id string) (*Appointment, error)
	GetAll(ctx context.Context) ([]*Appointment, error)
}

type AppointmentService struct {
	repo AppointmentRepository
}

func NewAppointmentService(repo AppointmentRepository) *AppointmentService {
	return &AppointmentService{repo: repo}
}

func (s *AppointmentService) CreateAppointment(ctx context.Context, appointment *Appointment) error {
	if appointment == nil {
		return errors.New("appointment cannot be nil")
	}
	return s.repo.Create(ctx, appointment)
}

func (s *AppointmentService) UpdateAppointment(ctx context.Context, appointment *Appointment) error {
	if appointment == nil {
		return errors.New("appointment cannot be nil")
	}
	return s.repo.Update(ctx, appointment)
}

func (s *AppointmentService) GetAppointmentByID(ctx context.Context, id string) (*Appointment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AppointmentService) GetAllAppointments(ctx context.Context) ([]*Appointment, error) {
	return s.repo.GetAll(ctx)
}
