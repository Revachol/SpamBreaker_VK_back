// === internal/transport/httphandler/telegram_bot_handler.go ===
package httphandler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TelegramBotHandler отвечает за управление ботами и администраторами.
type TelegramBotHandler struct {
	telegramBot *service.TelegramBotUseCase
	logger      logger.Log
}

// NewTelegramBotHandler создаёт новый TelegramBotHandler.
func NewTelegramBotHandler(
	telegramBot *service.TelegramBotUseCase,
	l logger.Log,
) *TelegramBotHandler {
	return &TelegramBotHandler{
		telegramBot: telegramBot,
		logger:      l,
	}
}

// ---------- DTOs ----------

type telegramBotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

type telegramBotStatusResponse struct {
	Connected   bool   `json:"connected"`
	ChatID      string `json:"chat_id,omitempty"`
	ActivatedAt string `json:"activated_at,omitempty"`
}

type telegramBotSettingsResponse struct {
	Sensitivity int      `json:"sensitivity"`
	BannedWords []string `json:"banned_words"`
	Enabled     bool     `json:"enabled"`
}

type telegramBotSettingsRequest struct {
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
//	@Produce      json
//	@Success      200 {object} telegramBotTokenResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/token [get]
//	@Security     Bearer
func (h *TelegramBotHandler) GetToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	apps, err := h.telegramBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	for _, a := range apps {
		if a.Platform == "telegram" {
			expiresAt := a.CreatedAt.Add(7 * 24 * time.Hour)
			c.JSON(http.StatusOK, telegramBotTokenResponse{
				Token:     a.Token,
				ExpiresAt: expiresAt.Format(time.RFC3339),
				CreatedAt: a.CreatedAt.Format(time.RFC3339),
			})
			return
		}
	}

	newApp, err := h.telegramBot.GenerateToken(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error generating token: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to generate token"})
		return
	}

	expiresAt := newApp.CreatedAt.Add(7 * 24 * time.Hour)
	c.JSON(http.StatusOK, telegramBotTokenResponse{
		Token:     newApp.Token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		CreatedAt: newApp.CreatedAt.Format(time.RFC3339),
	})
}

// GetStatus godoc
//
//	@Summary      Получить статус Telegram бота
//	@Description  Возвращает статус подключения бота по токену
//	@Tags         telegram-bot
//	@Produce      json
//	@Param        token query string true "Токен активации"
//	@Success      200 {object} telegramBotStatusResponse
//	@Failure      400 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/status [get]
//	@Security     Bearer
func (h *TelegramBotHandler) GetStatus(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	app, err := h.telegramBot.GetByToken(c.Request.Context(), token)
	if err != nil {
		h.logger.Errorf("Error getting application: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get application"})
		return
	}

	if app == nil {
		c.JSON(http.StatusOK, telegramBotStatusResponse{Connected: false})
		return
	}

	resp := telegramBotStatusResponse{Connected: app.Status == "active"}
	if app.ExternalID != "" {
		resp.ChatID = app.ExternalID
	}
	if !app.UpdatedAt.IsZero() && app.Status == "active" {
		resp.ActivatedAt = app.UpdatedAt.Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, resp)
}

// GetSettings godoc
//
//	@Summary      Получить настройки Telegram бота
//	@Description  Возвращает настройки активного Telegram бота текущего пользователя
//	@Tags         telegram-bot
//	@Produce      json
//	@Success      200 {object} telegramBotSettingsResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/settings [get]
//	@Security     Bearer
func (h *TelegramBotHandler) GetSettings(c *gin.Context) {
	userID, _ := c.Get("user_id")

	apps, err := h.telegramBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "telegram" && app.Status == "active" {
			activeApp = app
			break
		}
	}
	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active telegram bot found"})
		return
	}

	settings, err := h.telegramBot.GetSettings(c.Request.Context(), activeApp.ID)
	if err != nil {
		h.logger.Errorf("Error getting settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get settings"})
		return
	}

	c.JSON(http.StatusOK, telegramBotSettingsResponse{
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
//	@Accept       json
//	@Produce      json
//	@Param        body body telegramBotSettingsRequest true "Новые настройки"
//	@Success      200 {object} telegramBotSettingsResponse
//	@Failure      400 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/settings [post]
//	@Security     Bearer
func (h *TelegramBotHandler) UpdateSettings(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req telegramBotSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	apps, err := h.telegramBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "telegram" && app.Status == "active" {
			activeApp = app
			break
		}
	}
	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active telegram bot found"})
		return
	}

	settings, err := h.telegramBot.GetSettings(c.Request.Context(), activeApp.ID)
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

	if err := h.telegramBot.UpdateSettings(c.Request.Context(), settings); err != nil {
		h.logger.Errorf("Error updating settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, telegramBotSettingsResponse{
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
//	@Produce      json
//	@Success      200 {object} map[string]bool
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/disable [post]
//	@Security     Bearer
func (h *TelegramBotHandler) DisableBot(c *gin.Context) {
	userID, _ := c.Get("user_id")

	apps, err := h.telegramBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "telegram" && app.Status == "active" {
			activeApp = app
			break
		}
	}
	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active telegram bot found"})
		return
	}

	if err := h.telegramBot.DisableBot(c.Request.Context(), activeApp.ID); err != nil {
		h.logger.Errorf("Error disabling bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to disable bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// IsChatActive godoc
//
//	@Summary      Проверить, активен ли чат
//	@Description  Внутренний эндпоинт для бота — проверяет, зарегистрирован ли чат в системе
//	@Tags         telegram-internal
//	@Produce      json
//	@Param        chat_id query string true "Числовой ID чата Telegram"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Router       /api/v1/bots/telegram/internal/chat-active [get]
func (h *TelegramBotHandler) IsChatActive(c *gin.Context) {
	chatID := c.Query("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	active, err := h.telegramBot.IsChatActive(c.Request.Context(), chatID)
	if err != nil {
		h.logger.Errorf("Error checking chat active: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"active": active})
}

// VerifyAddChat godoc
//
//	@Summary      Проверить, что бот добавлен администратором с верификацией
//	@Description  Вызывается при событии my_chat_member: проверяет, что добавивший — верифицированный администратор чата
//	@Tags         telegram-internal
//	@Accept       json
//	@Produce      json
//	@Param        body body object{chat_id=int,user_id=int} true "chat_id и user_id добавившего"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/verify-add-chat [post]
func (h *BotHandler) VerifyAddChat(c *gin.Context) {
	var req AddChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// 1. Проверяем, верифицирован ли пользователь
	uid, err := strconv.ParseInt(req.UserID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid user id"})
	}
	_, err = h.moderatorService.GetModeratorIDByVerifiedTelegramID(c.Request.Context(), uid)
	if err != nil {
		h.logger.Warnf("VerifyAddChat: unverified user %d: %v", req.UserID, err)
		c.JSON(http.StatusForbidden, errorResponse{Error: "пользователь не верифицирован"})
		return
	}
	if !req.IsAdmin {
		c.JSON(http.StatusForbidden, AddChatResponse{Verified: false, Message: "пользователь не является администратором чата"})
		return
	}
	h.telegramBot.GenerateToken(c, req.UserID)

	c.JSON(http.StatusOK, AddChatResponse{
		Verified: true,
		Message:  "",
	})
}
