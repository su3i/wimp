package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/machine"
)

type machineRepository struct {
	db *gorm.DB
}

func (r *machineRepository) FindByProjectID(projectID uint) (*[]machine.Machine, error) {
	var machines []machine.Machine

	if err := r.db.Where("project_id = ?", projectID).Find(&machines).Error; err != nil {
		return nil, err
	}

	return &machines, nil
}

func (r *machineRepository) FindOneByID(id uint) (*machine.Machine, error) {
	var m machine.Machine

	if err := r.db.First(&m, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &m, nil
}

func (r *machineRepository) FindOneByToken(token string) (*machine.Machine, error) {
	var m machine.Machine

	if err := r.db.Where("token = ?", token).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &m, nil
}

func (r *machineRepository) FindOneByHostname(hostname string) (*machine.Machine, error) {
	var m machine.Machine

	if err := r.db.Where("hostname = ?", hostname).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &m, nil
}

func (r *machineRepository) Create(payload *machine.Machine) (*machine.Machine, error) {
	m := machine.Machine{
		ProjectID:      payload.ProjectID,
		Hostname:       payload.Hostname,
		Status:         payload.Status,
		Token:          payload.Token,
		TokenExpiresAt: payload.TokenExpiresAt,
	}

	if err := r.db.Create(&m).Error; err != nil {
		return nil, errors.New("failed to create machine: " + err.Error())
	}

	return &m, nil
}

func (r *machineRepository) Update(payload *machine.Machine) error {
	if err := r.db.Save(payload).Error; err != nil {
		return errors.New("failed to update machine: " + err.Error())
	}

	return nil
}

func NewMachineRepository(db *gorm.DB) machine.MachineRepository {
	return &machineRepository{db: db}
}
