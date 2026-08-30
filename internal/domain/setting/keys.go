package setting

import (
	"fmt"

	"github.com/su3i/wimp/internal/domain/notification"
)

// Keys are namespaced strings rather than an enum so a new setting is one constant, but
// they are still declared in one place so every stored key is greppable.
const (
	// KeyAlertingEnabled is the master switch for outbound delivery. Off means alerts are
	// still recorded and still drive incidents - only the hand-off to Alertmanager stops.
	KeyAlertingEnabled = "alerting.enabled"
	// KeyReceiverMinSeverity is the floor an alert must clear to be forwarded outbound.
	KeyReceiverMinSeverity = "alerting.receiver_min_severity"
	// KeyEnforceMfa requires every account to carry MFA, checked at login.
	KeyEnforceMfa = "security.enforce_mfa"
)

// SeverityKey names the override for one alert type's severity.
func SeverityKey(alertType notification.AlertType) string {
	return fmt.Sprintf("alert.severity.%s", alertType)
}
