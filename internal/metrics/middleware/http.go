package middlewaremetric

import (
	"regexp"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/metrics/interfaces"
	"github.com/gin-gonic/gin"
)

type HttpMetricCollector struct {
	metrics interfaces.HttpMetrics
}

func NewHttpMetricCollector(metric interfaces.HttpMetrics) *HttpMetricCollector {
	return &HttpMetricCollector{
		metrics: metric,
	}
}

func (h *HttpMetricCollector) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		path := h.normalizePath(c.Request.URL.Path)
		status := c.Writer.Status() // прямое получение статуса

		h.metrics.IncHTTPRequest(c.Request.Method, path, status)
		h.metrics.ObserveHTTPDuration(c.Request.Method, path, status, duration)
	}
}

// normalizePath нормализует путь URL, заменяя числовые ID и UUID на плейсхолдеры
func (c *HttpMetricCollector) normalizePath(path string) string {
	path, _ = strings.CutSuffix(path, "?")

	// Заменяем UUID на :id
	uuidRegex := regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	path = uuidRegex.ReplaceAllString(path, ":id")

	// Заменяем числовые ID на :id
	numericRegex := regexp.MustCompile(`/\d+`)
	path = numericRegex.ReplaceAllString(path, "/:id")

	// Заменяем email-like строки
	emailRegex := regexp.MustCompile(`/[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	path = emailRegex.ReplaceAllString(path, "/:email")

	// Заменяем хеши и токены
	hashRegex := regexp.MustCompile(`/[a-fA-F0-9]{32,}`)
	path = hashRegex.ReplaceAllString(path, "/:hash")

	path, _ = strings.CutSuffix(path, "/")

	return path
}
