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

func (r *appPoolRepository) SyncFromDiscovery(machineID uint, pools []apppool.AppPool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing []apppool.AppPool
		if err := tx.Where("machine_id = ?", machineID).Find(&existing).Error; err != nil {
			return err
		}

		byName := make(map[string]*apppool.AppPool, len(existing))
		for i := range existing {
			byName[existing[i].Name] = &existing[i]
		}

		incomingNames := make(map[string]bool, len(pools))
		for _, p := range pools {
			incomingNames[p.Name] = true
			if ex, ok := byName[p.Name]; ok {
				ex.State = p.State
				ex.RuntimeVersion = p.RuntimeVersion
				ex.PipelineMode = p.PipelineMode
				ex.StartMode = p.StartMode
				ex.IdentityType = p.IdentityType
				if err := tx.Save(ex).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Create(&p).Error; err != nil {
					return err
				}
			}
		}

		for _, ex := range existing {
			if !incomingNames[ex.Name] {
				if err := tx.Delete(&ex).Error; err != nil {
					return err
				}
			}
		}
		return nil
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
