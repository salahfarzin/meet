package doctors

type Doctor struct {
    ID        string
    Name      string
    Specialty string
    Email     string
    Phone     string
}

type DoctorRepository interface {
    CreateDoctor(doctor Doctor) error
    GetDoctorByID(id string) (Doctor, error)
    GetAllDoctors() ([]Doctor, error)
    UpdateDoctor(doctor Doctor) error
    DeleteDoctor(id string) error
}

type InMemoryDoctorRepository struct {
    doctors map[string]Doctor
}

func NewInMemoryDoctorRepository() *InMemoryDoctorRepository {
    return &InMemoryDoctorRepository{
        doctors: make(map[string]Doctor),
    }
}

func (r *InMemoryDoctorRepository) CreateDoctor(doctor Doctor) error {
    r.doctors[doctor.ID] = doctor
    return nil
}

func (r *InMemoryDoctorRepository) GetDoctorByID(id string) (Doctor, error) {
    doctor, exists := r.doctors[id]
    if !exists {
        return Doctor{}, fmt.Errorf("doctor not found")
    }
    return doctor, nil
}

func (r *InMemoryDoctorRepository) GetAllDoctors() ([]Doctor, error) {
    var doctorList []Doctor
    for _, doctor := range r.doctors {
        doctorList = append(doctorList, doctor)
    }
    return doctorList, nil
}

func (r *InMemoryDoctorRepository) UpdateDoctor(doctor Doctor) error {
    if _, exists := r.doctors[doctor.ID]; !exists {
        return fmt.Errorf("doctor not found")
    }
    r.doctors[doctor.ID] = doctor
    return nil
}

func (r *InMemoryDoctorRepository) DeleteDoctor(id string) error {
    if _, exists := r.doctors[id]; !exists {
        return fmt.Errorf("doctor not found")
    }
    delete(r.doctors, id)
    return nil
}