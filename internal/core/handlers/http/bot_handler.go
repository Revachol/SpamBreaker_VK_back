package httphandler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
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
	Text   string `json:"text" binding:"required"`
	ChatID string `json:"chat_id,omitempty"`
}

type checkResponse struct {
	ID         string             `json:"id"`
	Text       string             `json:"text"`
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	AllScores  map[string]float64 `json:"all_scores"`
	CreatedAt  string             `json:"created_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type activateBotRequest struct {
	Token  string `json:"token" binding:"required"`
	ChatID string `json:"chat_id" binding:"required"`
	UserID int64  `json:"user_id" binding:"required"`
}

type deactivateBotRequest struct {
	ChatID string `json:"chat_id"`
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

type verificationStatusResponse struct {
	Verified bool   `json:"verified"`
	Platform string `json:"platform"`
}

// ---------- Handlers ----------

// Check godoc
//
//	@Summary     Проверить текст
//	@Description Отправляет текст в ML-сервис и возвращает метку тональности
//	@Tags        moderation
//	@Accept      json
//	@Produce     json
//	@Param       body body     checkRequest  true "Текст для проверки"
//	@Success     200  {object} checkResponse
//	@Failure     400  {object} errorResponse
//	@Failure     502  {object} errorResponse
//	@Router      /api/v1/check [post]
func (h *BotHandler) Check(c *gin.Context) {
	var req checkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("Bind error %s", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	platform := c.Param("service")

	applicationID := ""
	if req.ChatID != "" {
		app, err := h.botUC.GetByChatID(c.Request.Context(), platform, req.ChatID)
		if err != nil {
			h.logger.Warnf("Check: GetByChatID(%q) error: %v", req.ChatID, err)
		} else if app != nil {
			applicationID = app.ID
			h.logger.Infof("Check: chat_id=%q -> application_id=%s", req.ChatID, applicationID)
		} else {
			h.logger.Warnf("Check: chat_id=%q -> no matching application found", req.ChatID)
		}
	}

	record, err := h.moderation.CheckText(c.Request.Context(), req.Text, applicationID)
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
		Text:       record.Text,
		Label:      record.Verdict.Label,
		Confidence: record.Verdict.Confidence,
		AllScores:  record.Verdict.AllScores,
		CreatedAt:  record.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// GetBotHistory возвращает историю сообщений, обработанных Telegram-ботом пользователя.
func (h *BotHandler) GetBotHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "not authenticated"})
		return
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	apps, err := h.botUC.ListAccessibleBots(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Errorf("GetBotHistory: list bots: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list bots"})
		return
	}

	var appID string
	for _, app := range apps {
		if app.Platform == "telegram" && app.Status == "active" {
			appID = app.ID
			break
		}
	}
	if appID == "" {
		c.JSON(http.StatusNotFound, errorResponse{Error: "no active telegram bot"})
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
	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)

	records, err := h.moderation.GetHistory(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	resp := make([]checkResponse, 0, len(records))
	for _, r := range records {
		resp = append(resp, checkResponse{
			ID:         r.ID,
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

	record, err := h.moderation.GetRecord(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, checkResponse{
		ID:         record.ID,
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
//	@Param        body body verifyUserTokenRequest true "Токен и Telegram ID пользователя"
//	@Success      200 {object} map[string]bool
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/verify-user [post]
func (h *BotHandler) VerifyUserToken(c *gin.Context) {
	var req verifyUserTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	platform := c.Param("service")

	err := h.moderatorService.VerifyTelegramAccount(c.Request.Context(), platform, req.Token, req.UserID)
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

// DeactivateBot deactivates a bot by platform and external chat ID.
func (h *BotHandler) DeactivateBot(c *gin.Context) {
	var req deactivateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.ChatID == "" {
		req.ChatID = c.Query("chat_id")
	}
	if req.ChatID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "chat_id is required"})
		return
	}

	platform := strings.ToLower(c.Param("service"))
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
	if err := h.botUC.DisableBot(c.Request.Context(), app.ID); err != nil {
		h.logger.Errorf("DeactivateBot telegram error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to deactivate bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DisableBot godoc
//
//	@Summary      Отключить Telegram бота
//	@Description  Деактивирует активного Telegram бота пользователя
//	@Tags         telegram-bot
//	@Produce      JSON
//	@Success      200 {object} map[string]bool
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/disable [post]
//	@Security     Bearer
func (h *BotHandler) DisableBot(c *gin.Context) {
	userID, _ := c.Get("user_id")

	apps, err := h.botUC.ListAccessibleBots(c.Request.Context(), userID.(string))
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

	if err := h.botUC.DisableBot(c.Request.Context(), activeApp.ID); err != nil {
		h.logger.Errorf("Error disabling bot: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to disable bot"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetVerificationStatus godoc
//
//	@Summary      Проверить статус верификации аккаунта
//	@Description  Возвращает true, если указанный аккаунт верифицирован для текущего пользователя
//	@Tags         moderator-verification
//	@Produce      json
//	@Param        platform   query string true "Платформа (vk, telegram, api)"
//	@Param        account_id query string true "ID аккаунта на платформе"
//	@Success      200 {object} verificationStatusResponse
//	@Failure      400 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/moderator/verify/status [get]
//	@Security     Bearer
func (h *BotHandler) GetVerificationStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	platform := c.Query("platform")
	accountID := c.Query("account_id")
	if platform == "" || accountID == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "platform and account_id are required"})
		return
	}

	verified, err := h.moderatorService.IsVerified(
		c.Request.Context(),
		userID.(string),
		platform,
		accountID,
	)
	if err != nil {
		h.logger.Errorf("GetVerificationStatus error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to check status"})
		return
	}

	c.JSON(http.StatusOK, verificationStatusResponse{
		Verified: verified,
		Platform: platform,
	})
}

// ActivateAddChat godoc
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
func (h *BotHandler) ActivateAddChat(c *gin.Context) {
	var req AddChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	platform := normalizeBotService(c.Param("service"))
	if platform == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "unsupported service"})
		return
	}

	moderatorID, err := h.moderatorService.GetModeratorIDByVerifiedAccount(c.Request.Context(), platform, req.UserID)
	if err != nil {
		h.logger.Warnf("ActivateAddChat: unverified user %s: %v", req.UserID, err)
		c.JSON(http.StatusForbidden, errorResponse{Error: "пользователь не верифицирован"})
		return
	}
	if !req.IsAdmin {
		c.JSON(http.StatusForbidden, AddChatResponse{Verified: false, Message: "пользователь не является администратором чата"})
		return
	}

	if _, err := h.botUC.AddChat(c.Request.Context(), moderatorID, platform, req.ChatID); err != nil {
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

// ---------- helpers ----------

func queryInt(c *gin.Context, key string, defaultVal int) int {
	if raw := c.Query(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
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
