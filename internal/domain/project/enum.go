package project

type ProjectStatus string

const (
	Active   ProjectStatus = "ACTIVE"
	Paused   ProjectStatus = "PAUSED"
	Archived ProjectStatus = "ARCHIVED"
)

// DefaultKey is the project seeded at first boot. It is the destination the UI falls back
// to when the project a user was in is deleted, so it has to always exist - deleting it
// would leave an account with nowhere to land.
const DefaultKey = "default"
