// Package setting resolves operator-editable configuration.
//
// Reads go to an in-memory snapshot, not the database. Severity lookup happens on the path
// of every alert emitted, and heartbeats alone put that on a steady tick per host - a
// database round trip there would be a query per alert for a value that changes maybe once
// a month.
//
// The snapshot is a cache of the overrides table, not the source of truth. It is loaded at
// boot and rebuilt on every write, so a restart re-reads the operator's choices rather than
// silently reverting them to the deployment defaults.
package setting

import (
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/setting"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

var (
	mu        sync.RWMutex
	overrides = map[string]string{}
)

// Load reads every stored override into memory. Called once at startup, before the HTTP
// server accepts traffic.
func Load(cfg *config.DatabaseConfig) {
	stored, err := database.NewSettingRepository(cfg).FindAll()
	if err != nil {
		// Deliberately not fatal. Losing the overrides means falling back to the
		// deployment's own configuration, which is a working system - refusing to boot
		// over it would not be.
		log.Printf("settings: failed to load overrides, using deployment defaults: %v", err)
		return
	}

	next := make(map[string]string, len(stored))
	for _, s := range stored {
		next[s.Key] = s.Value
	}

	mu.Lock()
	overrides = next
	mu.Unlock()

	log.Printf("settings: loaded %d override(s)", len(next))
}

// Override returns the stored value for a key, and whether one exists at all. The bool is
// what lets the UI distinguish "set to warning by an operator" from "warning because that
// is what the chart says".
func Override(key string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := overrides[key]
	return v, ok
}

// Set stores an override and refreshes the snapshot.
func Set(key, value string, cfg *config.DatabaseConfig) error {
	if err := database.NewSettingRepository(cfg).Upsert(key, value); err != nil {
		return err
	}
	mu.Lock()
	overrides[key] = value
	mu.Unlock()
	return nil
}

// Reset drops an override so the key falls back to its deployment default.
func Reset(key string, cfg *config.DatabaseConfig) error {
	if err := database.NewSettingRepository(cfg).Delete(key); err != nil {
		return err
	}
	mu.Lock()
	delete(overrides, key)
	mu.Unlock()
	return nil
}

func boolOverride(key string, deploymentDefault bool) bool {
	if raw, ok := Override(key); ok {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			return parsed
		}
		log.Printf("settings: %s holds unparseable bool %q, using deployment default", key, raw)
	}
	return deploymentDefault
}

// ── Typed accessors ────────────────────────────────────────────────────────────
// Everything outside this package reads settings through these, so the key strings and
// the string-to-value parsing stay in one file.

// AlertingEnabled reports whether alerts are forwarded to Alertmanager. Defaults to on
// when an Alertmanager URL is configured at all - a deployment that wired one up did so
// intending to use it.
func AlertingEnabled() bool {
	return boolOverride(setting.KeyAlertingEnabled, config.Common().AlertmanagerUrl != "")
}

// EnforceMfa reports whether every account must carry MFA.
func EnforceMfa() bool {
	return boolOverride(setting.KeyEnforceMfa, config.Common().EnforceMfa)
}

// ReceiverMinSeverity is the floor an alert must clear to be forwarded outbound.
func ReceiverMinSeverity() (level notification.Level, overridden bool) {
	if raw, ok := Override(setting.KeyReceiverMinSeverity); ok {
		if parsed, valid := ParseLevel(raw); valid {
			return parsed, true
		}
		log.Printf("settings: %s holds invalid level %q, using deployment default", setting.KeyReceiverMinSeverity, raw)
	}
	if parsed, valid := ParseLevel(config.Alerts().ReceiverMinSeverity); valid {
		return parsed, false
	}
	return notification.LevelWarning, false
}

// SeverityFor resolves one alert type's effective severity, and reports whether that came
// from an operator override rather than from the deployment.
func SeverityFor(alertType notification.AlertType) (level notification.Level, overridden bool) {
	if raw, ok := Override(setting.SeverityKey(alertType)); ok {
		if parsed, valid := ParseLevel(raw); valid {
			return parsed, true
		}
		log.Printf("settings: severity override for %s is invalid (%q), using deployment default", alertType, raw)
	}
	return DeploymentSeverity(alertType), false
}

// DeploymentSeverity is what the alert type resolves to with no override in play: the
// environment's value if it sets one, otherwise the registry default.
func DeploymentSeverity(alertType notification.AlertType) notification.Level {
	if env := envSeverity(alertType); env != "" {
		if parsed, valid := ParseLevel(env); valid {
			return parsed
		}
	}
	if meta, ok := notification.AlertTypeRegistry[alertType]; ok {
		return meta.DefaultSeverity
	}
	return notification.LevelInfo
}

func ParseLevel(s string) (notification.Level, bool) {
	switch notification.Level(strings.ToLower(strings.TrimSpace(s))) {
	case notification.LevelInfo:
		return notification.LevelInfo, true
	case notification.LevelWarning:
		return notification.LevelWarning, true
	case notification.LevelCritical:
		return notification.LevelCritical, true
	case notification.LevelSev:
		return notification.LevelSev, true
	case notification.LevelDisabled:
		return notification.LevelDisabled, true
	default:
		return "", false
	}
}
