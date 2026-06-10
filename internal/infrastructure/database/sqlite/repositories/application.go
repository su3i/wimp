package repositories

import (
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/application"
)

type applicationRepository struct {
	db *gorm.DB
}

func (r *applicationRepository) Create(app *application.Application) (*application.Application, error) {
	if err := r.db.Create(app).Error; err != nil {
		return nil, err
	}
	return app, nil
}

func (r *applicationRepository) FindByProjectID(projectID uint) (*[]application.Application, error) {
	var apps []application.Application
	if err := r.db.Where("project_id = ?", projectID).Find(&apps).Error; err != nil {
		return nil, err
	}
	return &apps, nil
}

func (r *applicationRepository) FindOneByID(id uint) (*application.Application, error) {
	var app application.Application
	if err := r.db.First(&app, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &app, nil
}

func (r *applicationRepository) AddAppPool(rel *application.ApplicationAppPool) error {
	return r.db.Create(rel).Error
}

func (r *applicationRepository) FindAppPoolRelations(applicationID uint) (*[]application.ApplicationAppPool, error) {
	var relations []application.ApplicationAppPool
	if err := r.db.Where("application_id = ?", applicationID).Find(&relations).Error; err != nil {
		return nil, err
	}
	return &relations, nil
}

func (r *applicationRepository) HasAppPool(applicationID, appPoolID uint) (bool, error) {
	var count int64
	err := r.db.Model(&application.ApplicationAppPool{}).
		Where("application_id = ? AND app_pool_id = ?", applicationID, appPoolID).
		Count(&count).Error
	return count > 0, err
}

func NewApplicationRepository(db *gorm.DB) application.ApplicationRepository {
	return &applicationRepository{db: db}
}
