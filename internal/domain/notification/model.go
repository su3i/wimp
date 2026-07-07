package notification

import (
	"time"

	"gorm.io/gorm"
)

type Level string
type Category string

const (
	LevelInfo     Level = "info"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
	LevelSev      Level = "sev"      // above Critical - drop-everything tier
	LevelDisabled Level = "disabled" // user opted out entirely - never even created

	CategoryMachine Category = "machine"
	CategoryIIS     Category = "iis"
	CategoryAppPool Category = "apppool"
	CategoryService Category = "service"
	CategorySidecar Category = "sidecar"
	CategoryMetrics Category = "metrics"
)

// levelRank gives an ordinal ranking for severity comparisons (e.g. the receiver
// minimum-severity cutoff). Disabled is intentionally not ranked - it's handled as a
// special "don't emit at all" case, never compared.
var levelRank = map[Level]int{
	LevelInfo:     0,
	LevelWarning:  1,
	LevelCritical: 2,
	LevelSev:      3,
}

// AtLeast reports whether this level is ordinally >= other (Info < Warning < Critical < Sev).
func (l Level) AtLeast(other Level) bool {
	return levelRank[l] >= levelRank[other]
}

type Notification struct {
	gorm.Model
	ProjectID uint     `gorm:"not null;index"`
	MachineID uint     `gorm:"not null;index"`
	Level     Level    `gorm:"type:text;not null"`
	Category  Category `gorm:"type:text;not null"`
	Title     string   `gorm:"not null"`
	Detail    string
	ReadAt    *time.Time
	// IsRepeat marks a reminder fired for a breach that's still ongoing (e.g. the
	// 15-minute Sev repeat reminder in internal/application/metrics/checker.go), as
	// opposed to the alert's initial firing. Excluded from incident-count metrics like
	// the dashboard's Sev Events card, which counts distinct incidents, not every reminder.
	IsRepeat bool `gorm:"not null;default:false"`
}
