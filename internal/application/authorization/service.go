package authorization

import (
	"log"
	"strings"

	"github.com/casbin/casbin/v3"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/authorization"
)

var enforcer *casbin.Enforcer

func Initialize(casbinCfg *config.CasbinConfig) {
	var err error
	enforcer, err = casbin.NewEnforcer("./data/model.conf", "./data/policy.csv")
	if err != nil {
		log.Fatalf("failed to load Casbin enforcer: %v", err)
	}
	log.Print("Successfully initialized authorizer")

	// Optional: load policy from file
	// if err := enforcer.LoadPolicy(); err != nil {
	// 	log.Fatalf("failed to load Casbin policy: %v", err)
	// }
}

// EnforceRoles checks if **any** of the given roles can perform the action on object/domain
// roles: list of roles in format "ROLE__ENTITYKEY"
// domain: entity key to check against
// object: resource type, e.g., "organization" or "project"
// action: "read", "write", "admin"
// Returns true if any role allows, false otherwise
func EnforceRoles(roles []string, domain authorization.AuthorizationDomain, object authorization.AuthorizationObject, action string) (bool, error) {
	for _, role := range roles {
		return enforcer.Enforce(strings.SplitN(role, "__", 2)[0], string(domain), string(object), action)
	}
	// No role was successful
	return false, nil
}