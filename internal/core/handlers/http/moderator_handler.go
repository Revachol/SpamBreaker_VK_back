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

type initiateVerificationResponse struct {
	Token       string `json:"token"`
	ExpiresAt   string `json:"expires_at"`
	Instruction string `json:"instruction"`
}

type addAdminRequest struct {
	Username string `json:"username" binding:"required"`
}

type moderatorAccountResponse struct {
	ID         string     `json:"id"`
	Platform   string     `json:"platform"`
	AccountID  string     `json:"account_id"`
	VerifiedAt *time.Time `json:"verified_at"`
}

type moderatorAccountInfoResponse struct {
	ID             string     `json:"id"`
	ModeratorID    string     `json:"moderator_id"`
	Platform       string     `json:"platform"`
	AccountID      *string    `json:"account_id"`
	TokenExpiresAt *time.Time `json:"token_expires_at"`
	VerifiedAt     *time.Time `json:"verified_at"`
}

type userBotResponse struct {
	Name       string    `json:"name"`
	ID         string    `json:"id"`
	ExternalID string    `json:"external_id"`
	OwnerID    string    `json:"owner_id"`
	OwnAccID   string    `json:"own_acc_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// ---------- Handlers ----------

// InitiateVerification godoc
//
//	@Summary      Инициировать верификацию аккаунта
//	@Description  Создаёт одноразовый токен для подтверждения платформенного аккаунта
//	@Tags         moderator-verification
//	@Accept       json
//	@Produce      json
//	@Param        service path string true "Платформа аккаунта (telegram, vk)"
//	@Param        body body initiateVerificationRequest true "ID аккаунта"
//	@Success      200 {object} initiateVerificationResponse
//	@Failure      400 {object} errorResponse
//	@Failure      409 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/user/{service}/verify [post]
//	@Security     Bearer
func (h *ModeratorHandler) InitiateVerification(c *gin.Context) {
	userID, exists := c.Get("user_id")
	platform := c.Param("service")
	if !exists {
		h.logger.Warnf("InitiateVerification: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	if platform == "" {
		h.logger.Warnf("InitiateVerification: missing platform path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "platform is required"})
		return
	}

	token, err := h.moderatorService.InitiateVerification(
		c.Request.Context(),
		userID.(string),
		platform,
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

// GetModeratorAccounts godoc
//
//	@Summary      Получить аккаунты модератора
//	@Description  Возвращает платформенные аккаунты текущего модератора
//	@Tags         moderator-verification
//	@Produce      json
//	@Param        platform query string false "Платформа аккаунта (telegram, vk, api)"
//	@Param        active   query bool   false "Только верифицированные или неверифицированные аккаунты"
//	@Success      200 {array}  moderatorAccountResponse
//	@Failure      400 {object} errorResponse
//	@Failure      401 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/user/account [get]
//	@Security     Bearer
func (h *ModeratorHandler) GetModeratorAccounts(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("GetModeratorAccounts: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	active, err := optionalBoolQuery(c, "active", h.logger)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "active must be boolean"})
		return
	}

	accounts, err := h.moderatorService.ListModeratorAccounts(
		c.Request.Context(),
		userID.(string),
		c.Query("platform"),
		active,
	)
	if err != nil {
		h.logger.Errorf("GetModeratorAccounts error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list moderator accounts"})
		return
	}

	resp := make([]moderatorAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		resp = append(resp, moderatorAccountResponse{
			ID:         account.ID,
			Platform:   account.Platform,
			AccountID:  *account.AccountID,
			VerifiedAt: account.VerifiedAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

// GetModeratorAccountInfo godoc
//
//	@Summary      Получить аккаунт модератора
//	@Description  Возвращает платформенный аккаунт текущего модератора по ID
//	@Tags         moderator-verification
//	@Produce      json
//	@Param        accID path string true "ID аккаунта модератора"
//	@Success      200 {object} moderatorAccountInfoResponse
//	@Failure      400 {object} errorResponse
//	@Failure      401 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/user/account/{accID} [get]
//	@Security     Bearer
func (h *ModeratorHandler) GetModeratorAccountInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("GetModeratorAccountInfo: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	accID := c.Param("accID")
	if accID == "" {
		h.logger.Warnf("GetModeratorAccountInfo: missing accID path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "accID is required"})
		return
	}

	account, err := h.moderatorService.GetModeratorAccountInfo(c.Request.Context(), userID.(string), accID)
	if err != nil {
		switch err.Error() {
		case "forbidden":
			c.JSON(http.StatusForbidden, errorResponse{Error: "account does not belong to current user"})
		case "moderator_account not found":
			c.JSON(http.StatusNotFound, errorResponse{Error: "account not found"})
		default:
			h.logger.Errorf("GetModeratorAccountInfo error: %v", err)
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get moderator account"})
		}
		return
	}

	c.JSON(http.StatusOK, moderatorAccountInfoResponse{
		ID:             account.ID,
		ModeratorID:    account.ModeratorID,
		Platform:       account.Platform,
		AccountID:      account.AccountID,
		TokenExpiresAt: account.TokenExpiresAt,
		VerifiedAt:     account.VerifiedAt,
	})
}

// GetUserBots godoc
//
//	@Summary      Получить ботов пользователя
//	@Description  Возвращает приложения, к которым текущий пользователь относится
//	@Tags         user-bots
//	@Produce      json
//	@Param        platform query string false "Платформа бота (telegram, vk, api)"
//	@Param        role     query string false "Роль пользователя (admin, moderator)"
//	@Success      200 {array}  userBotResponse
//	@Failure      400 {object} errorResponse
//	@Failure      401 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/user/bot [get]
//	@Security     Bearer
func (h *ModeratorHandler) GetUserBots(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("GetUserBots: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}

	bots, err := h.moderatorService.ListUserBots(
		c.Request.Context(),
		userID.(string),
		c.Query("platform"),
		c.Query("role"),
	)
	if err != nil {
		if err.Error() == "unsupported role" {
			c.JSON(http.StatusBadRequest, errorResponse{Error: "role must be owner or admin"})
			return
		}
		h.logger.Errorf("GetUserBots error: %v", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to list user bots"})
		return
	}

	resp := make([]userBotResponse, 0, len(bots))
	for _, bot := range bots {
		resp = append(resp, userBotResponse{
			Name:       bot.Name,
			ID:         bot.ID,
			ExternalID: bot.ExternalID,
			OwnerID:    bot.OwnerID,
			OwnAccID:   bot.OwnAccID,
			CreatedAt:  bot.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

// GetAdmins godoc
//
//	@Summary      Получить список администраторов бота
//	@Description  Возвращает список соадминов активного Telegram бота пользователя
//	@Tags         telegram-bot-admins
//	@Produce      json
//	@Param        appID path string true "ID приложения"
//	@Success      200 {array} adminInfoResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/admin [get]
//	@Security     Bearer
func (h *ModeratorHandler) GetAdmins(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		h.logger.Warnf("GetAdmins: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}

	admins, err := h.moderatorService.GetAdmins(c.Request.Context(), appID)
	if err != nil {
		h.logger.Errorf("Error getting admins: %s", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get admins"})
		return
	}

	resp := make([]domain.ApplicationAdminInfo, 0, len(admins))
	for _, admin := range admins {
		resp = append(resp, domain.ApplicationAdminInfo{
			ID:        admin.ID,
			Username:  admin.Username,
			Role:      admin.Role,
			CreatedAt: admin.CreatedAt,
		})
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
//	@Param        appID path string true "ID приложения"
//	@Param        body body addAdminRequest true "Username нового администратора"
//	@Success      200 {array} adminInfoResponse
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/admin [post]
//	@Security     Bearer
func (h *ModeratorHandler) AddAdmin(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("AddAdmin: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	appID := c.Param("app_id")
	if appID == "" {
		h.logger.Warnf("AddAdmin: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}

	var req addAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("AddAdmin: bind request: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	_, err := h.moderatorService.AddAdmin(c.Request.Context(), userID.(string), appID, req.Username)
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
	c.JSON(http.StatusOK, gin.H{})
}

// RemoveAdmin godoc
//
//	@Summary      Удалить администратора бота
//	@Description  Удаляет соадмина у активного Telegram бота (только владелец)
//	@Tags         telegram-bot-admins
//	@Produce      json
//	@Param        appID path string true "ID приложения"
//	@Param        username path string true "Username администратора"
//	@Success      200 {array} nil
//	@Failure      400 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/admin/{username} [delete]
//	@Security     Bearer
func (h *ModeratorHandler) RemoveAdmin(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("RemoveAdmin: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	targetUsername := c.Param("username")
	appID := c.Param("app_id")
	if targetUsername == "" {
		h.logger.Warnf("RemoveAdmin: missing username path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "username is required"})
		return
	}
	if appID == "" {
		h.logger.Warnf("RemoveAdmin: missing app_id path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "app_id is required"})
		return
	}

	if err := h.moderatorService.RemoveAdmin(c.Request.Context(), userID.(string), appID, targetUsername); err != nil {
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

	c.JSON(http.StatusOK, gin.H{})
}

func optionalBoolQuery(c *gin.Context, key string, l logger.Log) (*bool, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		l.Warnf("invalid query param %s=%q: %v", key, raw, err)
		return nil, err
	}
	return &value, nil
}
