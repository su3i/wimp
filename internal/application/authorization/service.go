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
}

// Each role is formatted "ROLE__ENTITYKEY"; only the ROLE part is passed to Casbin.
func EnforceRoles(roles []string, domain authorization.AuthorizationDomain, object authorization.AuthorizationObject, action string) (bool, error) {
	for _, role := range roles {
		return enforcer.Enforce(strings.SplitN(role, "__", 2)[0], string(domain), string(object), action)
	}
	return false, nil
}
