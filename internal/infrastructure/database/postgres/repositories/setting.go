package repositories

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/su3i/wimp/internal/domain/setting"
)

type settingRepository struct {
	db *gorm.DB
}

func (r *settingRepository) FindAll() ([]setting.Setting, error) {
	var settings []setting.Setting
	if err := r.db.Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *settingRepository) Upsert(key, value string) error {
	// One statement rather than read-then-write: two operators saving at once would
	// otherwise race, and the loser's insert would fail on the unique index.
	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&setting.Setting{Key: key, Value: value}).Error
	if err != nil {
		return errors.New("failed to save setting: " + err.Error())
	}
	return nil
}

func (r *settingRepository) Delete(key string) error {
	return r.db.Unscoped().Where("key = ?", key).Delete(&setting.Setting{}).Error
}

func NewSettingRepository(db *gorm.DB) setting.Repository {
	return &settingRepository{db: db}
}
