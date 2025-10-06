package utils

import (
	"github.com/gin-gonic/gin"
	"time"
)

// timeWindowFromContext extracts 'from' and 'to' query params, defaults to last 1 minute.
func TimeWindowFromContext(c *gin.Context) (from time.Time, to time.Time, err error) {
	toStr := c.Query("to")
	fromStr := c.Query("from")

	if fromStr == "" && toStr == "" {
		to = time.Now().UTC()
		from = to.Add(-1 * time.Minute)
		return
	}
	// parse to
	if toStr == "" {
		to = time.Now().UTC()
	} else {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return
		}
	}
	// parse from
	if fromStr == "" {
		from = to.Add(-1 * time.Minute)
	} else {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return
		}
	}
	return
}
