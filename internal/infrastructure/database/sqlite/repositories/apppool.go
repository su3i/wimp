package repositories

import (
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/apppool"
)

type appPoolRepository struct {
	db *gorm.DB
}

func (r *appPoolRepository) FindByMachineID(machineID uint) (*[]apppool.AppPool, error) {
	var pools []apppool.AppPool
	if err := r.db.Where("machine_id = ?", machineID).Find(&pools).Error; err != nil {
		return nil, err
	}
	return &pools, nil
}

func (r *appPoolRepository) FindOneByID(id uint) (*apppool.AppPool, error) {
	var pool apppool.AppPool
	if err := r.db.First(&pool, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &pool, nil
}

func (r *appPoolRepository) ReplaceAll(machineID uint, pools []apppool.AppPool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("machine_id = ?", machineID).Delete(&apppool.AppPool{}).Error; err != nil {
			return err
		}
		if len(pools) == 0 {
			return nil
		}
		return tx.Create(&pools).Error
	})
}

func (r *appPoolRepository) SyncStates(machineID uint, runningNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&apppool.AppPool{}).Where("machine_id = ?", machineID).Update("state", "Stopped").Error; err != nil {
			return err
		}
		if len(runningNames) == 0 {
			return nil
		}
		return tx.Model(&apppool.AppPool{}).
			Where("machine_id = ? AND name IN ?", machineID, runningNames).
			Update("state", "Started").Error
	})
}

func NewAppPoolRepository(db *gorm.DB) apppool.AppPoolRepository {
	return &appPoolRepository{db: db}
}
