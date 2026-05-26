package httphandler

import (
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	httpmetric "github.com/Revachol/SpamBreaker_VK_back/internal/metrics/http"
	"github.com/Revachol/SpamBreaker_VK_back/internal/middleware"
	jwtpkg "github.com/Revachol/SpamBreaker_VK_back/pkg/jwt"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter собирает gin.Engine с маршрутами и middleware.
func NewRouter(
	h *Handler,
	ah *AuthHandler,
	tbh *TelegramBotHandler,
	vbh *VkBotHandler,
	jwtManager *jwtpkg.Manager,
	reg *prometheus.Registry,
	cfg *config.Config,
	l logger.Log,
) *gin.Engine {
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

	// Публичные маршруты — регистрация и вход.
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", ah.Register)
		authGroup.POST("/login", ah.Login)
	}

	// Публичные маршруты бота — вызываются Telegram-ботом без JWT.
	bot := r.Group("/api/v1")
	{
		bot.POST("/check", h.Check)
		bot.POST("/bots/telegram/activate", tbh.ActivateBot)
		bot.GET("/bots/telegram/internal/chat-active", tbh.IsChatActive)
		bot.POST("/bots/vk/activate", vbh.ActivateBot)
		bot.GET("/bots/vk/internal/chat-active", vbh.IsChatActive)
	}

	// Защищённые маршруты — требуют Bearer-токен.
	v1 := r.Group("/api/v1")
	v1.Use(JWTMiddleware(jwtManager))
	{
		v1.GET("/history", h.GetHistory)
		v1.GET("/history/:id", h.GetRecord)
		v1.GET("/bots/telegram/history", h.GetBotHistory)
		v1.GET("/bots/vk/history", h.GetBotHistory)

		// Telegram bot routes
		telegram := v1.Group("/bots/telegram")
		{
			telegram.GET("", tbh.ListBots)
			telegram.POST("", tbh.CreateBot)

			bot := telegram.Group("/:botId")
			{
				bot.GET("", tbh.GetBot)
				bot.PUT("", tbh.RenameBot)
				bot.DELETE("", tbh.DeleteBot)
				bot.GET("/token", tbh.GetBotToken)
				bot.GET("/status", tbh.GetBotStatus)
				bot.GET("/settings", tbh.GetSettings)
				bot.POST("/settings", tbh.UpdateSettings)
				bot.POST("/disable", tbh.DisableBot)
				bot.POST("/verify-chat", tbh.VerifyChat)
				bot.GET("/history", tbh.GetHistory)
				bot.GET("/admins", tbh.GetAdmins)
				bot.POST("/admins", tbh.AddAdmin)
				bot.DELETE("/admins/:username", tbh.RemoveAdmin)
			}
		}

		// Telegram bot routes
		vk := v1.Group("/bots/vk")
		{
			vk.GET("/token", vbh.GetToken)
			vk.GET("/status", vbh.GetStatus)
			vk.GET("/settings", vbh.GetSettings)
			vk.POST("/settings", vbh.UpdateSettings)
			vk.POST("/verify-chat", vbh.VerifyChat)
			vk.POST("/disable", vbh.DisableBot)
			vk.GET("/admins", vbh.GetAdmins)
			vk.POST("/admins", vbh.AddAdmin)
			vk.DELETE("/admins/:username", vbh.RemoveAdmin)
		}
	}

	return r
}
