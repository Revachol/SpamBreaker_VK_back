package httphandler

import (
	"net/http"
	"time"

	middlewaremetric "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/middleware"
	prometheusmetric "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/prometheus"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter собирает gin.Engine с маршрутами и middleware.
func NewRouter(reg *prometheus.Registry, h *Handler) *gin.Engine {
	r := gin.New()

	coll := prometheusmetric.NewPrometheusHttpMetrics("Core", reg)
	r.Use(middlewaremetric.NewHttpMetricCollector(coll).Middleware())
	r.Use(gin.Recovery())
	r.Use(loggerMiddleware())
	r.Use(corsMiddleware())

	// Health-check — используется Docker и оркестраторами.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "ts": time.Now().UTC()})
	})

	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	v1 := r.Group("/api/v1")
	{
		v1.POST("/check", h.Check)
		v1.GET("/history", h.GetHistory)
		v1.GET("/history/:id", h.GetRecord)
	}

	return r
}

// loggerMiddleware — минимальный структурированный лог каждого запроса.
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		gin.DefaultWriter.Write([]byte(
			time.Now().Format("2006-01-02 15:04:05") + " | " +
				statusColor(c.Writer.Status()) +
				" | " + latency.String() +
				" | " + c.Request.Method +
				" " + c.Request.URL.Path + "\n",
		))
	}
}

func statusColor(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	default:
		return "2xx"
	}
}

// corsMiddleware — разрешаем запросы от любого origin (для хакатона достаточно).
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
