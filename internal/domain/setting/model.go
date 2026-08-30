// Package setting holds operator-editable configuration.
//
// The deployment's environment (and therefore the Helm chart) remains the source of truth
// for everything nobody has touched. This table stores only the values an operator has
// explicitly changed in the UI, and those win at runtime.
//
// It is a key/value table rather than a column per setting on purpose: settings are added
// constantly, a typed row would need a migration every time, and nothing here is ever
// queried by anything other than key. The typed accessors in internal/application/setting
// are what keep that stringly-typed storage from leaking into the rest of the codebase.
package setting

import "gorm.io/gorm"

type Setting struct {
	gorm.Model

	Key   string `gorm:"uniqueIndex;not null"`
	Value string `gorm:"type:text;not null"`
}
