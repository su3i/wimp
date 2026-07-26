package repositories

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	if err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&apps).Error; err != nil {
		return nil, err
	}
	return &apps, nil
}

func (r *applicationRepository) FindByProjectIDPaginated(projectID uint, page, perPage int) (*[]application.Application, int64, error) {
	var apps []application.Application
	var total int64

	q := r.db.Model(&application.Application{}).Where("project_id = ?", projectID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	if err := q.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&apps).Error; err != nil {
		return nil, 0, err
	}
	return &apps, total, nil
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

func (r *applicationRepository) FindAppPoolRelationsPaginated(applicationID uint, page, perPage int, state string) (*[]application.ApplicationAppPool, int64, error) {
	var relations []application.ApplicationAppPool
	var total int64

	q := r.db.Model(&application.ApplicationAppPool{}).
		Joins("JOIN app_pools ON app_pools.id = application_app_pools.app_pool_id AND app_pools.deleted_at IS NULL").
		Where("application_app_pools.application_id = ?", applicationID)
	if state != "" {
		q = q.Where("app_pools.state = ?", state)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	if err := q.Order("application_app_pools.created_at DESC").Offset(offset).Limit(perPage).Find(&relations).Error; err != nil {
		return nil, 0, err
	}
	return &relations, total, nil
}

func (r *applicationRepository) FindPoolCountsByApplicationIDs(appIDs []uint) (map[uint]application.PoolCounts, error) {
	if len(appIDs) == 0 {
		return map[uint]application.PoolCounts{}, nil
	}
	type row struct {
		ApplicationID uint
		Total         int64
		Healthy       int64
	}
	var rows []row
	// A pool only counts as healthy if its owning machine is currently online - a
	// "Started" state on a disconnected machine is just stale last-known state, not a
	// live health signal.
	err := r.db.Raw(`
		SELECT aap.application_id,
		       COUNT(ap.id)                                                                                   AS total,
		       SUM(CASE WHEN ap.state = 'Started' AND m.status = 'online' THEN 1 ELSE 0 END)                 AS healthy
		FROM application_app_pools aap
		JOIN app_pools ap ON ap.id = aap.app_pool_id AND ap.deleted_at IS NULL
		JOIN machines m ON m.id = ap.machine_id AND m.deleted_at IS NULL
		WHERE aap.application_id IN ?
		GROUP BY aap.application_id
	`, appIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]application.PoolCounts, len(rows))
	for _, r := range rows {
		out[r.ApplicationID] = application.PoolCounts{Total: r.Total, Healthy: r.Healthy}
	}
	return out, nil
}


func (r *applicationRepository) AddAppPool(rel *application.ApplicationAppPool) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(rel).Error
}

func (r *applicationRepository) FindAppPoolRelations(applicationID uint) (*[]application.ApplicationAppPool, error) {
	var relations []application.ApplicationAppPool
	if err := r.db.Where("application_id = ?", applicationID).Order("created_at DESC").Find(&relations).Error; err != nil {
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

func (r *applicationRepository) FindAppPoolRelation(applicationID, appPoolID uint) (*application.ApplicationAppPool, error) {
	var rel application.ApplicationAppPool
	err := r.db.Where("application_id = ? AND app_pool_id = ?", applicationID, appPoolID).First(&rel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rel, nil
}

func (r *applicationRepository) RemoveAppPool(applicationID, appPoolID uint) error {
	return r.db.Unscoped().Where("application_id = ? AND app_pool_id = ?", applicationID, appPoolID).Delete(&application.ApplicationAppPool{}).Error
}

func (r *applicationRepository) UpdateAppPoolRelation(rel *application.ApplicationAppPool) error {
	return r.db.Save(rel).Error
}

func (r *applicationRepository) FindAppPoolRelationsByPoolIDs(poolIDs []uint) (*[]application.ApplicationAppPool, error) {
	var relations []application.ApplicationAppPool
	if len(poolIDs) == 0 {
		return &relations, nil
	}
	if err := r.db.Where("app_pool_id IN ?", poolIDs).Find(&relations).Error; err != nil {
		return nil, err
	}
	return &relations, nil
}

func (r *applicationRepository) Update(app *application.Application) error {
	return r.db.Model(app).Select("name", "health_check_url", "health_check_interval_seconds").Updates(app).Error
}

func (r *applicationRepository) UpdateCheckState(id uint, consecutiveFailures int, alertFired bool) error {
	return r.db.Model(&application.Application{}).Where("id = ?", id).Updates(map[string]interface{}{
		"consecutive_failures": consecutiveFailures,
		"alert_fired":          alertFired,
	}).Error
}

func (r *applicationRepository) FindAllWithHealthCheck() ([]application.Application, error) {
	var apps []application.Application
	if err := r.db.Where("health_check_url IS NOT NULL AND health_check_url != ''").Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

func (r *applicationRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("application_id = ?", id).Delete(&application.ApplicationAppPool{}).Error; err != nil {
			return err
		}
		return tx.Delete(&application.Application{}, id).Error
	})
}

func NewApplicationRepository(db *gorm.DB) application.ApplicationRepository {
	return &applicationRepository{db: db}
}
