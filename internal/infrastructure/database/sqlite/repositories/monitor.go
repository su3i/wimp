package repositories

import (
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/monitor"
)

type monitorRepository struct {
	db *gorm.DB
}

func (r *monitorRepository) Create(m *monitor.Monitor) (*monitor.Monitor, error) {
	if err := r.db.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

func (r *monitorRepository) FindByProjectKey(projectKey string) ([]monitor.Monitor, error) {
	var monitors []monitor.Monitor
	err := r.db.
		Joins("JOIN projects ON projects.id = monitors.project_id AND projects.deleted_at IS NULL").
		Where("projects.key = ? AND monitors.deleted_at IS NULL", projectKey).
		Find(&monitors).Error
	return monitors, err
}

func (r *monitorRepository) FindAll() ([]monitor.Monitor, error) {
	var monitors []monitor.Monitor
	err := r.db.Where("deleted_at IS NULL").Find(&monitors).Error
	return monitors, err
}

func (r *monitorRepository) FindByID(id uint) (*monitor.Monitor, error) {
	var m monitor.Monitor
	if err := r.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *monitorRepository) Update(m *monitor.Monitor) error {
	return r.db.Save(m).Error
}

func (r *monitorRepository) Delete(id uint) error {
	return r.db.Delete(&monitor.Monitor{}, id).Error
}

func (r *monitorRepository) UpdateCheckState(id uint, consecutiveFailures int, alertFired bool) error {
	return r.db.Model(&monitor.Monitor{}).Where("id = ?", id).Updates(map[string]interface{}{
		"consecutive_failures": consecutiveFailures,
		"alert_fired":          alertFired,
	}).Error
}

func NewMonitorRepository(db *gorm.DB) monitor.Repository {
	return &monitorRepository{db: db}
}
