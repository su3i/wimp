package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/incident"
	"github.com/su3i/wimp/internal/domain/notification"
)

type incidentRepository struct {
	db *gorm.DB
}

func (r *incidentRepository) Create(i *incident.Incident) (*incident.Incident, error) {
	if err := r.db.Create(i).Error; err != nil {
		return nil, errors.New("failed to create incident: " + err.Error())
	}
	return i, nil
}

func (r *incidentRepository) FindOpen(projectID, machineID uint, kind notification.IncidentKind, subject string) (*incident.Incident, error) {
	var found incident.Incident
	err := r.db.
		Where("project_id = ? AND machine_id = ? AND kind = ? AND subject = ? AND status = ?",
			projectID, machineID, kind, subject, incident.StatusOpen).
		// Newest first: if a previous resolve was ever missed, the most recent failure is
		// the one this recovery actually refers to.
		Order("started_at DESC").
		First(&found).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &found, nil
}

func (r *incidentRepository) Resolve(id uint, resolvedAt any, title, detail string, notificationID uint) error {
	updates := map[string]any{
		"status":          incident.StatusResolved,
		"resolved_at":     resolvedAt,
		"resolved_title":  title,
		"resolved_detail": detail,
	}
	if notificationID != 0 {
		updates["resolved_notification_id"] = notificationID
	}
	// Guarded on status so a duplicate recovery cannot overwrite the resolution time of an
	// incident that is already closed.
	return r.db.Model(&incident.Incident{}).
		Where("id = ? AND status = ?", id, incident.StatusOpen).
		Updates(updates).Error
}

func (r *incidentRepository) FindPaginated(f incident.Filter) ([]incident.Incident, int64, error) {
	var incidents []incident.Incident
	var total int64

	q := r.db.Model(&incident.Incident{}).Where("project_id = ?", f.ProjectID)
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.PerPage
	if err := q.Order("started_at DESC").Offset(offset).Limit(f.PerPage).Find(&incidents).Error; err != nil {
		return nil, 0, err
	}
	return incidents, total, nil
}

func (r *incidentRepository) CountByStatus(projectID uint) (incident.Counts, error) {
	var rows []struct {
		Status incident.Status
		Count  int64
	}
	if err := r.db.Model(&incident.Incident{}).
		Select("status, count(*) as count").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return incident.Counts{}, err
	}

	var counts incident.Counts
	for _, row := range rows {
		switch row.Status {
		case incident.StatusOpen:
			counts.Open = row.Count
		case incident.StatusResolved:
			counts.Resolved = row.Count
		}
	}
	return counts, nil
}

func NewIncidentRepository(db *gorm.DB) incident.Repository {
	return &incidentRepository{db: db}
}
