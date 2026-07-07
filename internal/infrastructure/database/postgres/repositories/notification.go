package repositories

import (
	"time"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/notification"
)

type notificationRepository struct {
	db *gorm.DB
}

func (r *notificationRepository) Create(n *notification.Notification) (*notification.Notification, error) {
	if err := r.db.Create(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

func (r *notificationRepository) FindPaginated(f notification.Filter) ([]notification.Notification, int64, error) {
	q := r.db.Model(&notification.Notification{})
	if f.Level != "" {
		q = q.Where("level = ?", f.Level)
	}
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if f.MachineID != nil {
		q = q.Where("machine_id = ?", *f.MachineID)
	}
	if f.ProjectKey != nil {
		q = q.Where("project_id = (SELECT id FROM projects WHERE key = ?)", *f.ProjectKey)
	}
	if f.UnreadOnly {
		q = q.Where("read_at IS NULL")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var results []notification.Notification
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&results).Error; err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func (r *notificationRepository) UnreadCount() (int64, error) {
	var count int64
	err := r.db.Model(&notification.Notification{}).Where("read_at IS NULL").Count(&count).Error
	return count, err
}

func (r *notificationRepository) MarkRead(id uint) error {
	now := time.Now()
	return r.db.Model(&notification.Notification{}).Where("id = ?", id).Update("read_at", now).Error
}

func (r *notificationRepository) MarkAllRead() error {
	now := time.Now()
	return r.db.Model(&notification.Notification{}).Where("read_at IS NULL").Update("read_at", now).Error
}

func (r *notificationRepository) FindActiveAlerts() ([]notification.Notification, error) {
	var results []notification.Notification
	err := r.db.Where("level IN (?, ?) AND read_at IS NULL", notification.LevelCritical, notification.LevelSev).
		Order("created_at DESC").
		Find(&results).Error
	return results, err
}

func (r *notificationRepository) CountByHour(hours int) ([]notification.HourCount, error) {
	type row struct {
		Hour  time.Time
		Count int64
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	var rows []row
	err := r.db.Raw(`
		SELECT date_trunc('hour', created_at) AS hour, COUNT(*) AS count
		FROM notifications
		WHERE created_at >= ? AND deleted_at IS NULL
		GROUP BY hour
		ORDER BY hour ASC
	`, since).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]notification.HourCount, len(rows))
	for i, r := range rows {
		result[i] = notification.HourCount{Hour: r.Hour, Count: r.Count}
	}
	return result, nil
}

func (r *notificationRepository) CountSevSince(since time.Time, projectKey string) (int64, error) {
	var count int64
	err := r.db.Model(&notification.Notification{}).
		Where("level = ? AND is_repeat = ? AND created_at >= ? AND deleted_at IS NULL", notification.LevelSev, false, since).
		Where("project_id = (SELECT id FROM projects WHERE key = ?)", projectKey).
		Count(&count).Error
	return count, err
}

func NewNotificationRepository(db *gorm.DB) notification.Repository {
	return &notificationRepository{db: db}
}
