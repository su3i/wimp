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
	UpdateSlowState(id uint, consecutiveSlow int, slowAlertFired bool) error
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

	AddSqlInstance(rel *ApplicationSqlInstance) error
	RemoveSqlInstance(applicationID, sqlInstanceID uint) error
	FindSqlInstanceRelations(applicationID uint) (*[]ApplicationSqlInstance, error)
	// FindSqlCountsByApplicationIDs counts member instances and how many are running on an
	// online host. A clustered instance counts as running if it runs on any of its nodes.
	FindSqlCountsByApplicationIDs(appIDs []uint) (map[uint]PoolCounts, error)
}
