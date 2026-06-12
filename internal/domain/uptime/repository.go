package uptime

import "time"

type DayStats struct {
	Date            time.Time
	Status          string // ok, warning, critical, no_data
	IncidentCount   int
	DowntimeMinutes int
}

type Repository interface {
	Record(machineID uint, event EventType) error
	FindByMachineAndRange(machineID uint, start, end time.Time) ([]Event, error)
	FindAllInRange(start, end time.Time) ([]Event, error)
}
