package repositories

import (
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/site"
)

type siteRepository struct {
	db *gorm.DB
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

func (r *siteRepository) ReplaceAll(machineID uint, sites []site.Site) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("machine_id = ?", machineID).Delete(&site.Site{}).Error; err != nil {
			return err
		}
		if len(sites) == 0 {
			return nil
		}
		return tx.Create(&sites).Error
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
