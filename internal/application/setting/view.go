package setting

import (
	"sort"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/setting"
)

// AlertSeverityView is one alert type as the settings page needs it: what it resolves to
// now, what it would resolve to with no override, and whether an operator has changed it.
type AlertSeverityView struct {
	AlertType  string `json:"alert_type"`
	Category   string `json:"category"`
	Level      string `json:"level"`
	Default    string `json:"default"`
	Overridden bool   `json:"overridden"`
}

type ToggleView struct {
	Value      bool `json:"value"`
	Default    bool `json:"default"`
	Overridden bool `json:"overridden"`
}

type LevelView struct {
	Value      string `json:"value"`
	Default    string `json:"default"`
	Overridden bool   `json:"overridden"`
}

// AgentView is read-only. Agent version and fleet auto-update change what every host in
// the estate downloads and runs, which is a deployment decision rather than a toggle.
type AgentView struct {
	Version     string `json:"version"`
	AutoUpdate  bool   `json:"auto_update"`
	ReleaseBase string `json:"release_base"`
}

type View struct {
	AlertingEnabled     ToggleView          `json:"alerting_enabled"`
	ReceiverMinSeverity LevelView           `json:"receiver_min_severity"`
	EnforceMfa          ToggleView          `json:"enforce_mfa"`
	AlertSeverities     []AlertSeverityView `json:"alert_severities"`
	Agent               AgentView           `json:"agent"`
}

// Levels is the set a severity may be set to, in ascending order of urgency. Disabled is
// last because it is not a severity so much as an opt-out.
var Levels = []string{
	string(notification.LevelInfo),
	string(notification.LevelWarning),
	string(notification.LevelCritical),
	string(notification.LevelSev),
	string(notification.LevelDisabled),
}

func Current() View {
	common := config.Common()

	severities := make([]AlertSeverityView, 0, len(notification.AlertTypeRegistry))
	for alertType, meta := range notification.AlertTypeRegistry {
		level, overridden := SeverityFor(alertType)
		severities = append(severities, AlertSeverityView{
			AlertType:  string(alertType),
			Category:   string(meta.Category),
			Level:      string(level),
			Default:    string(DeploymentSeverity(alertType)),
			Overridden: overridden,
		})
	}
	// Map iteration order is random; the page would otherwise reshuffle on every poll.
	sort.Slice(severities, func(i, j int) bool {
		if severities[i].Category != severities[j].Category {
			return severities[i].Category < severities[j].Category
		}
		return severities[i].AlertType < severities[j].AlertType
	})

	_, minOverridden := ReceiverMinSeverity()
	minLevel, _ := ReceiverMinSeverity()
	deploymentMin, _ := ParseLevel(config.Alerts().ReceiverMinSeverity)
	if deploymentMin == "" {
		deploymentMin = notification.LevelWarning
	}

	_, alertingOverridden := Override(setting.KeyAlertingEnabled)
	_, mfaOverridden := Override(setting.KeyEnforceMfa)

	return View{
		AlertingEnabled: ToggleView{
			Value:      AlertingEnabled(),
			Default:    common.AlertmanagerUrl != "",
			Overridden: alertingOverridden,
		},
		ReceiverMinSeverity: LevelView{
			Value:      string(minLevel),
			Default:    string(deploymentMin),
			Overridden: minOverridden,
		},
		EnforceMfa: ToggleView{
			Value:      EnforceMfa(),
			Default:    common.EnforceMfa,
			Overridden: mfaOverridden,
		},
		AlertSeverities: severities,
		Agent: AgentView{
			Version:     common.AgentVersion,
			AutoUpdate:  common.AutoUpdateAgent,
			ReleaseBase: config.AgentReleaseBaseURL,
		},
	}
}
