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
	moderation  *service.ModerationUseCase
	logger      logger.Log
}

// NewTelegramBotHandler creates a new TelegramBotHandler.
func NewTelegramBotHandler(telegramBot *service.TelegramBotUseCase, moderation *service.ModerationUseCase, l logger.Log) *TelegramBotHandler {
	return &TelegramBotHandler{telegramBot: telegramBot, moderation: moderation, logger: l}
}

// ---------- DTOs ----------

type createBotRequest struct {
	Name string `json:"name"`
}

type botResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	ChatID     string `json:"chat_id,omitempty"`
	Token      string `json:"token,omitempty"`
	CreatedAt  string `json:"created_at"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

type renameBotRequest struct {
	Name string `json:"name" binding:"required"`
}

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

type adminInfoResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type addAdminRequest struct {
	Username string `json:"username" binding:"required"`
}

// ---------- helpers ----------

func appToBot(a *domain.Application) botResponse {
	r := botResponse{
		ID:        a.ID,
		Name:      a.Name,
		Status:    a.Status,
		ChatID:    a.ExternalID,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
	}
	if !a.VerifiedAt.IsZero() {
		r.VerifiedAt = a.VerifiedAt.Format(time.RFC3339)
	}
	return r
}

func (h *TelegramBotHandler) checkErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "bot not found":
		c.JSON(http.StatusNotFound, errorResponse{Error: "bot not found"})
	case "forbidden":
		c.JSON(http.StatusForbidden, errorResponse{Error: "access denied"})
	default:
		h.logger.Errorf("telegram handler error: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
	return true
}

func userID(c *gin.Context) (string, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return "", false
	}
	return v.(string), true
}

// ---------- Bot CRUD ----------

// ListBots godoc
//
//	@Summary     Список Telegram-ботов
//	@Tags        telegram
//	@Produce     json
//	@Success     200 {array}  botResponse
//	@Router      /api/v1/bots/telegram [get]
//	@Security    Bearer
func (h *TelegramBotHandler) ListBots(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}

	bots, err := h.telegramBot.ListTelegramBots(c.Request.Context(), uid)
	if h.checkErr(c, err) {
		return
	}

	resp := make([]botResponse, 0, len(bots))
	for _, a := range bots {
		resp = append(resp, appToBot(a))
	}
	c.JSON(http.StatusOK, resp)
}

// CreateBot godoc
//
//	@Summary     Создать Telegram-бота
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       body body     createBotRequest false "Имя бота"
//	@Success     201  {object} botResponse
//	@Router      /api/v1/bots/telegram [post]
//	@Security    Bearer
func (h *TelegramBotHandler) CreateBot(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}

	var req createBotRequest
	_ = c.ShouldBindJSON(&req)

	app, err := h.telegramBot.CreateBot(c.Request.Context(), uid, req.Name)
	if h.checkErr(c, err) {
		return
	}

	r := appToBot(app)
	r.Token = app.Token
	c.JSON(http.StatusCreated, r)
}

// GetBot godoc
//
//	@Summary     Получить Telegram-бота
//	@Tags        telegram
//	@Produce     json
//	@Param       botId path     string true "ID бота"
//	@Success     200   {object} botResponse
//	@Router      /api/v1/bots/telegram/{botId} [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetBot(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	app, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID)
	if h.checkErr(c, err) {
		return
	}

	c.JSON(http.StatusOK, appToBot(app))
}

// DeleteBot godoc
//
//	@Summary     Удалить Telegram-бота
//	@Tags        telegram
//	@Param       botId path string true "ID бота"
//	@Success     200
//	@Router      /api/v1/bots/telegram/{botId} [delete]
//	@Security    Bearer
func (h *TelegramBotHandler) DeleteBot(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	if err := h.telegramBot.DeleteBot(c.Request.Context(), uid, botID); h.checkErr(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RenameBot godoc
//
//	@Summary     Переименовать Telegram-бота
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       botId path     string          true "ID бота"
//	@Param       body  body     renameBotRequest true "Новое имя"
//	@Success     200   {object} botResponse
//	@Router      /api/v1/bots/telegram/{botId} [put]
//	@Security    Bearer
func (h *TelegramBotHandler) RenameBot(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	var req renameBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := h.telegramBot.RenameBot(c.Request.Context(), uid, botID, req.Name); h.checkErr(c, err) {
		return
	}

	app, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID)
	if h.checkErr(c, err) {
		return
	}

	c.JSON(http.StatusOK, appToBot(app))
}

// GetBotToken godoc
//
//	@Summary     Получить токен бота
//	@Tags        telegram
//	@Produce     json
//	@Param       botId path     string true "ID бота"
//	@Success     200   {object} telegramBotTokenResponse
//	@Router      /api/v1/bots/telegram/{botId}/token [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetBotToken(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	app, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID)
	if h.checkErr(c, err) {
		return
	}

	expiresAt := app.CreatedAt.Add(7 * 24 * time.Hour)
	c.JSON(http.StatusOK, telegramBotTokenResponse{
		Token:     app.Token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		CreatedAt: app.CreatedAt.Format(time.RFC3339),
	})
}

// GetBotStatus godoc
//
//	@Summary     Статус конкретного бота
//	@Tags        telegram
//	@Produce     json
//	@Param       botId path     string true "ID бота"
//	@Success     200   {object} telegramBotStatusResponse
//	@Router      /api/v1/bots/telegram/{botId}/status [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetBotStatus(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	app, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID)
	if h.checkErr(c, err) {
		return
	}

	resp := telegramBotStatusResponse{
		Connected: app.Status == "active",
	}
	if app.ExternalID != "" {
		resp.ChatID = app.ExternalID
	}
	if !app.VerifiedAt.IsZero() && app.Status == "active" {
		resp.ActivatedAt = app.VerifiedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, resp)
}

// GetSettings godoc
//
//	@Summary     Настройки бота
//	@Tags        telegram
//	@Produce     json
//	@Param       botId path     string true "ID бота"
//	@Success     200   {object} telegramBotSettingsResponse
//	@Router      /api/v1/bots/telegram/{botId}/settings [get]
//	@Security    Bearer
func (h *TelegramBotHandler) GetSettings(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	if _, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID); h.checkErr(c, err) {
		return
	}

	settings, err := h.telegramBot.GetSettings(c.Request.Context(), botID)
	if h.checkErr(c, err) {
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
//	@Summary     Обновить настройки бота
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       botId path     string                      true "ID бота"
//	@Param       body  body     telegramBotSettingsRequest  true "Настройки"
//	@Success     200   {object} telegramBotSettingsResponse
//	@Router      /api/v1/bots/telegram/{botId}/settings [post]
//	@Security    Bearer
func (h *TelegramBotHandler) UpdateSettings(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	var req telegramBotSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if _, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID); h.checkErr(c, err) {
		return
	}

	settings, err := h.telegramBot.GetSettings(c.Request.Context(), botID)
	if h.checkErr(c, err) {
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

	if err := h.telegramBot.UpdateSettings(c.Request.Context(), settings); h.checkErr(c, err) {
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
//	@Summary     Отключить бота
//	@Tags        telegram
//	@Param       botId path string true "ID бота"
//	@Success     200   {object} map[string]bool
//	@Router      /api/v1/bots/telegram/{botId}/disable [post]
//	@Security    Bearer
func (h *TelegramBotHandler) DisableBot(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	app, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID)
	if h.checkErr(c, err) {
		return
	}
	if app.OwnerID != uid {
		c.JSON(http.StatusForbidden, errorResponse{Error: "only the bot owner can disable it"})
		return
	}

	if err := h.telegramBot.DisableBot(c.Request.Context(), botID); h.checkErr(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// VerifyChat godoc
//
//	@Summary     Подключить бота к чату
//	@Tags        telegram
//	@Accept      json
//	@Produce     json
//	@Param       botId path     string           true "ID бота"
//	@Param       body  body     verifyChatRequest true "ID чата"
//	@Success     200   {object} verifyChatResponse
//	@Router      /api/v1/bots/telegram/{botId}/verify-chat [post]
//	@Security    Bearer
func (h *TelegramBotHandler) VerifyChat(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	var req verifyChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	app, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID)
	if h.checkErr(c, err) {
		return
	}

	if err := h.telegramBot.VerifyChat(c.Request.Context(), app.ID, req.ChatID); err != nil {
		h.logger.Errorf("Error verifying chat: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to verify chat: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, verifyChatResponse{
		Success:   true,
		Verified:  true,
		Message:   "Bot successfully verified and activated in chat",
		Activated: true,
		Token:     app.Token,
	})
}

// GetAdmins возвращает список соадминов бота.
func (h *TelegramBotHandler) GetAdmins(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	if _, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID); h.checkErr(c, err) {
		return
	}

	admins, err := h.telegramBot.GetAdmins(c.Request.Context(), botID)
	if h.checkErr(c, err) {
		return
	}

	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}

// AddAdmin добавляет соадмина. Только владелец.
func (h *TelegramBotHandler) AddAdmin(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	var req addAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if _, err := h.telegramBot.AddAdmin(c.Request.Context(), uid, botID, req.Username); err != nil {
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

	admins, err := h.telegramBot.GetAdmins(c.Request.Context(), botID)
	if h.checkErr(c, err) {
		return
	}

	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}

// RemoveAdmin удаляет соадмина. Только владелец.
func (h *TelegramBotHandler) RemoveAdmin(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")
	targetUsername := c.Param("username")
	if targetUsername == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "username is required"})
		return
	}

	if err := h.telegramBot.RemoveAdmin(c.Request.Context(), uid, botID, targetUsername); err != nil {
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

	admins, err := h.telegramBot.GetAdmins(c.Request.Context(), botID)
	if h.checkErr(c, err) {
		return
	}

	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}

// GetHistory возвращает историю сообщений конкретного бота.
// Доступно только владельцу или соадмину.
func (h *TelegramBotHandler) GetHistory(c *gin.Context) {
	uid, ok := userID(c)
	if !ok {
		return
	}
	botID := c.Param("botId")

	if _, err := h.telegramBot.GetBot(c.Request.Context(), uid, botID); h.checkErr(c, err) {
		return
	}

	limit := queryIntParam(c, "limit", 100)
	offset := queryIntParam(c, "offset", 0)

	records, err := h.moderation.GetHistoryByApp(c.Request.Context(), botID, limit, offset)
	if err != nil {
		h.logger.Errorf("GetHistory: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get history"})
		return
	}

	type record struct {
		ID         string             `json:"id"`
		Label      string             `json:"label"`
		Confidence float64            `json:"confidence"`
		AllScores  map[string]float64 `json:"all_scores"`
		CreatedAt  string             `json:"created_at"`
	}

	resp := make([]record, 0, len(records))
	for _, r := range records {
		resp = append(resp, record{
			ID:         r.ID,
			Label:      r.Verdict.Label,
			Confidence: r.Verdict.Confidence,
			AllScores:  r.Verdict.AllScores,
			CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, resp)
}

func queryIntParam(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// ---------- Public endpoints (called by the Telegram bot process, no JWT) ----------

// IsChatActive godoc
//
//	@Summary     Проверить, зарегистрирован ли чат
//	@Tags        telegram-internal
//	@Produce     json
//	@Param       chat_id query  string true "ID чата Telegram"
//	@Success     200     {object} map[string]bool
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
//	@Summary     Активировать бота через токен (вызывается Telegram-ботом)
//	@Tags        telegram
//	@Param       token   query string true "Токен активации"
//	@Param       chat_id query string true "ID чата"
//	@Success     200     {object} map[string]bool
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

	if err := h.telegramBot.ActivateBotByChatID(c.Request.Context(), token, chatID); err != nil {
		h.logger.Errorf("Error activating bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to activate bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
