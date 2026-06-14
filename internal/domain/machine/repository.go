package machine

type MachineRepository interface {
	FindAll() (*[]Machine, error)
	FindByProjectID(projectID uint) (*[]Machine, error)
	FindOneByID(id uint) (*Machine, error)
	FindOneByHostname(hostname string) (*Machine, error)
	FindOneByToken(token string) (*Machine, error)
	Create(payload *Machine) (*Machine, error)
	Update(payload *Machine) error
	Delete(id uint) error
}
