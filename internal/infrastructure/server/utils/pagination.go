package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const defaultPerPage = 20
const maxPerPage = 100

func ParsePageQuery(c *gin.Context) (page int, perPage int, status string) {
	page = 1
	perPage = defaultPerPage

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 {
			if v > maxPerPage {
				v = maxPerPage
			}
			perPage = v
		}
	}
	status = c.Query("status")
	return
}
