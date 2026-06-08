package project

type ProjectStatus string

const (
	Active  ProjectStatus = "ACTIVE"
	Paused ProjectStatus = "PAUSED"
	Archived ProjectStatus = "ARCHIVED"
)
