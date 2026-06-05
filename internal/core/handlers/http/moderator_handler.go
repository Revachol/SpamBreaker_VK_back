package httphandler

import (
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ModeratorHandler отвечает за верификацию платформенных аккаунтов модератора.
type ModeratorHandler struct {
	moderatorService *service.ModeratorService
	telegramBot      *service.BotUseCase
	logger           logger.Log
}

// NewModeratorHandler создаёт новый ModeratorHandler.
func NewModeratorHandler(
	moderatorService *service.ModeratorService,
	telegramBot *service.BotUseCase,
	l logger.Log,
) *ModeratorHandler {
	return &ModeratorHandler{
		moderatorService: moderatorService,
		telegramBot:      telegramBot,
		logger:           l,
	}
}

// ---------- DTOs ----------

// Верификация модератора
type initiateVerificationRequest struct {
	Platform  string `json:"platform" binding:"required"`
	AccountID string `json:"account_id" binding:"required"`
}

type initiateVerificationResponse struct {
	Token       string `json:"token"`
	ExpiresAt   string `json:"expires_at"`
	Instruction string `json:"instruction"`
}

type adminInfoResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type addAdminRequest struct {
	Username string `json:"username" binding:"required"`
}

// ---------- Handlers ----------

// InitiateVerification godoc
//
//	@Summary      Инициировать верификацию аккаунта
//	@Description  Создаёт одноразовый токен для подтверждения платформенного аккаунта
//	@Tags         moderator-verification
//	@Accept       json
//	@Produce      json
//	@Param        body body initiateVerificationRequest true "Платформа и ID аккаунта"
//	@Success      200 {object} initiateVerificationResponse
//	@Failure      400 {object} errorResponse
//	@Failure      409 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/moderator/verify/initiate [post]
//	@Security     Bearer
func (h *ModeratorHandler) InitiateVerification(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	var req initiateVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	token, err := h.moderatorService.InitiateVerification(
		c.Request.Context(),
		userID.(string),
		req.Platform,
		req.AccountID,
	)
	if err != nil {
		h.logger.Errorf("InitiateVerification error: %v", err)
		status := http.StatusInternalServerError
		if err.Error() == "account already verified" {
			status = http.StatusConflict
		}
		c.JSON(status, errorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, initiateVerificationResponse{
		Token:       token,
		ExpiresAt:   time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
		Instruction: "Отправьте этот код боту @SpamBreakerBot в личные сообщения",
	})
}

// GetAdmins godoc
//
//	@Summary      Получить список администраторов бота
//	@Description  Возвращает список соадминов активного Telegram бота пользователя
//	@Tags         telegram-bot-admins
//	@Produce      json
//	@Success      200 {array} adminInfoResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/admins [get]
//	@Security     Bearer
func (h *ModeratorHandler) GetAdmins(c *gin.Context) {
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

	admins, err := h.moderatorService.GetAdmins(c.Request.Context(), activeApp.ID)
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

// AddAdmin godoc
//
//	@Summary      Добавить администратора бота
//	@Description  Добавляет нового соадмина к активному Telegram боту (только владелец)
//	@Tags         telegram-bot-admins
//	@Accept       json
//	@Produce      json
//	@Param        body body addAdminRequest true "Username нового администратора"
//	@Success      200 {array} adminInfoResponse
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/admins [post]
//	@Security     Bearer
func (h *ModeratorHandler) AddAdmin(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req addAdminRequest
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

	added, err := h.moderatorService.AddAdmin(c.Request.Context(), userID.(string), activeApp.ID, req.Username)
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

	admins, _ := h.moderatorService.GetAdmins(c.Request.Context(), activeApp.ID)
	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
	_ = added
}

// RemoveAdmin godoc
//
//	@Summary      Удалить администратора бота
//	@Description  Удаляет соадмина у активного Telegram бота (только владелец)
//	@Tags         telegram-bot-admins
//	@Produce      json
//	@Param        username path string true "Username администратора"
//	@Success      200 {array} adminInfoResponse
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bots/telegram/admins/{username} [delete]
//	@Security     Bearer
func (h *ModeratorHandler) RemoveAdmin(c *gin.Context) {
	userID, _ := c.Get("user_id")
	targetUsername := c.Param("username")
	if targetUsername == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "username is required"})
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

	if err := h.moderatorService.RemoveAdmin(c.Request.Context(), userID.(string), activeApp.ID, targetUsername); err != nil {
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

	admins, _ := h.moderatorService.GetAdmins(c.Request.Context(), activeApp.ID)
	resp := make([]adminInfoResponse, 0, len(admins))
	for _, m := range admins {
		resp = append(resp, adminInfoResponse{ID: m.ID, Username: m.Username})
	}
	c.JSON(http.StatusOK, resp)
}
