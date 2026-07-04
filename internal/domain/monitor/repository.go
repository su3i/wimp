package monitor

type Repository interface {
	Create(m *Monitor) (*Monitor, error)
	FindByProjectKey(projectKey string) ([]Monitor, error)
	FindAll() ([]Monitor, error)
	FindByID(id uint) (*Monitor, error)
	Update(m *Monitor) error
	Delete(id uint) error
	UpdateCheckState(id uint, consecutiveFailures int, alertFired bool) error
}
