package setting

type Repository interface {
	FindAll() ([]Setting, error)
	// Upsert writes an override, replacing any existing value for the key.
	Upsert(key, value string) error
	// Delete removes an override so the key falls back to its deployment default.
	Delete(key string) error
}
