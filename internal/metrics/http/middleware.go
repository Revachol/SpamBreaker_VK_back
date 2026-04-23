package httpmetric

import (
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Middleware(m HttpMetricsIface) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		path := middleware.NormalizePath(c.Request.URL.Path)
		status := c.Writer.Status()

		if !strings.HasPrefix(path, "/api/v1/") {
			return
		}
		m.IncHTTPRequest(c.Request.Method, path, status)
		m.ObserveHTTPDuration(c.Request.Method, path, status, duration)
	}
}
