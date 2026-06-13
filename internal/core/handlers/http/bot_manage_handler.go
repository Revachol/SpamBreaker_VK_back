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

type botInfoResponse struct {
	Name       string    `json:"name"`
	ID         string    `json:"id"`
	Platform   string    `json:"platform"`
	ExternalID string    `json:"external_id"`
	OwnerID    string    `json:"owner_id"`
	OwnAccID   string    `json:"own_acc_id"`
	Status     string    `json:"status"`
	VerifiedAt time.Time `json:"verified_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AddChatRequest struct {
	Name    string `json:"name"`
	ChatID  string `json:"chat_id" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
	IsAdmin bool   `json:"is_admin" binding:"required"`
}

type AddChatResponse struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// ---------- Handlers ----------

// GetInfo godoc
//
//	@Summary      Получить информацию о боте
//	@Description  Возвращает приложение по ID, если текущий пользователь является владельцем
//	@Tags         telegram-bot
//	@Produce      json
//	@Param        appID path string true "ID приложения"
//	@Success      200 {object} botInfoResponse
//	@Failure      400 {object} errorResponse
//	@Failure      401 {object} errorResponse
//	@Failure      403 {object} errorResponse
//	@Failure      404 {object} errorResponse
//	@Failure      500 {object} errorResponse
//	@Router       /api/v1/bot/{appID}/ [get]
//	@Security     Bearer
func (h *BotManageHandler) GetInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		h.logger.Warnf("GetInfo: missing user_id in context")
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "user not authenticated"})
		return
	}
	appID := c.Param("appID")
	if appID == "" {
		h.logger.Warnf("GetInfo: missing appID path param")
		c.JSON(http.StatusBadRequest, errorResponse{Error: "appID is required"})
		return
	}

	app, err := h.moderator.GetOwnedAppInfo(c.Request.Context(), userID.(string), appID)
	if err != nil {
		switch err.Error() {
		case "forbidden":
			c.JSON(http.StatusForbidden, errorResponse{Error: "bot does not belong to current user"})
		case "not found":
			c.JSON(http.StatusNotFound, errorResponse{Error: "bot not found"})
		default:
			h.logger.Errorf("GetInfo error: %v", err)
			c.JSON(http.StatusInternalServerError, errorResponse{Error: "failed to get bot info"})
		}
		return
	}

	c.JSON(http.StatusOK, botInfoResponse{
		Name:       app.Name,
		ID:         app.ID,
		Platform:   app.Platform,
		ExternalID: app.ExternalID,
		OwnerID:    app.OwnerID,
		OwnAccID:   app.OwnAccID,
		Status:     app.Status,
		VerifiedAt: app.VerifiedAt,
		CreatedAt:  app.CreatedAt,
		UpdatedAt:  app.UpdatedAt,
	})
}

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
//	@Accept       json
//	@Produce      json
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
//	@Produce      json
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
