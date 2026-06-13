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
	bh *BotHandler,
	ah *AuthHandler,
	mh *ModeratorHandler,
	bmh *BotManageHandler,
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
	bot := r.Group("/api/bot/v1/:service")
	{
		bot.POST("/check", bh.Check)                 // check message
		bot.POST("/chat/active", bh.ActivateAddChat) // Add bot to chat
		bot.PATCH("/chat/active", bh.ActivateChat)   // Activate bot in chat
		bot.PATCH("/chat/name", bh.UpdateChatName)   // Update bot chat name
		bot.DELETE("/chat/active", bh.DeactivateBot) // Del bot to chat
		bot.POST("/active", bh.VerifyUserToken)      // Verify user
	}

	// Защищённые маршруты — требуют Bearer-токен.
	v1 := r.Group("/api/v1")
	v1.Use(JWTMiddleware(jwtManager))
	{
		v1.GET("/history", bh.GetHistory)
		v1.GET("/history/:id", bh.GetHistoryRecord)

		user := v1.Group("/user")
		{
			user.POST("/:service/verify", mh.InitiateVerification)
			user.GET("/account", mh.GetModeratorAccounts)
			user.GET("/account/:accID", mh.GetModeratorAccountInfo)
			user.GET("/bot", mh.GetUserBots)
		}

		// Telegram bot routes
		bots := v1.Group("/bot/:app_id")
		{
			bots.GET("/", bmh.GetInfo)
			bots.GET("/admin", mh.GetAdmins)
			bots.POST("/admin", mh.AddAdmin)
			bots.DELETE("/admin/:mod_id", mh.RemoveAdmin)
			bots.GET("/history", bh.GetBotHistory)
			bots.GET("/settings", bmh.GetSettings)
			bots.POST("/settings", bmh.UpdateSettings)
			bots.POST("/disable", bmh.DisableBot)
		}
	}

	return r
}
