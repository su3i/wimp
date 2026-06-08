package authorization

type AuthorizationObject string

const (
	Organization  AuthorizationObject = "organization"
	Project  AuthorizationObject = "project"
)

type AuthorizationDomain string

const (
	AuthorizationDomainOrg     AuthorizationDomain = "org"
	AuthorizationDomainProject AuthorizationDomain = "project"
)