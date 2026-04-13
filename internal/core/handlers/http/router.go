package httphandler

import (
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	httpmetric "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/http"
	"github.com/Revachol/SpamBreaker_VK_back/internal/middleware"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter собирает gin.Engine с маршрутами и middleware.
func NewRouter(h *Handler, reg *prometheus.Registry, cfg config.Config, l logger.Log) *gin.Engine {
	r := gin.New()

	coll := httpmetric.NewPrometheusHttpCollector(cfg.Name, reg)
	r.Use(httpmetric.Middleware(coll))
	r.Use(gin.Recovery())
	r.Use(middleware.LogMiddleware(l))
	r.Use(middleware.CORSMiddleware(&cfg.Cors))

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
