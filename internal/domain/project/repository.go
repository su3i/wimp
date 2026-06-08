package project

type ProjectRepository interface {
	Find() (*[]Project, error)
	FindOneByKey(key string) (*Project, error)
	Create(payload *Project) (*Project, error)
	Update(payload *Project) error
}