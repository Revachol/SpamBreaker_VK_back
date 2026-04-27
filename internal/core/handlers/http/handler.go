package httphandler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/service"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Handler держит зависимости всех хендлеров.
type Handler struct {
	logger      logger.Log
	moderation  *service.ModerationUseCase
	telegramBot *service.TelegramBotUseCase
}

func NewHandler(moderation *service.ModerationUseCase, telegramBot *service.TelegramBotUseCase, l logger.Log) *Handler {
	return &Handler{moderation: moderation, telegramBot: telegramBot, logger: l}
}

// ---------- Request / Response DTOs ----------

type checkRequest struct {
	Text   string `json:"text" binding:"required"`
	ChatID string `json:"chat_id,omitempty"` // передаётся ботом для привязки к приложению
}

type checkResponse struct {
	ID         string             `json:"id"`
	Text       string             `json:"text"`
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	AllScores  map[string]float64 `json:"all_scores"`
	CreatedAt  string             `json:"created_at"`
	Threshold  float64            `json:"threshold,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
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
func (h *Handler) Check(c *gin.Context) {
	var req checkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("Bind error %s", err)
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Если бот передал chat_id — находим приложение и загружаем его настройки.
	applicationID := ""
	var threshold float64
	var bannedWords []string

	if req.ChatID != "" {
		app, err := h.telegramBot.GetByChatID(c.Request.Context(), req.ChatID)
		if err != nil {
			h.logger.Warnf("Check: GetByChatID(%q) error: %v", req.ChatID, err)
		} else if app != nil {
			applicationID = app.ID
			h.logger.Infof("Check: chat_id=%q -> application_id=%s", req.ChatID, applicationID)
			if settings, sErr := h.telegramBot.GetSettings(c.Request.Context(), applicationID); sErr == nil && settings != nil {
				threshold = float64(settings.ToxicityThreshold) / 100.0
				bannedWords = settings.BannedWords
			}
		} else {
			h.logger.Warnf("Check: chat_id=%q -> no matching application found", req.ChatID)
		}
	}

	// Проверяем запрещённые слова — если есть совпадение, пропускаем ML и сразу возвращаем negative.
	if len(bannedWords) > 0 {
		textLower := strings.ToLower(req.Text)
		for _, word := range bannedWords {
			if word != "" && strings.Contains(textLower, strings.ToLower(word)) {
				record, err := h.moderation.CheckTextForcedNegative(c.Request.Context(), req.Text, applicationID)
				if err != nil {
					c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
					return
				}
				c.JSON(http.StatusOK, checkResponse{
					ID:         record.ID,
					Text:       record.Text,
					Label:      record.Verdict.Label,
					Confidence: record.Verdict.Confidence,
					AllScores:  record.Verdict.AllScores,
					CreatedAt:  record.CreatedAt.Format("2006-01-02T15:04:05Z"),
					Threshold:  threshold,
				})
				return
			}
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
		Threshold:  threshold,
	})
}

// GetBotHistory возвращает историю сообщений, обработанных Telegram-ботом пользователя.
func (h *Handler) GetBotHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "not authenticated"})
		return
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	apps, err := h.telegramBot.ListAccessibleBots(c.Request.Context(), userID.(string))
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
func (h *Handler) GetHistory(c *gin.Context) {
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

// GetRecord godoc
//
//	@Summary     Получить запись по ID
//	@Tags        moderation
//	@Produce     json
//	@Param       id  path     string true "ID записи"
//	@Success     200 {object} checkResponse
//	@Failure     404 {object} errorResponse
//	@Router      /api/v1/history/{id} [get]
func (h *Handler) GetRecord(c *gin.Context) {
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

// ---------- helpers ----------

func queryInt(c *gin.Context, key string, defaultVal int) int {
	if raw := c.Query(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
		}
	}
	return defaultVal
}

// isUpstreamError — простая эвристика: ошибки от ML-клиента содержат "ml client".
func isUpstreamError(err error) bool {
	return err != nil && len(err.Error()) > 9 && err.Error()[:9] == "ml client"
}
