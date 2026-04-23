package httphandler

import (
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TelegramBotHandler handles Telegram bot related HTTP requests.
type TelegramBotHandler struct {
	telegramBot *service.TelegramBotUseCase
	logger      logger.Log
}

// NewTelegramBotHandler creates a new TelegramBotHandler.
func NewTelegramBotHandler(telegramBot *service.TelegramBotUseCase, l logger.Log) *TelegramBotHandler {
	return &TelegramBotHandler{telegramBot: telegramBot, logger: l}
}

// ---------- Request / Response DTOs ----------

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

type verifyChatRequest struct {
	ChatID string `json:"chat_id" binding:"required"`
}

type verifyChatResponse struct {
	Success   bool   `json:"success"`
	Verified  bool   `json:"verified"`
	Message   string `json:"message"`
	Activated bool   `json:"activated"`
	Token     string `json:"token"`
}

// ---------- Handlers ----------

// GetToken godoc
//
//	@Summary     Получить токен для активации Telegram бота
//	@Description Генерирует новый токен для активации Telegram бота
//	@Tags        telegram
//	@Produce     json
//	@Success     200 {object} telegramBotTokenResponse
//	@Failure     500 {object} errorResponse
//	@Router      /api/v1/bots/telegram/token [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Возвращаем токен существующего приложения если есть, иначе создаём новое.
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

	// Приложения нет — создаём
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
//	@Summary     Получить статус Telegram бота
//	@Description Проверяет статус подключения Telegram бота по токену
//	@Tags        telegram
//	@Produce     json
//	@Param       token query    string true "Токен активации"
//	@Success     200   {object} telegramBotStatusResponse
//	@Failure     400   {object} errorResponse
//	@Failure     500   {object} errorResponse
//	@Router      /api/v1/bots/telegram/status [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetStatus(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	// Получаем приложение по токену
	app, err := h.telegramBot.GetByToken(c.Request.Context(), token)
	if err != nil {
		h.logger.Errorf("Error getting application by token: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get application"})
		return
	}

	if app == nil {
		c.JSON(http.StatusOK, telegramBotStatusResponse{Connected: false})
		return
	}

	// Формируем ответ
	response := telegramBotStatusResponse{
		Connected: app.Status == "active",
	}

	if app.ExternalID != "" {
		response.ChatID = app.ExternalID
	}

	if !app.UpdatedAt.IsZero() && app.Status == "active" {
		response.ActivatedAt = app.UpdatedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, response)
}

// GetSettings godoc
//
//	@Summary     Получить настройки Telegram бота
//	@Description Возвращает текущие настройки Telegram бота
//	@Tags        telegram
//	@Produce     json
//	@Success     200 {object} telegramBotSettingsResponse
//	@Failure     500 {object} errorResponse
//	@Router      /api/v1/bots/telegram/settings [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetSettings(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Получаем список ботов пользователя (берем первый активный)
	apps, err := h.telegramBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем активный Telegram бот
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

	// Получаем настройки
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
//	@Summary     Обновить настройки Telegram бота
//	@Description Обновляет настройки Telegram бота
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       body body     telegramBotSettingsRequest true "Новые настройки"
//	@Success     200  {object} telegramBotSettingsResponse
//	@Failure     400  {object} errorResponse
//	@Failure     500  {object} errorResponse
//	@Router      /api/v1/bots/telegram/settings [post]
//	@Security    Bearer
func (h *TelegramBotHandler) UpdateSettings(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	var req telegramBotSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("Bind error %s", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Получаем список ботов пользователя (берем первый активный)
	apps, err := h.telegramBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем активный Telegram бот
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

	// Получаем текущие настройки
	settings, err := h.telegramBot.GetSettings(c.Request.Context(), activeApp.ID)
	if err != nil {
		h.logger.Errorf("Error getting settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get settings"})
		return
	}

	// Обновляем настройки
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
//	@Summary     Отключить Telegram бота
//	@Description Отключает Telegram бота
//	@Tags        telegram
//	@Produce     json
//	@Success     200 {object} map[string]bool
//	@Failure     500 {object} errorResponse
//	@Router      /api/v1/bots/telegram/disable [post]
//	@Security    Bearer
func (h *TelegramBotHandler) DisableBot(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Получаем список ботов пользователя (берем первый активный)
	apps, err := h.telegramBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем активный Telegram бот
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

	// Отключаем бота
	if err := h.telegramBot.DisableBot(c.Request.Context(), activeApp.ID); err != nil {
		h.logger.Errorf("Error disabling bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to disable bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// VerifyChat godoc
//
//	@Summary     Проверить и активировать Telegram бота
//	@Description Проверяет, что бот находится в указанном чате, и активирует его
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       body body     verifyChatRequest true "ID или username чата Telegram"
//	@Success     200  {object} verifyChatResponse
//	@Failure     400  {object} errorResponse
//	@Failure     500  {object} errorResponse
//	@Router      /api/v1/bots/telegram/verify-chat [post]
//	@Security    Bearer
func (h *TelegramBotHandler) VerifyChat(c *gin.Context) {
	var req verifyChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Получаем приложение пользователя
	apps, err := h.telegramBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем Telegram бот пользователя
	var userApp *domain.Application
	for _, app := range apps {
		if app.Platform == "telegram" {
			userApp = app
			break
		}
	}

	// Если приложения нет — создаём автоматически
	if userApp == nil {
		newApp, err := h.telegramBot.GenerateToken(c.Request.Context(), userID.(string))
		if err != nil {
			h.logger.Errorf("Error creating application: %s", err)
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to create application"})
			return
		}
		userApp = newApp
	}

	// Проверяем чат
	if err := h.telegramBot.VerifyChat(c.Request.Context(), userApp.ID, req.ChatID); err != nil {
		h.logger.Errorf("Error verifying chat: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to verify chat: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, verifyChatResponse{
		Success:   true,
		Verified:  true,
		Message:   "Bot successfully verified and activated in chat",
		Activated: true,
		Token:     userApp.Token,
	})
}

// IsChatActive godoc
//
//	@Summary     Проверить, зарегистрирован ли чат
//	@Description Внутренний эндпоинт для бота — проверяет, активен ли чат в системе
//	@Tags        telegram-internal
//	@Produce     json
//	@Param       chat_id query  string true "Числовой ID чата Telegram"
//	@Success     200     {object} map[string]bool
//	@Failure     400     {object} errorResponse
//	@Router      /api/v1/bots/telegram/internal/chat-active [get]
func (h *TelegramBotHandler) IsChatActive(c *gin.Context) {
	chatID := c.Query("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	active, err := h.telegramBot.IsChatActive(c.Request.Context(), chatID)
	if err != nil {
		h.logger.Errorf("Error checking chat active status: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"active": active})
}

// ActivateBot godoc
//
//	@Summary     Активировать Telegram бота
//	@Description Активирует Telegram бота по токену и ID чата (вызывается Telegram ботом)
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       token query    string true "Токен активации"
//	@Param       chat_id query  string true "ID чата Telegram"
//	@Success     200   {object} map[string]bool
//	@Failure     400   {object} errorResponse
//	@Failure     500   {object} errorResponse
//	@Router      /api/v1/bots/telegram/activate [post]
func (h *TelegramBotHandler) ActivateBot(c *gin.Context) {
	token := c.Query("token")
	chatID := c.Query("chat_id")

	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	if chatID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	// Активируем бота
	if err := h.telegramBot.ActivateBotByChatID(c.Request.Context(), token, chatID); err != nil {
		h.logger.Errorf("Error activating bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to activate bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
