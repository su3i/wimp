package application

type PoolCounts struct {
	Total   int64
	Healthy int64
}

type ApplicationRepository interface {
	Create(app *Application) (*Application, error)
	FindByProjectID(projectID uint) (*[]Application, error)
	FindByProjectIDPaginated(projectID uint, page, perPage int) (*[]Application, int64, error)
	FindPoolCountsByApplicationIDs(appIDs []uint) (map[uint]PoolCounts, error)
	FindOneByID(id uint) (*Application, error)
	Update(app *Application) error
	UpdateCheckState(id uint, consecutiveFailures int, alertFired bool) error
	FindAllWithHealthCheck() ([]Application, error)
	AddAppPool(rel *ApplicationAppPool) error
	RemoveAppPool(applicationID, appPoolID uint) error
	FindAppPoolRelations(applicationID uint) (*[]ApplicationAppPool, error)
	FindAppPoolRelationsPaginated(applicationID uint, page, perPage int, state string) (*[]ApplicationAppPool, int64, error)
	HasAppPool(applicationID, appPoolID uint) (bool, error)
	FindAppPoolRelation(applicationID, appPoolID uint) (*ApplicationAppPool, error)
	UpdateAppPoolRelation(rel *ApplicationAppPool) error
	FindAppPoolRelationsByPoolIDs(poolIDs []uint) (*[]ApplicationAppPool, error)
	Delete(id uint) error
}
