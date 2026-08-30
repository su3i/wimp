package setting

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	settingDomain "github.com/su3i/wimp/internal/domain/setting"
	sqliteDB "github.com/su3i/wimp/internal/infrastructure/database/sqlite"
)

// The point of this layer is that an operator's choice outranks the deployment's, survives
// a restart, and cannot be used to write anything that was never meant to be editable.

func setup(t *testing.T) *config.DatabaseConfig {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:settings?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&settingDomain.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatalf("reset table: %v", err)
	}
	sqliteDB.DB = db

	// The env layer these fall back to. Only the values envconfig marks required are set,
	// and none of the optional ones - so every setting under test falls through to its
	// registry default and the tests are exercising the fallback chain, not the env.
	for k, v := range map[string]string{
		"APPENV":               "test",
		"APPURL":               "http://localhost",
		"APPPORT":              "8080",
		"LOKIHOST":             "loki",
		"LOKIPORT":             "3100",
		"BOOTSTRAPTOKEN":       "token",
		"AGENTVERSION":         "1.0.0",
		"JWTSECRET":            "secret",
		"DEFAULTADMINUSERNAME": "admin",
		"DEFAULTADMINPASSWORD": "password",
	} {
		t.Setenv(k, v)
	}
	config.Initialize()

	mu.Lock()
	overrides = map[string]string{}
	mu.Unlock()

	return &config.DatabaseConfig{}
}

func ptr(s string) *string { return &s }

func TestOverrideBeatsDeploymentDefault(t *testing.T) {
	cfg := setup(t)

	// The registry has high_cpu at warning.
	if got := DeploymentSeverity(notification.AlertHighCPU); got != notification.LevelWarning {
		t.Fatalf("deployment default = %q, want warning", got)
	}
	if level, overridden := SeverityFor(notification.AlertHighCPU); level != notification.LevelWarning || overridden {
		t.Fatalf("with no override: level %q overridden %v, want warning/false", level, overridden)
	}

	key := settingDomain.SeverityKey(notification.AlertHighCPU)
	if err := Apply([]Change{{Key: key, Value: ptr("sev")}}, cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}

	level, overridden := SeverityFor(notification.AlertHighCPU)
	if level != notification.LevelSev || !overridden {
		t.Fatalf("after override: level %q overridden %v, want sev/true", level, overridden)
	}
	// The deployment's own value is unchanged, which is what the reset affordance restores.
	if got := DeploymentSeverity(notification.AlertHighCPU); got != notification.LevelWarning {
		t.Fatalf("deployment default should be untouched, got %q", got)
	}
}

func TestOverridesSurviveAProcessRestart(t *testing.T) {
	cfg := setup(t)

	key := settingDomain.SeverityKey(notification.AlertLowDisk)
	if err := Apply([]Change{{Key: key, Value: ptr("info")}}, cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Simulate a restart: the in-memory snapshot is gone, only the table remains.
	mu.Lock()
	overrides = map[string]string{}
	mu.Unlock()
	if level, _ := SeverityFor(notification.AlertLowDisk); level == notification.LevelInfo {
		t.Fatal("test is not exercising the reload - the value survived without one")
	}

	Load(cfg)

	if level, overridden := SeverityFor(notification.AlertLowDisk); level != notification.LevelInfo || !overridden {
		t.Fatalf("after reload: level %q overridden %v, want info/true", level, overridden)
	}
}

func TestResetFallsBackToDeploymentDefault(t *testing.T) {
	cfg := setup(t)

	key := settingDomain.SeverityKey(notification.AlertSiteStopped)
	if err := Apply([]Change{{Key: key, Value: ptr("sev")}}, cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := Apply([]Change{{Key: key, Value: nil}}, cfg); err != nil {
		t.Fatalf("reset: %v", err)
	}

	level, overridden := SeverityFor(notification.AlertSiteStopped)
	if overridden {
		t.Fatal("value should no longer be marked as overridden")
	}
	if level != DeploymentSeverity(notification.AlertSiteStopped) {
		t.Fatalf("level = %q, want the deployment default %q", level, DeploymentSeverity(notification.AlertSiteStopped))
	}
}

func TestAlertingToggleRoundTrips(t *testing.T) {
	cfg := setup(t)

	// No Alertmanager URL in this environment, so outbound alerting defaults off.
	if AlertingEnabled() {
		t.Fatal("alerting should default off with no alertmanager configured")
	}

	if err := Apply([]Change{{Key: settingDomain.KeyAlertingEnabled, Value: ptr("true")}}, cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !AlertingEnabled() {
		t.Fatal("alerting override did not take effect")
	}
}

func TestEnforceMfaIsReadOnly(t *testing.T) {
	cfg := setup(t)

	// Read-only until account MFA enrolment exists. Writable here, it would lock out every
	// account without MFA and give them no way to add it - so the API refuses the key
	// rather than relying on the settings page not to offer it.
	err := Apply([]Change{{Key: settingDomain.KeyEnforceMfa, Value: ptr("true")}}, cfg)
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("error = %v, want ErrUnknownKey", err)
	}
	if EnforceMfa() {
		t.Fatal("mfa enforcement must still follow the deployment value")
	}
}

func TestUnknownAndUneditableKeysAreRejected(t *testing.T) {
	cfg := setup(t)

	// Nothing outside the writable set can be reached through this API - credentials and
	// infrastructure are not merely hidden by the UI, they are unwritable here.
	for _, key := range []string{
		"database.password",
		"common.jwt_secret",
		"alert.severity.not_a_real_alert",
		"",
	} {
		err := Apply([]Change{{Key: key, Value: ptr("x")}}, cfg)
		if !errors.Is(err, ErrUnknownKey) {
			t.Fatalf("key %q: error = %v, want ErrUnknownKey", key, err)
		}
	}
}

func TestInvalidValuesAreRejectedAndNothingIsWritten(t *testing.T) {
	cfg := setup(t)

	validKey := settingDomain.SeverityKey(notification.AlertHighCPU)

	// A batch with one bad entry must not apply the good one either.
	err := Apply([]Change{
		{Key: validKey, Value: ptr("sev")},
		{Key: settingDomain.KeyAlertingEnabled, Value: ptr("maybe")},
	}, cfg)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}
	if _, overridden := SeverityFor(notification.AlertHighCPU); overridden {
		t.Fatal("a rejected batch must not have applied its valid entries")
	}

	// Disabled is an alert-type opt-out, not a delivery floor.
	err = Apply([]Change{{Key: settingDomain.KeyReceiverMinSeverity, Value: ptr("disabled")}}, cfg)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation for a disabled delivery floor", err)
	}
}

func TestCurrentReportsEveryAlertTypeWithoutLeakingSecrets(t *testing.T) {
	setup(t)

	view := Current()
	if len(view.AlertSeverities) != len(notification.AlertTypeRegistry) {
		t.Fatalf("view has %d severities, want one per registered alert type (%d)",
			len(view.AlertSeverities), len(notification.AlertTypeRegistry))
	}
	// Stable ordering, or the page reshuffles on every poll.
	for i := 1; i < len(view.AlertSeverities); i++ {
		prev, cur := view.AlertSeverities[i-1], view.AlertSeverities[i]
		if prev.Category > cur.Category {
			t.Fatalf("severities are not ordered by category: %q before %q", prev.Category, cur.Category)
		}
	}
	// Service status says whether things work, never what they are.
	for _, svc := range view.Services {
		if svc.Detail != "not configured" && svc.Detail != "reachable" && svc.Detail != "unreachable" && svc.Detail != "unhealthy" {
			t.Fatalf("service %q leaked detail %q", svc.Name, svc.Detail)
		}
	}
}
