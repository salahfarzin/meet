package psychologists

type Psychologist struct {
    ID   string
    Name string
    Specialization string
}

type PsychologistRepository interface {
    CreatePsychologist(psychologist *Psychologist) error
    GetPsychologist(id string) (*Psychologist, error)
    UpdatePsychologist(psychologist *Psychologist) error
    DeletePsychologist(id string) error
    ListPsychologists() ([]*Psychologist, error)
}

type InMemoryPsychologistRepository struct {
    psychologists map[string]*Psychologist
}

func NewInMemoryPsychologistRepository() *InMemoryPsychologistRepository {
    return &InMemoryPsychologistRepository{
        psychologists: make(map[string]*Psychologist),
    }
}

func (r *InMemoryPsychologistRepository) CreatePsychologist(psychologist *Psychologist) error {
    r.psychologists[psychologist.ID] = psychologist
    return nil
}

func (r *InMemoryPsychologistRepository) GetPsychologist(id string) (*Psychologist, error) {
    psychologist, exists := r.psychologists[id]
    if !exists {
        return nil, fmt.Errorf("psychologist not found")
    }
    return psychologist, nil
}

func (r *InMemoryPsychologistRepository) UpdatePsychologist(psychologist *Psychologist) error {
    r.psychologists[psychologist.ID] = psychologist
    return nil
}

func (r *InMemoryPsychologistRepository) DeletePsychologist(id string) error {
    delete(r.psychologists, id)
    return nil
}

func (r *InMemoryPsychologistRepository) ListPsychologists() ([]*Psychologist, error) {
    var list []*Psychologist
    for _, psychologist := range r.psychologists {
        list = append(list, psychologist)
    }
    return list, nil
}