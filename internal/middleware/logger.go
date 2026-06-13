package middleware

import (
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

func LogMiddleware(log logger.Log) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/health" || path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		rawQuery := c.Request.URL.RawQuery

		// Обработка запроса
		c.Next()

		// Сбор данных после выполнения
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()
		normalizedPath := NormalizePath(path)

		// Базовое сообщение лога
		msg := "[GIN] %s | %3d | %13v | %15s | %-7s %s | %s"
		args := []interface{}{
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			normalizedPath,
			userAgent,
		}

		// Выбор уровня логирования в зависимости от статус-кода
		switch {
		case statusCode >= 500:
			log.Errorf(msg, args...)
		case statusCode >= 400:
			log.Warnf(msg, args...)
		default:
			log.Infof(msg, args...)
		}

		// Дополнительно логируем query-параметры, если они есть (на уровне Info)
		if rawQuery != "" {
			log.Debugf("Query: %s", rawQuery)
		}
	}
}
