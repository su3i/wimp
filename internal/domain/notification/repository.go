package notification

import "time"

type Filter struct {
	Level      string
	Category   string
	MachineID  *uint
	ProjectKey *string
	UnreadOnly bool
	Page       int
	Limit      int
}

type HourCount struct {
	Hour  time.Time
	Count int64
}

type Repository interface {
	Create(n *Notification) (*Notification, error)
	FindPaginated(f Filter) ([]Notification, int64, error)
	UnreadCount() (int64, error)
	MarkRead(id uint) error
	MarkAllRead() error
	FindActiveAlerts() ([]Notification, error)
	CountByHour(hours int) ([]HourCount, error)
	CountCriticalSince(since time.Time, projectKey string) (int64, error)
}
