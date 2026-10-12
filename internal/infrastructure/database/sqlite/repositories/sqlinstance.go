package repositories

import (
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/sqlinstance"
)

type sqlInstanceRepository struct {
	db *gorm.DB
}

func (r *sqlInstanceRepository) FindByMachineID(machineID uint) (*[]sqlinstance.SqlInstance, error) {
	var instances []sqlinstance.SqlInstance
	if err := r.db.Where("machine_id = ?", machineID).Order("name ASC").Find(&instances).Error; err != nil {
		return nil, err
	}
	return &instances, nil
}

func (r *sqlInstanceRepository) Sync(machineID uint, instances []sqlinstance.SqlInstance) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing []sqlinstance.SqlInstance
		if err := tx.Where("machine_id = ?", machineID).Find(&existing).Error; err != nil {
			return err
		}

		byName := make(map[string]*sqlinstance.SqlInstance, len(existing))
		for i := range existing {
			byName[existing[i].Name] = &existing[i]
		}

		incoming := make(map[string]bool, len(instances))
		for _, in := range instances {
			incoming[in.Name] = true
			in.MachineID = machineID
			if ex, ok := byName[in.Name]; ok {
				in.Model = ex.Model
				if err := tx.Save(&in).Error; err != nil {
					return err
				}
			} else if err := tx.Create(&in).Error; err != nil {
				return err
			}
		}

		for _, ex := range existing {
			if !incoming[ex.Name] {
				if err := tx.Unscoped().Delete(&ex).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *sqlInstanceRepository) UpdateStates(machineID uint, instances []sqlinstance.SqlInstance) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, in := range instances {
			// Select so an empty owner (an instance that stopped being clustered) is written.
			err := tx.Model(&sqlinstance.SqlInstance{}).
				Where("machine_id = ? AND name = ?", machineID, in.Name).
				Select("state", "agent_state", "owner_node", "monitored", "monitor_error").
				Updates(sqlinstance.SqlInstance{State: in.State, AgentState: in.AgentState, OwnerNode: in.OwnerNode,
					Monitored: in.Monitored, MonitorError: in.MonitorError}).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *sqlInstanceRepository) DeleteByMachineID(machineID uint) error {
	return r.db.Unscoped().Where("machine_id = ?", machineID).Delete(&sqlinstance.SqlInstance{}).Error
}

func NewSqlInstanceRepository(db *gorm.DB) sqlinstance.SqlInstanceRepository {
	return &sqlInstanceRepository{db: db}
}
