package repositories

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/machine"
)

type machineRepository struct {
	db *gorm.DB
}

func (r *machineRepository) FindAll() (*[]machine.Machine, error) {
	var machines []machine.Machine
	if err := r.db.Order("created_at DESC").Find(&machines).Error; err != nil {
		return nil, err
	}
	return &machines, nil
}

func (r *machineRepository) FindByProjectID(projectID uint) (*[]machine.Machine, error) {
	var machines []machine.Machine

	if err := r.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&machines).Error; err != nil {
		return nil, err
	}

	return &machines, nil
}

func (r *machineRepository) FindByProjectIDFiltered(projectID uint, page, perPage int, status string) (*[]machine.Machine, int64, error) {
	var machines []machine.Machine
	var total int64

	q := r.db.Model(&machine.Machine{}).Where("project_id = ?", projectID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	if err := q.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&machines).Error; err != nil {
		return nil, 0, err
	}
	return &machines, total, nil
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

func (r *machineRepository) FindPredecessors(excludeID uint, machineUID, hostname string) (*[]machine.Machine, error) {
	var machines []machine.Machine

	q := r.db.Where("id <> ?", excludeID).Where("status NOT IN ?", []machine.MachineStatus{machine.Reassigned, machine.Deleting})

	switch {
	case machineUID != "":
		// A UID match is definitive, so it also catches rows whose UID predates this
		// agent build (empty UID) but whose hostname still matches the same box.
		q = q.Where("machine_uid = ? OR (COALESCE(machine_uid, '') = '' AND hostname = ? AND hostname <> '')", machineUID, hostname)
	case hostname != "":
		q = q.Where("hostname = ?", hostname)
	default:
		return &[]machine.Machine{}, nil
	}

	if err := q.Find(&machines).Error; err != nil {
		return nil, err
	}
	return &machines, nil
}

func (r *machineRepository) MarkReassigned(id, supersededBy uint) error {
	now := time.Now()
	return r.db.Model(&machine.Machine{}).Where("id = ?", id).Updates(map[string]any{
		"status":           machine.Reassigned,
		"superseded_by_id": supersededBy,
		"reassigned_at":    now,
	}).Error
}

func (r *machineRepository) SetIdentity(id uint, hostname string, ips []string, agentVersion, windowsVersion, machineUID string) error {
	// Struct-with-Select rather than a map: IPs carries a json serializer tag, which GORM
	// only applies when the value comes through a struct field. Select is what still lets
	// a zero value (an agent reporting no IPs) be written.
	values := machine.Machine{Hostname: hostname, IPs: ips, AgentVersion: agentVersion}
	columns := []string{"hostname", "ips", "agent_version"}

	// Both are best-effort on the agent side and come back empty rather than wrong when
	// they can't be read, so an empty value must not overwrite a previously good one.
	if windowsVersion != "" {
		values.WindowsVersion = windowsVersion
		columns = append(columns, "windows_version")
	}
	if machineUID != "" {
		values.MachineUID = machineUID
		columns = append(columns, "machine_uid")
	}

	return r.db.Model(&machine.Machine{}).Where("id = ?", id).Select(columns).Updates(values).Error
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

func (r *machineRepository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&machine.Machine{}, id).Error
}

func NewMachineRepository(db *gorm.DB) machine.MachineRepository {
	return &machineRepository{db: db}
}
