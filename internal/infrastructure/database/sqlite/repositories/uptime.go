package repositories

import (
	"time"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/uptime"
)

type uptimeRepository struct {
	db *gorm.DB
}

func (r *uptimeRepository) Record(machineID uint, event uptime.EventType) error {
	return r.db.Create(&uptime.Event{
		MachineID:  machineID,
		Event:      event,
		OccurredAt: time.Now().UTC(),
	}).Error
}

func (r *uptimeRepository) FindByMachineAndRange(machineID uint, start, end time.Time) ([]uptime.Event, error) {
	var events []uptime.Event
	err := r.db.Where("machine_id = ? AND occurred_at >= ? AND occurred_at <= ?", machineID, start, end).
		Order("occurred_at ASC").Find(&events).Error
	return events, err
}

func (r *uptimeRepository) FindAllInRange(start, end time.Time) ([]uptime.Event, error) {
	var events []uptime.Event
	err := r.db.Where("occurred_at >= ? AND occurred_at <= ?", start, end).
		Order("occurred_at ASC").Find(&events).Error
	return events, err
}

func NewUptimeRepository(db *gorm.DB) uptime.Repository {
	return &uptimeRepository{db: db}
}
