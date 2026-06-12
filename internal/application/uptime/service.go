package uptime

import (
	"time"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/uptime"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func Record(machineID uint, event uptime.EventType, cfg *config.DatabaseConfig) error {
	return database.NewUptimeRepository(cfg).Record(machineID, event)
}

type MachineDayStats struct {
	MachineID uint
	Hostname  string
	Days      []uptime.DayStats
	// overall uptime % across the window
	UptimePct float64
}

// Calculate computes per-day uptime stats for a machine from its events.
// events must be sorted ASC by OccurredAt.
// prevStatus is the machine's last known status before the window (assumed offline if unknown).
func Calculate(machineID uint, hostname string, days int, events []uptime.Event) MachineDayStats {
	now := time.Now().UTC()
	windowStart := now.AddDate(0, 0, -days).Truncate(24 * time.Hour)

	var totalOnlineMinutes float64
	totalMinutes := float64(days * 24 * 60)

	dayStats := make([]uptime.DayStats, days)
	for i := range dayStats {
		day := windowStart.AddDate(0, 0, i)
		dayStats[i] = uptime.DayStats{
			Date:   day,
			Status: "no_data",
		}
	}

	if len(events) == 0 {
		return MachineDayStats{
			MachineID: machineID,
			Hostname:  hostname,
			Days:      dayStats,
			UptimePct: 0,
		}
	}

	// Determine status at start of window from events before it
	wasOnline := false
	for _, e := range events {
		if e.OccurredAt.Before(windowStart) {
			wasOnline = e.Event == uptime.EventOnline
		}
	}

	// Walk through each day
	for i := range dayStats {
		dayStart := windowStart.AddDate(0, 0, i)
		dayEnd := dayStart.Add(24 * time.Hour)

		onlineMinutes := 0.0
		incidentCount := 0
		currentlyOnline := wasOnline
		lastTransition := dayStart
		hasData := false

		for _, e := range events {
			if e.OccurredAt.Before(dayStart) {
				currentlyOnline = e.Event == uptime.EventOnline
				lastTransition = dayStart
				continue
			}
			if !e.OccurredAt.Before(dayEnd) {
				break
			}
			hasData = true
			if currentlyOnline {
				onlineMinutes += e.OccurredAt.Sub(lastTransition).Minutes()
			} else if e.Event == uptime.EventOnline {
				incidentCount++
			}
			currentlyOnline = e.Event == uptime.EventOnline
			lastTransition = e.OccurredAt
		}

		// Account for remainder of day
		if hasData || wasOnline {
			if currentlyOnline {
				end := dayEnd
				if end.After(now) {
					end = now
				}
				onlineMinutes += end.Sub(lastTransition).Minutes()
			}
		}

		if !hasData && !wasOnline {
			dayStats[i].Status = "no_data"
			continue
		}

		dayMins := 24 * 60.0
		if i == days-1 {
			dayMins = now.Sub(dayStart).Minutes()
		}
		downtimeMins := dayMins - onlineMinutes
		if downtimeMins < 0 {
			downtimeMins = 0
		}

		totalOnlineMinutes += onlineMinutes
		dayStats[i].DowntimeMinutes = int(downtimeMins)
		dayStats[i].IncidentCount = incidentCount

		pct := onlineMinutes / dayMins * 100
		switch {
		case pct >= 99:
			dayStats[i].Status = "ok"
		case pct >= 90:
			dayStats[i].Status = "warning"
		default:
			dayStats[i].Status = "critical"
		}

		// update wasOnline for next day
		for _, e := range events {
			if !e.OccurredAt.Before(dayEnd) {
				break
			}
			wasOnline = e.Event == uptime.EventOnline
		}
	}

	uptimePct := 0.0
	if totalMinutes > 0 {
		uptimePct = totalOnlineMinutes / totalMinutes * 100
	}

	return MachineDayStats{
		MachineID: machineID,
		Hostname:  hostname,
		Days:      dayStats,
		UptimePct: uptimePct,
	}
}

func GetAllMachineStats(days int, machines []struct{ ID uint; Hostname string }, cfg *config.DatabaseConfig) ([]MachineDayStats, error) {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -days)

	repo := database.NewUptimeRepository(cfg)
	allEvents, err := repo.FindAllInRange(start, now)
	if err != nil {
		return nil, err
	}

	// Group events by machine
	byMachine := make(map[uint][]uptime.Event)
	for _, e := range allEvents {
		byMachine[e.MachineID] = append(byMachine[e.MachineID], e)
	}

	result := make([]MachineDayStats, 0, len(machines))
	for _, m := range machines {
		stats := Calculate(m.ID, m.Hostname, days, byMachine[m.ID])
		result = append(result, stats)
	}
	return result, nil
}
