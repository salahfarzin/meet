package appointments

import "fmt"

type AppointmentRepository interface {
	Create(appointment Appointment) error
	GetByID(id string) (Appointment, error)
	Update(appointment Appointment) error
	Delete(id string) error
	ListByDoctor(doctorID string) ([]Appointment, error)
	ListByPsychologist(psychologistID string) ([]Appointment, error)
}

type InMemoryAppointmentRepository struct {
	appointments map[string]Appointment
}

func NewInMemoryAppointmentRepository() *InMemoryAppointmentRepository {
	return &InMemoryAppointmentRepository{
		appointments: make(map[string]Appointment),
	}
}

func (repo *InMemoryAppointmentRepository) Create(appointment Appointment) error {
	repo.appointments[appointment.ID] = appointment
	return nil
}

func (repo *InMemoryAppointmentRepository) GetByID(id string) (Appointment, error) {
	appointment, exists := repo.appointments[id]
	if !exists {
		return Appointment{}, fmt.Errorf("appointment not found")
	}
	return appointment, nil
}

func (repo *InMemoryAppointmentRepository) Update(appointment Appointment) error {
	repo.appointments[appointment.ID] = appointment
	return nil
}

func (repo *InMemoryAppointmentRepository) Delete(id string) error {
	delete(repo.appointments, id)
	return nil
}

func (repo *InMemoryAppointmentRepository) ListByDoctor(doctorID string) ([]Appointment, error) {
	var result []Appointment
	for _, appointment := range repo.appointments {
		if appointment.DoctorID == doctorID {
			result = append(result, appointment)
		}
	}
	return result, nil
}

func (repo *InMemoryAppointmentRepository) ListByPsychologist(psychologistID string) ([]Appointment, error) {
	var result []Appointment
	for _, appointment := range repo.appointments {
		if appointment.PsychologistID == psychologistID {
			result = append(result, appointment)
		}
	}
	return result, nil
}
