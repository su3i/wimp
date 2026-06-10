package repositories

import (
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/site"
)

type siteRepository struct {
	db *gorm.DB
}

func (r *siteRepository) FindByMachineAndAppPool(machineID uint, appPoolName string) (*[]site.Site, error) {
	var sites []site.Site
	if err := r.db.Where("machine_id = ? AND app_pool_name = ?", machineID, appPoolName).Find(&sites).Error; err != nil {
		return nil, err
	}
	return &sites, nil
}

func (r *siteRepository) FindByMachineID(machineID uint) (*[]site.Site, error) {
	var sites []site.Site
	if err := r.db.Where("machine_id = ?", machineID).Find(&sites).Error; err != nil {
		return nil, err
	}
	return &sites, nil
}

func (r *siteRepository) FindOneByID(id uint) (*site.Site, error) {
	var s site.Site
	if err := r.db.First(&s, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *siteRepository) SyncFromDiscovery(machineID uint, sites []site.Site) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing []site.Site
		if err := tx.Where("machine_id = ?", machineID).Find(&existing).Error; err != nil {
			return err
		}

		byName := make(map[string]*site.Site, len(existing))
		for i := range existing {
			byName[existing[i].Name] = &existing[i]
		}

		incomingNames := make(map[string]bool, len(sites))
		for _, s := range sites {
			incomingNames[s.Name] = true
			if ex, ok := byName[s.Name]; ok {
				ex.State = s.State
				ex.PhysicalPath = s.PhysicalPath
				ex.AppPoolName = s.AppPoolName
				ex.Bindings = s.Bindings
				if err := tx.Save(ex).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Create(&s).Error; err != nil {
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

func (r *siteRepository) SyncStates(machineID uint, runningNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&site.Site{}).Where("machine_id = ?", machineID).Update("state", "Stopped").Error; err != nil {
			return err
		}
		if len(runningNames) == 0 {
			return nil
		}
		return tx.Model(&site.Site{}).
			Where("machine_id = ? AND name IN ?", machineID, runningNames).
			Update("state", "Started").Error
	})
}

func NewSiteRepository(db *gorm.DB) site.SiteRepository {
	return &siteRepository{db: db}
}
