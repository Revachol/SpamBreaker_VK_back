package httphandler

import (
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// VkBotHandler handles Vk bot related HTTP requests.
type VkBotHandler struct {
	vkBot  *service.VkBotUseCase
	logger logger.Log
}

// NewVkBotHandler creates a new VkBotHandler.
func NewVkBotHandler(vkBot *service.VkBotUseCase, l logger.Log) *VkBotHandler {
	return &VkBotHandler{vkBot: vkBot, logger: l}
}

// ---------- Request / Response DTOs ----------

type vkBotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

type vkBotStatusResponse struct {
	Connected   bool   `json:"connected"`
	ChatID      string `json:"chat_id,omitempty"`
	ActivatedAt string `json:"activated_at,omitempty"`
}

type vkBotSettingsResponse struct {
	Sensitivity int      `json:"sensitivity"`
	BannedWords []string `json:"banned_words"`
	Enabled     bool     `json:"enabled"`
}

type vkBotSettingsRequest struct {
	Sensitivity *int      `json:"sensitivity,omitempty"`
	BannedWords *[]string `json:"banned_words,omitempty"`
	Enabled     *bool     `json:"enabled,omitempty"`
}

// ---------- Handlers ----------

// GetToken godoc
//
//	@Summary     Получить токен для активации Vk бота
//	@Description Генерирует новый токен для активации Vk бота
//	@Tags        vk
//	@Produce     json
//	@Success     200 {object} vkBotTokenResponse
//	@Failure     500 {object} errorResponse
//	@Router      /api/v1/bots/vk/token [get]
//	@Security    Bearer
func (h *VkBotHandler) GetToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Возвращаем токен существующего приложения если есть, иначе создаём новое.
	apps, err := h.vkBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	for _, a := range apps {
		if a.Platform == "vk" {
			expiresAt := a.CreatedAt.Add(7 * 24 * time.Hour)
			c.JSON(http.StatusOK, vkBotTokenResponse{
				Token:     a.Token,
				ExpiresAt: expiresAt.Format(time.RFC3339),
				CreatedAt: a.CreatedAt.Format(time.RFC3339),
			})
			return
		}
	}

	// Приложения нет — создаём
	newApp, err := h.vkBot.GenerateToken(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error generating token: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to generate token"})
		return
	}

	expiresAt := newApp.CreatedAt.Add(7 * 24 * time.Hour)
	c.JSON(http.StatusOK, vkBotTokenResponse{
		Token:     newApp.Token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		CreatedAt: newApp.CreatedAt.Format(time.RFC3339),
	})
}

// GetStatus godoc
//
//	@Summary     Получить статус Vk бота
//	@Description Проверяет статус подключения Vk бота по токену
//	@Tags        vk
//	@Produce     json
//	@Param       token query    string true "Токен активации"
//	@Success     200   {object} vkBotStatusResponse
//	@Failure     400   {object} errorResponse
//	@Failure     500   {object} errorResponse
//	@Router      /api/v1/bots/vk/status [get]
//	@Security    Bearer
func (h *VkBotHandler) GetStatus(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	// Получаем приложение по токену
	app, err := h.vkBot.GetByToken(c.Request.Context(), token)
	if err != nil {
		h.logger.Errorf("Error getting application by token: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get application"})
		return
	}

	if app == nil {
		c.JSON(http.StatusOK, vkBotStatusResponse{Connected: false})
		return
	}

	// Формируем ответ
	response := vkBotStatusResponse{
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
//	@Summary     Получить настройки Vk бота
//	@Description Возвращает текущие настройки Vk бота
//	@Tags        vk
//	@Produce     json
//	@Success     200 {object} vkBotSettingsResponse
//	@Failure     500 {object} errorResponse
//	@Router      /api/v1/bots/vk/settings [get]
//	@Security    Bearer
func (h *VkBotHandler) GetSettings(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Получаем список ботов пользователя (берем первый активный)
	apps, err := h.vkBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем активный Vk бот
	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" && app.Status == "active" {
			activeApp = app
			break
		}
	}

	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active vk bot found"})
		return
	}

	// Получаем настройки
	settings, err := h.vkBot.GetSettings(c.Request.Context(), activeApp.ID)
	if err != nil {
		h.logger.Errorf("Error getting settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get settings"})
		return
	}

	c.JSON(http.StatusOK, vkBotSettingsResponse{
		Sensitivity: settings.ToxicityThreshold,
		BannedWords: settings.BannedWords,
		Enabled:     settings.AutoModerate,
	})
}

// UpdateSettings godoc
//
//	@Summary     Обновить настройки Vk бота
//	@Description Обновляет настройки Vk бота
//	@Tags        vk
//	@Accept      json
//	@Produce     json
//	@Param       body body     vkBotSettingsRequest true "Новые настройки"
//	@Success     200  {object} vkBotSettingsResponse
//	@Failure     400  {object} errorResponse
//	@Failure     500  {object} errorResponse
//	@Router      /api/v1/bots/vk/settings [post]
//	@Security    Bearer
func (h *VkBotHandler) UpdateSettings(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	var req vkBotSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("Bind error %s", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Получаем список ботов пользователя (берем первый активный)
	apps, err := h.vkBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем активный Vk бот
	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" && app.Status == "active" {
			activeApp = app
			break
		}
	}

	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active vk bot found"})
		return
	}

	// Получаем текущие настройки
	settings, err := h.vkBot.GetSettings(c.Request.Context(), activeApp.ID)
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

	if err := h.vkBot.UpdateSettings(c.Request.Context(), settings); err != nil {
		h.logger.Errorf("Error updating settings: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, vkBotSettingsResponse{
		Sensitivity: settings.ToxicityThreshold,
		BannedWords: settings.BannedWords,
		Enabled:     settings.AutoModerate,
	})
}

// DisableBot godoc
//
//	@Summary     Отключить Vk бота
//	@Description Отключает Vk бота
//	@Tags        vk
//	@Produce     json
//	@Success     200 {object} map[string]bool
//	@Failure     500 {object} errorResponse
//	@Router      /api/v1/bots/vk/disable [post]
//	@Security    Bearer
func (h *VkBotHandler) DisableBot(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	// Получаем список ботов пользователя (берем первый активный)
	apps, err := h.vkBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем активный Vk бот
	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" && app.Status == "active" {
			activeApp = app
			break
		}
	}

	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active vk bot found"})
		return
	}

	// Отключаем бота
	if err := h.vkBot.DisableBot(c.Request.Context(), activeApp.ID); err != nil {
		h.logger.Errorf("Error disabling bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to disable bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// VerifyChat godoc
//
//	@Summary     Проверить и активировать Vk бота
//	@Description Проверяет, что бот находится в указанном чате, и активирует его
//	@Tags        vk
//	@Accept      json
//	@Produce     json
//	@Param       body body     verifyChatRequest true "ID или username чата Vk"
//	@Success     200  {object} verifyChatResponse
//	@Failure     400  {object} errorResponse
//	@Failure     500  {object} errorResponse
//	@Router      /api/v1/bots/vk/verify-chat [post]
//	@Security    Bearer
func (h *VkBotHandler) VerifyChat(c *gin.Context) {
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
	apps, err := h.vkBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	// Ищем Vk бот пользователя
	var userApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" {
			userApp = app
			break
		}
	}

	// Если приложения нет — создаём автоматически
	if userApp == nil {
		newApp, err := h.vkBot.GenerateToken(c.Request.Context(), userID.(string))
		if err != nil {
			h.logger.Errorf("Error creating application: %s", err)
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to create application"})
			return
		}
		userApp = newApp
	}

	// Проверяем чат
	if err := h.vkBot.VerifyChat(c.Request.Context(), userApp.ID, req.ChatID); err != nil {
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
//	@Tags        vk-internal
//	@Produce     json
//	@Param       chat_id query  string true "Числовой ID чата Vk"
//	@Success     200     {object} map[string]bool
//	@Failure     400     {object} errorResponse
//	@Router      /api/v1/bots/vk/internal/chat-active [get]
func (h *VkBotHandler) IsChatActive(c *gin.Context) {
	chatID := c.Query("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	active, err := h.vkBot.IsChatActive(c.Request.Context(), chatID)
	if err != nil {
		h.logger.Errorf("Error checking chat active status: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"active": active})
}

// GetAdmins возвращает список соадминов бота текущего пользователя.
func (h *VkBotHandler) GetAdmins(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	apps, err := h.vkBot.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" && app.Status == "active" {
			activeApp = app
			break
		}
	}
	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active vk bot found"})
		return
	}

	admins, err := h.vkBot.GetAdmins(c.Request.Context(), activeApp.ID)
	if err != nil {
		h.logger.Errorf("Error getting admins: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get admins"})
		return
	}

	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}

// AddAdmin добавляет соадмина по username. Только владелец бота.
func (h *VkBotHandler) AddAdmin(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	var req addAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	apps, err := h.vkBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" && app.Status == "active" {
			activeApp = app
			break
		}
	}
	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active vk bot found"})
		return
	}

	added, err := h.vkBot.AddAdmin(c.Request.Context(), userID.(string), activeApp.ID, req.Username)
	if err != nil {
		switch err.Error() {
		case "forbidden":
			c.JSON(http.StatusForbidden, errorResponse{Error: "only the bot owner can manage admins"})
		case "user not found":
			c.JSON(http.StatusNotFound, errorResponse{Error: "user not found"})
		case "cannot add yourself as admin":
			c.JSON(http.StatusBadRequest, errorResponse{Error: "cannot add yourself as admin"})
		default:
			h.logger.Errorf("Error adding admin: %s", err)
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to add admin"})
		}
		return
	}

	admins, err := h.vkBot.GetAdmins(c.Request.Context(), activeApp.ID)
	if err != nil {
		h.logger.Errorf("Error getting admins after add: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get admins"})
		return
	}

	_ = added
	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}

// RemoveAdmin удаляет соадмина по username. Только владелец бота.
func (h *VkBotHandler) RemoveAdmin(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	targetUsername := c.Param("username")
	if targetUsername == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "username is required"})
		return
	}

	apps, err := h.vkBot.ListBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("Error listing bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var activeApp *domain.Application
	for _, app := range apps {
		if app.Platform == "vk" && app.Status == "active" {
			activeApp = app
			break
		}
	}
	if activeApp == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active vk bot found"})
		return
	}

	if err := h.vkBot.RemoveAdmin(c.Request.Context(), userID.(string), activeApp.ID, targetUsername); err != nil {
		switch err.Error() {
		case "forbidden":
			c.JSON(http.StatusForbidden, errorResponse{Error: "only the bot owner can manage admins"})
		case "user not found":
			c.JSON(http.StatusNotFound, errorResponse{Error: "user not found"})
		default:
			h.logger.Errorf("Error removing admin: %s", err)
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to remove admin"})
		}
		return
	}

	admins, err := h.vkBot.GetAdmins(c.Request.Context(), activeApp.ID)
	if err != nil {
		h.logger.Errorf("Error getting admins after remove: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get admins"})
		return
	}

	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}

// ActivateBot godoc
//
//	@Summary     Активировать Vk бота
//	@Description Активирует Vk бота по токену и ID чата (вызывается Vk ботом)
//	@Tags        vk
//	@Accept      json
//	@Produce     json
//	@Param       token query    string true "Токен активации"
//	@Param       chat_id query  string true "ID чата Vk"
//	@Success     200   {object} map[string]bool
//	@Failure     400   {object} errorResponse
//	@Failure     500   {object} errorResponse
//	@Router      /api/v1/bots/vk/activate [post]
func (h *VkBotHandler) ActivateBot(c *gin.Context) {
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
	if err := h.vkBot.ActivateBotByChatID(c.Request.Context(), token, chatID); err != nil {
		h.logger.Errorf("Error activating bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to activate bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
