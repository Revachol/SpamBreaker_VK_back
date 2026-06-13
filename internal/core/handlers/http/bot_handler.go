package httphandler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// BotHandler объединяет обработчики модерации, телеграм-ботов и верификации.
type BotHandler struct {
	logger           logger.Log
	moderation       *service.ModerationUseCase
	botUC            *service.BotUseCase
	moderatorService *service.ModeratorService
}

// NewBotHandler создаёт экземпляр BotHandler со всеми зависимостями.
func NewBotHandler(
	moderation *service.ModerationUseCase,
	botUC *service.BotUseCase,
	moderatorService *service.ModeratorService,
	l logger.Log,
) *BotHandler {
	return &BotHandler{
		moderation:       moderation,
		botUC:            botUC,
		moderatorService: moderatorService,
		logger:           l,
	}
}

// ---------- DTOs ----------

type checkRequest struct {
	Text      string `json:"text" binding:"required"`
	ChatID    string `json:"chat_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

type checkResponse struct {
	ID         string             `json:"id"`
	MessageID  string             `json:"message_id,omitempty"`
	Text       string             `json:"text"`
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	AllScores  map[string]float64 `json:"all_scores"`
	CreatedAt  string             `json:"created_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type deactivateBotRequest struct {
	ChatID string `json:"chat_id"`
}

type updateChatNameRequest struct {
	ChatID string `json:"chat_id" binding:"required"`
	Name   string `json:"name" binding:"required"`
}

type verifyUserTokenRequest struct {
	Token  string `json:"token" binding:"required"`
	UserID string `json:"user_id" binding:"required"`
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

// Check godoc
//
//	@Summary     Проверить текст
//	@Description Отправляет текст в ML-сервис и возвращает метку тональности
//	@Tags        moderation
//	@Accept      json
//	@Produce     json
//	@Param       service path string true "Платформа бота (telegram, vk)"
//	@Param       body body     checkRequest  true "Текст для проверки"
//	@Success     200  {object} checkResponse
//	@Failure     400  {object} errorResponse
//	@Failure     502  {object} errorResponse
//	@Router      /api/bot/v1/{service}/check [post]
func (h *BotHandler) Check(c *gin.Context) {
	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		h.logger.Warnf("Check: unsupported service param %q", c.Param("service"))
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	var req checkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("Bind error %s", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if req.ChatID == "" {
		h.logger.Warnf("Check: missing chat ID from request")
	}
	app, err := h.botUC.GetByChatID(c.Request.Context(), platform, req.ChatID)
	if err != nil {
		h.logger.Errorf("Check: GetByChatID(%q) error: %v", req.ChatID, err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if app == nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		h.logger.Warnf("Check: chat_id=%q -> no matching application found", req.ChatID)
	}
	if app.Status != "active" {
		c.JSON(http.StatusNoContent, gin.H{})
	}

	record, err := h.moderation.CheckText(c.Request.Context(), req.Text, app.ID, req.MessageID)
	if err != nil {
		status := http.StatusBadRequest
		if isUpstreamError(err) {
			status = http.StatusBadGateway
		}
		h.logger.Errorf("Check text error %s with status %d", err, status)
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, checkResponse{
		ID:         record.ID,
		MessageID:  record.MessageID,
		Text:       record.Text,
		Label:      record.Verdict.Label,
		Confidence: record.Verdict.Confidence,
		AllScores:  record.Verdict.AllScores,
		CreatedAt:  record.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// GetBotHistory godoc
//
//	@Summary      История проверок Telegram-бота
//	@Description  Возвращает историю сообщений для приложения Telegram-бота текущего пользователя
//	@Tags         telegram-bot
//	@Produce      json
//	@Param        appID  path  string true  "ID приложения"
//	@Param        limit  query int    false "Количество записей (default 50)"
//	@Param        offset query int    false "Смещение"
//	@Success      200 {array}  checkResponse
//	@Failure      401 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/history [get]
//	@Security     Bearer
func (h *BotHandler) GetBotHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	appID := c.Param("app_id")
	if !exists {
		h.logger.Warnf("GetBotHistory: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "not authenticated"})
		return
	}
	if appID == "" {
		h.logger.Warnf("GetBotHistory: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}
	limit := queryInt(c, "limit", 50, h.logger)
	offset := queryInt(c, "offset", 0, h.logger)

	err := h.moderatorService.CheckUserOwnApp(c.Request.Context(), userID.(string), appID)
	if err != nil {
		h.logger.Errorf("GetBotHistory: check app owner: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid app"})
		return
	}

	records, err := h.moderation.GetHistoryByApp(c.Request.Context(), appID, limit, offset)
	if err != nil {
		h.logger.Errorf("GetBotHistory: get history: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	resp := make([]checkResponse, 0, len(records))
	for _, r := range records {
		resp = append(resp, checkResponse{
			ID:         r.ID,
			MessageID:  r.MessageID,
			Text:       r.Text,
			Label:      r.Verdict.Label,
			Confidence: r.Verdict.Confidence,
			AllScores:  r.Verdict.AllScores,
			CreatedAt:  r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	c.JSON(http.StatusOK, resp)
}

// GetHistory godoc
//
//	@Summary     История проверок
//	@Description Возвращает список последних проверок с пагинацией
//	@Tags        moderation
//	@Produce     json
//	@Param       limit  query int false "Количество записей (default 20, max 100)"
//	@Param       offset query int false "Смещение"
//	@Success     200    {array}  checkResponse
//	@Failure     500    {object} errorResponse
//	@Router      /api/v1/history [get]
func (h *BotHandler) GetHistory(c *gin.Context) {
	limit := queryInt(c, "limit", 20, h.logger)
	offset := queryInt(c, "offset", 0, h.logger)

	records, err := h.moderation.GetHistory(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Errorf("GetHistory: get history: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	resp := make([]checkResponse, 0, len(records))
	for _, r := range records {
		resp = append(resp, checkResponse{
			ID:         r.ID,
			MessageID:  r.MessageID,
			Text:       r.Text,
			Label:      r.Verdict.Label,
			Confidence: r.Verdict.Confidence,
			AllScores:  r.Verdict.AllScores,
			CreatedAt:  r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, resp)
}

// GetHistoryRecord godoc
//
//	@Summary     Получить запись по ID
//	@Tags        moderation
//	@Produce     json
//	@Param       id  path     string true "ID записи"
//	@Success     200 {object} checkResponse
//	@Failure     404 {object} errorResponse
//	@Router      /api/v1/history/{id} [get]
func (h *BotHandler) GetHistoryRecord(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		h.logger.Warnf("GetHistoryRecord: missing id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "id is required"})
		return
	}

	record, err := h.moderation.GetRecord(c.Request.Context(), id)
	if err != nil {
		h.logger.Errorf("GetHistoryRecord: get record %q: %v", id, err)
		c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, checkResponse{
		ID:         record.ID,
		MessageID:  record.MessageID,
		Text:       record.Text,
		Label:      record.Verdict.Label,
		Confidence: record.Verdict.Confidence,
		AllScores:  record.Verdict.AllScores,
		CreatedAt:  record.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// VerifyUserToken godoc
//
//	@Summary      Подтвердить аккаунт пользователя Telegram
//	@Description  Проверяет токен верификации, отправленный боту в личные сообщения, и активирует аккаунт модератора
//	@Tags         telegram-internal
//	@Accept       json
//	@Produce      json
//	@Param        service path string true "Платформа бота (telegram, vk)"
//	@Param        body body verifyUserTokenRequest true "Токен и Telegram ID пользователя"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/bot/v1/{service}/active [post]
func (h *BotHandler) VerifyUserToken(c *gin.Context) {
	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		h.logger.Warnf("VerifyUserToken: unsupported service param %q", c.Param("service"))
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	var req verifyUserTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("VerifyUserToken: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	err := h.moderatorService.VerifyAccount(c.Request.Context(), platform, req.Token, req.UserID)
	if err != nil {
		h.logger.Errorf("VerifyUserToken error: %v", err)
		switch {
		case strings.Contains(err.Error(), "invalid token"):
			c.JSON(http.StatusBadRequest, errorResponse{Error: "неверный код верификации"})
		case strings.Contains(err.Error(), "token expired"):
			c.JSON(http.StatusBadRequest, errorResponse{Error: "срок действия кода истёк"})
		case strings.Contains(err.Error(), "already verified"):
			c.JSON(http.StatusConflict, errorResponse{Error: "аккаунт уже верифицирован"})
		default:
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "ошибка верификации"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ActivateAddChat godoc
//
//	@Summary      Проверить, что бот добавлен администратором с верификацией
//	@Description  Вызывается при событии my_chat_member: проверяет, что добавивший — верифицированный администратор чата
//	@Tags         telegram-internal
//	@Accept       json
//	@Produce      json
//	@Param        service path string true "Платформа бота (telegram, vk)"
//	@Param        body body object{chat_id=int,user_id=int} true "chat_id и user_id добавившего"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/bot/v1/{service}/chat/active [post]
func (h *BotHandler) ActivateAddChat(c *gin.Context) {
	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		h.logger.Warnf("ActivateAddChat: unsupported service param %q", c.Param("service"))
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	var req AddChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("ActivateAddChat: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if _, err := h.botUC.AddChat(c.Request.Context(), platform, req.Name, req.UserID, req.ChatID); err != nil {
		if strings.Contains(err.Error(), "chat already connected") {
			c.JSON(http.StatusConflict, errorResponse{Error: "чат уже подключён другим модератором"})
			return
		}
		h.logger.Errorf("ActivateAddChat: add chat: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "не удалось добавить чат"})
		return
	}

	c.JSON(http.StatusOK, AddChatResponse{
		Verified: true,
		Message:  "",
	})
}

// ActivateChat godoc
//
//	@Summary      Активировать чат бота
//	@Description  Активирует бота для указанного чата на платформе
//	@Tags         bot-internal
//	@Accept       json
//	@Produce      json
//	@Param        service path string true "Платформа бота (telegram, vk)"
//	@Param        body body deactivateBotRequest true "Данные чата"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/bot/v1/{service}/chat/active [patch]
func (h *BotHandler) ActivateChat(c *gin.Context) {
	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		h.logger.Warnf("ActivateChat: unsupported service param %q", c.Param("service"))
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	var req deactivateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("ActivateChat: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.ChatID == "" {
		h.logger.Warnf("ActivateChat: missing chat_id")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	if err := h.botUC.ActivateBot(c.Request.Context(), platform, req.ChatID); err != nil {
		h.logger.Errorf("ActivateChat error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to activate bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateChatName godoc
//
//	@Summary      Обновить имя чата бота
//	@Description  Обновляет имя подключённого чата на платформе
//	@Tags         bot-internal
//	@Accept       json
//	@Produce      json
//	@Param        service path string true "Платформа бота (telegram, vk)"
//	@Param        body body updateChatNameRequest true "Новое имя чата"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/bot/v1/{service}/chat/name [patch]
func (h *BotHandler) UpdateChatName(c *gin.Context) {
	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		h.logger.Warnf("UpdateChatName: unsupported service param %q", c.Param("service"))
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	var req updateChatNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("UpdateChatName: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := h.botUC.UpdateChatName(c.Request.Context(), platform, req.ChatID, req.Name); err != nil {
		if strings.Contains(err.Error(), "no application found") {
			c.JSON(http.StatusNotFound, errorResponse{Error: "bot not found for chat"})
			return
		}
		h.logger.Errorf("UpdateChatName error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to update chat name"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeactivateBot godoc
//
//	@Summary      Деактивировать чат бота
//	@Description  Деактивирует бота для указанного чата на платформе
//	@Tags         bot-internal
//	@Accept       json
//	@Produce      json
//	@Param        service path string true "Платформа бота (telegram, vk)"
//	@Param        body body deactivateBotRequest true "Данные чата"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/bot/v1/{service}/chat/active [delete]
func (h *BotHandler) DeactivateBot(c *gin.Context) {
	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		h.logger.Warnf("DeactivateBot: unsupported service param %q", c.Param("service"))
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	var req deactivateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("DeactivateBot: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.ChatID == "" {
		h.logger.Warnf("DeactivateBot: missing chat_id")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	app, err := h.botUC.GetByChatID(c.Request.Context(), platform, req.ChatID)
	if err != nil {
		h.logger.Errorf("DeactivateBot telegram lookup error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to find bot"})
		return
	}
	if app == nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: "telegram bot not found for chat"})
		return
	}
	if err := h.botUC.DeactivateBot(c.Request.Context(), app.ID); err != nil {
		h.logger.Errorf("DeactivateBot telegram error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to deactivate bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------- helpers ----------

func queryInt(c *gin.Context, key string, defaultVal int, l logger.Log) int {
	if raw := c.Query(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
		} else {
			l.Warnf("invalid query param %s=%q: %v", key, raw, err)
		}
	}
	return defaultVal
}

func isUpstreamError(err error) bool {
	return err != nil && len(err.Error()) > 9 && err.Error()[:9] == "ml client"
}

func normalizeBotService(service string) string {
	switch strings.ToLower(service) {
	case "telegram", "tg":
		return "telegram"
	case "vk":
		return "vk"
	default:
		return ""
	}
}
