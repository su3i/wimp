package setting

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/setting"
)

// ErrValidation marks anything the caller got wrong, so the handler can answer 400 for a
// bad value and 500 for a database failure instead of conflating the two.
var (
	ErrValidation = errors.New("invalid setting")
	ErrUnknownKey = fmt.Errorf("%w: unknown setting", ErrValidation)
)

// Change is one edit. A nil Value means "reset to the deployment default" rather than
// "set to empty" - the two are different outcomes and the API has to be able to say both.
type Change struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// Apply validates and writes a batch of edits. Validation runs over the whole batch first,
// so a request containing one bad value changes nothing rather than applying half of it.
func Apply(changes []Change, cfg *config.DatabaseConfig) error {
	for _, ch := range changes {
		if err := validate(ch); err != nil {
			return err
		}
	}
	for _, ch := range changes {
		var err error
		if ch.Value == nil {
			err = Reset(ch.Key, cfg)
		} else {
			err = Set(ch.Key, *ch.Value, cfg)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// validate rejects anything that is not a known key, and anything whose value would not
// parse. Only keys listed here are writable - the settings API cannot be used to set
// arbitrary configuration, so infrastructure and credentials stay out of reach by
// construction rather than by the UI declining to show them.
func validate(ch Change) error {
	switch {
	// Note KeyEnforceMfa is deliberately absent: it is surfaced read-only until account
	// MFA enrolment exists. Enforcing it today would refuse the login of every account
	// without MFA and offer them no way to add it.
	case ch.Key == setting.KeyAlertingEnabled:
		if ch.Value == nil {
			return nil
		}
		if _, err := strconv.ParseBool(*ch.Value); err != nil {
			return fmt.Errorf("%w: %s must be true or false", ErrValidation, ch.Key)
		}
		return nil

	case ch.Key == setting.KeyReceiverMinSeverity:
		if ch.Value == nil {
			return nil
		}
		level, ok := ParseLevel(*ch.Value)
		if !ok {
			return fmt.Errorf("%w: %s is not a valid severity", ErrValidation, *ch.Value)
		}
		// Disabled is an alert-type opt-out; as a delivery floor it would mean nothing.
		if level == notification.LevelDisabled {
			return fmt.Errorf("%w: receiver minimum severity cannot be disabled - switch alerting off instead", ErrValidation)
		}
		return nil

	case isSeverityKey(ch.Key):
		if ch.Value == nil {
			return nil
		}
		if _, ok := ParseLevel(*ch.Value); !ok {
			return fmt.Errorf("%w: %s is not a valid severity", ErrValidation, *ch.Value)
		}
		return nil
	}

	return fmt.Errorf("%w %q", ErrUnknownKey, ch.Key)
}

// isSeverityKey reports whether the key names a real alert type's severity, checked
// against the registry so a typo cannot quietly write an override nothing will ever read.
func isSeverityKey(key string) bool {
	for alertType := range notification.AlertTypeRegistry {
		if setting.SeverityKey(alertType) == key {
			return true
		}
	}
	return false
}
