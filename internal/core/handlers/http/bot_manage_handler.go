package httphandler

import (
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// BotManageHandler отвечает за управление ботами и администраторами.
type BotManageHandler struct {
	botUC     *service.BotUseCase
	moderator *service.ModeratorService
	logger    logger.Log
}

// NewBotManageHandler создаёт новый BotManageHandler.
func NewBotManageHandler(
	bot *service.BotUseCase,
	moderator *service.ModeratorService,
	l logger.Log,
) *BotManageHandler {
	return &BotManageHandler{
		botUC:     bot,
		moderator: moderator,
		logger:    l,
	}
}

// ---------- DTOs ----------

type botUCSettingsResponse struct {
	Sensitivity int      `json:"sensitivity"`
	BannedWords []string `json:"banned_words"`
	Enabled     bool     `json:"enabled"`
}

type botUCSettingsRequest struct {
	Sensitivity *int      `json:"sensitivity,omitempty"`
	BannedWords *[]string `json:"banned_words,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
}

type AddChatRequest struct {
	ChatID  string `json:"chat_id" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
	IsAdmin bool   `json:"is_admin" binding:"required"`
}

type AddChatResponse struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// ---------- Handlers ----------

// GetToken godoc
//
//	@Summary      Получить токен для активации Telegram бота
//	@Description  Генерирует или возвращает существующий токен для Telegram бота пользователя
//	@Tags         telegram-bot
//	@Produce      JSON
//	@Success      200 {object} botUCTokenResponse
//	@Failure      500 {object} errorResponse
//	@Security     Bearer
//func (h *BotManageHandler) GetToken(c *gin.Context) {
//	userID, exists := c.Get("user_id")
//	if !exists {
//		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
//		return
//	}
//
//	apps, err := h.botUC.ListBots(c.Request.Context(), userID.(string))
//	if err != nil {
//		h.logger.Errorf("Error listing bots: %s", err)
//		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
//		return
//	}
//
//	for _, a := range apps {
//		if a.Platform == "telegram" {
//			expiresAt := a.CreatedAt.Add(7 * 24 * time.Hour)
//			c.JSON(http.StatusOK, botUCTokenResponse{
//				Token:     a.Token,
//				ExpiresAt: expiresAt.Format(time.RFC3339),
//				CreatedAt: a.CreatedAt.Format(time.RFC3339),
//			})
//			return
//		}
//	}
//
//	newApp, err := h.botUC.GenerateToken(c.Request.Context(), userID.(string))
//	if err != nil {
//		h.logger.Errorf("Error generating token: %s", err)
//		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to generate token"})
//		return
//	}
//
//	expiresAt := newApp.CreatedAt.Add(7 * 24 * time.Hour)
//	c.JSON(http.StatusOK, botUCTokenResponse{
//		Token:     newApp.Token,
//		ExpiresAt: expiresAt.Format(time.RFC3339),
//		CreatedAt: newApp.CreatedAt.Format(time.RFC3339),
//	})
//}

// GetStatus godoc
//
//	@Summary      Получить статус Telegram бота
//	@Description  Возвращает статус подключения бота по токену
//	@Tags         telegram-bot
//	@Produce      json
//	@Param        token query string true "Токен активации"
//	@Success      200 {object} botUCStatusResponse
//	@Failure      400 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Security     Bearer
//func (h *BotManageHandler) GetStatus(c *gin.Context) {
//	ap
//	if token == "" {
//		c.JSON(http.StatusBadRequest, errorResponse{Error: "token is required"})
//		return
//	}
//
//	app, err := h.botUC.GetByToken(c.Request.Context(), token)
//	if err != nil {
//		h.logger.Errorf("Error getting application: %s", err)
//		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get application"})
//		return
//	}
//
//	if app == nil {
//		c.JSON(http.StatusOK, botUCStatusResponse{Connected: false})
//		return
//	}
//
//	resp := botUCStatusResponse{Connected: app.Status == "active"}
//	if app.ExternalID != "" {
//		resp.ChatID = app.ExternalID
//	}
//	if !app.UpdatedAt.IsZero() && app.Status == "active" {
//		resp.ActivatedAt = app.UpdatedAt.Format(time.RFC3339)
//	}
//	c.JSON(http.StatusOK, resp)
//}

// GetSettings godoc
//
//	@Summary      Получить настройки Telegram бота
//	@Description  Возвращает настройки активного Telegram бота текущего пользователя
//	@Tags         telegram-bot
//	@Produce      json
//	@Param        appID path string true "ID приложения"
//	@Success      200 {object} botUCSettingsResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/settings [get]
//	@Security     Bearer
func (h *BotManageHandler) GetSettings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("GetSettings: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	appID := c.Param("app_id")
	if appID == "" {
		h.logger.Warnf("GetSettings: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}

	err := h.moderator.CheckUserOwnApp(c.Request.Context(), userID.(string), appID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	settings, err := h.botUC.GetSettings(c.Request.Context(), appID)
	if err != nil {
		h.logger.Errorf("Error getting settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get settings"})
		return
	}

	c.JSON(http.StatusOK, botUCSettingsResponse{
		Sensitivity: settings.ToxicityThreshold,
		BannedWords: settings.BannedWords,
		Enabled:     settings.AutoModerate,
	})
}

// UpdateSettings godoc
//
//	@Summary      Обновить настройки Telegram бота
//	@Description  Обновляет настройки активного Telegram бота
//	@Tags         telegram-bot
//	@Accept       JSON
//	@Produce      JSON
//	@Param        appID path string true "ID приложения"
//	@Param        body body botUCSettingsRequest true "Новые настройки"
//	@Success      200 {object} botUCSettingsResponse
//	@Failure      400 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/settings [post]
//	@Security     Bearer
func (h *BotManageHandler) UpdateSettings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("UpdateSettings: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	appID := c.Param("app_id")
	if appID == "" {
		h.logger.Warnf("UpdateSettings: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}

	var req botUCSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("UpdateSettings: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	err := h.moderator.CheckUserOwnApp(c.Request.Context(), userID.(string), appID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	settings, err := h.botUC.GetSettings(c.Request.Context(), appID)
	if err != nil {
		h.logger.Errorf("Error getting settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get settings"})
		return
	}

	if req.Sensitivity != nil {
		settings.ToxicityThreshold = *req.Sensitivity
	}
	if req.BannedWords != nil {
		settings.BannedWords = *req.BannedWords
	}
	if req.Enabled != nil {
		settings.AutoModerate = *req.Enabled
	}
	settings.UpdatedAt = time.Now().UTC()

	if err := h.botUC.UpdateSettings(c.Request.Context(), settings); err != nil {
		h.logger.Errorf("Error updating settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, botUCSettingsResponse{
		Sensitivity: settings.ToxicityThreshold,
		BannedWords: settings.BannedWords,
		Enabled:     settings.AutoModerate,
	})
}

// DisableBot godoc
//
//	@Summary      Отключить Telegram бота
//	@Description  Деактивирует активного Telegram бота пользователя
//	@Tags         telegram-bot
//	@Produce      JSON
//	@Param        appID path string true "ID приложения"
//	@Success      200 {object} map[string]bool
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/disable [post]
//	@Security     Bearer
func (h *BotManageHandler) DisableBot(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("DisableBot: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	appID := c.Param("app_id")
	if appID == "" {
		h.logger.Warnf("DisableBot: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}

	err := h.moderator.CheckUserOwnApp(c.Request.Context(), userID.(string), appID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	if err := h.botUC.DisableBot(c.Request.Context(), appID); err != nil {
		h.logger.Errorf("Error disabling bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to disable bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
