package application

type ApplicationRepository interface {
	Create(app *Application) (*Application, error)
	FindByProjectID(projectID uint) (*[]Application, error)
	FindOneByID(id uint) (*Application, error)
	AddAppPool(rel *ApplicationAppPool) error
	RemoveAppPool(applicationID, appPoolID uint) error
	FindAppPoolRelations(applicationID uint) (*[]ApplicationAppPool, error)
	HasAppPool(applicationID, appPoolID uint) (bool, error)
	FindAppPoolRelation(applicationID, appPoolID uint) (*ApplicationAppPool, error)
	UpdateAppPoolRelation(rel *ApplicationAppPool) error
	FindAppPoolRelationsByPoolIDs(poolIDs []uint) (*[]ApplicationAppPool, error)
}
