package account

type AccountRole string

const (
	SuperAdmin  AccountRole = "SUPERADMIN"
	Admin AccountRole = "ADMIN"
	Guest AccountRole = "GUEST"
)

type SecurityLevel string

const (
	SecurityExcellent SecurityLevel = "excellent"
	SecurityStrong    SecurityLevel = "strong"
	SecurityFair      SecurityLevel = "fair"
	SecurityWeak      SecurityLevel = "weak"
)