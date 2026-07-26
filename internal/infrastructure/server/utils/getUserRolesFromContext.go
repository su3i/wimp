package utils

import "github.com/gin-gonic/gin"

func GetUserRolesFromContext(c *gin.Context) []string {
	raw, exists := c.Get("roles")
	if !exists {
		return nil
	}

	if roles, ok := raw.([]string); ok {
		return roles
	}

	// JWT claims decode as []interface{}, not []string.
	if rolesIface, ok := raw.([]interface{}); ok {
		roles := make([]string, 0, len(rolesIface))
		for _, r := range rolesIface {
			if s, ok := r.(string); ok {
				roles = append(roles, s)
			}
		}
		return roles
	}

	return nil
}
