package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
	for _, el := range cfg.AllowedOrigins {
		if el == "*" {
			cfg.AllowedOrigins = []string{"*"}
			break
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if len(cfg.AllowedOrigins) > 0 && cfg.AllowedOrigins[0] == "*" {
			origin = "*"
		}
		for _, or := range cfg.AllowedOrigins {
			if or == origin {
				c.Header("Access-Control-Allow-Origin", origin)
				break
			}
		}

		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if len(cfg.AllowedMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
		}

		if len(cfg.AllowedHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
		}

		if len(cfg.ExposeHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		}
		if cfg.MaxAgeSeconds > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAgeSeconds))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
		}

		c.Next()
	}
}
