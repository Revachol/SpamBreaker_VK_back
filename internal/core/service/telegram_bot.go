package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// TelegramBotUseCase реализует бизнес-логику для управления Telegram ботами.
type TelegramBotUseCase struct {
	applicationRepo         interfaces.ApplicationRepository
	applicationSettingsRepo interfaces.ApplicationSettingsRepository
	moderatorRepo           interfaces.ModeratorRepository
	telegramAPI             *tgbotapi.BotAPI
	logger                  logger.Log
}

func NewTelegramBotUseCase(
	applicationRepo interfaces.ApplicationRepository,
	applicationSettingsRepo interfaces.ApplicationSettingsRepository,
	moderatorRepo interfaces.ModeratorRepository,
	telegramAPI *tgbotapi.BotAPI,
	l logger.Log,
) *TelegramBotUseCase {
	return &TelegramBotUseCase{
		applicationRepo:         applicationRepo,
		applicationSettingsRepo: applicationSettingsRepo,
		moderatorRepo:           moderatorRepo,
		telegramAPI:             telegramAPI,
		logger:                  l,
	}
}

// CreateBot создаёт нового Telegram-бота с заданным именем.
func (uc *TelegramBotUseCase) CreateBot(ctx context.Context, ownerID, name string) (*domain.Application, error) {
	token, err := generateToken()
	if err != nil {
		uc.logger.Errorf("Error generating token: %s", err)
		return nil, err
	}

	if name == "" {
		name = "Telegram Bot"
	}

	app := &domain.Application{
		ID:        uuid.New().String(),
		Name:      name,
		Platform:  "telegram",
		Token:     token,
		OwnerID:   ownerID,
		Status:    "inactive",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := uc.applicationRepo.Create(ctx, app); err != nil {
		uc.logger.Errorf("Error creating application: %s", err)
		return nil, err
	}

	settings := &domain.ApplicationSettings{
		ID:                uuid.New().String(),
		ApplicationID:     app.ID,
		ToxicityThreshold: 70,
		ActionOnSpam:      "notify",
		AutoModerate:      false,
		NotifyModerator:   true,
		AllowedLanguages:  nil,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if err := uc.applicationSettingsRepo.Create(ctx, settings); err != nil {
		uc.logger.Errorf("Error creating application settings: %s", err)
		_ = uc.applicationRepo.Delete(ctx, app.ID)
		return nil, err
	}

	return app, nil
}

// GenerateToken генерирует новый токен для активации Telegram бота.
// Deprecated: используйте CreateBot.
func (uc *TelegramBotUseCase) GenerateToken(ctx context.Context, ownerID string) (*domain.Application, error) {
	return uc.CreateBot(ctx, ownerID, "Telegram Bot")
}

// ListTelegramBots возвращает только Telegram-ботов владельца.
func (uc *TelegramBotUseCase) ListTelegramBots(ctx context.Context, ownerID string) ([]*domain.Application, error) {
	apps, err := uc.applicationRepo.ListByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	var result []*domain.Application
	for _, a := range apps {
		if a.Platform == "telegram" {
			result = append(result, a)
		}
	}
	return result, nil
}

// GetBot возвращает конкретного бота. Проверяет, что пользователь — владелец или соадмин.
func (uc *TelegramBotUseCase) GetBot(ctx context.Context, userID, botID string) (*domain.Application, error) {
	app, err := uc.applicationRepo.GetByID(ctx, botID)
	if err != nil {
		return nil, err
	}
	if app == nil || app.Platform != "telegram" {
		return nil, fmt.Errorf("bot not found")
	}
	if app.OwnerID == userID {
		return app, nil
	}
	adminIDs, err := uc.applicationRepo.ListAdminIDs(ctx, botID)
	if err != nil {
		return nil, err
	}
	for _, id := range adminIDs {
		if id == userID {
			return app, nil
		}
	}
	return nil, fmt.Errorf("forbidden")
}

// DeleteBot удаляет бота. Только владелец может удалять.
func (uc *TelegramBotUseCase) DeleteBot(ctx context.Context, ownerID, botID string) error {
	app, err := uc.applicationRepo.GetByID(ctx, botID)
	if err != nil {
		return err
	}
	if app == nil || app.Platform != "telegram" {
		return fmt.Errorf("bot not found")
	}
	if app.OwnerID != ownerID {
		return fmt.Errorf("forbidden")
	}
	return uc.applicationRepo.Delete(ctx, botID)
}

// RenameBot переименовывает бота. Только владелец может переименовывать.
func (uc *TelegramBotUseCase) RenameBot(ctx context.Context, ownerID, botID, name string) error {
	app, err := uc.applicationRepo.GetByID(ctx, botID)
	if err != nil {
		return err
	}
	if app == nil || app.Platform != "telegram" {
		return fmt.Errorf("bot not found")
	}
	if app.OwnerID != ownerID {
		return fmt.Errorf("forbidden")
	}
	app.Name = name
	app.UpdatedAt = time.Now().UTC()
	return uc.applicationRepo.Update(ctx, app)
}

// ActivateBotByChatID активирует бота по ID чата (вызывается Telegram ботом).
func (uc *TelegramBotUseCase) ActivateBotByChatID(ctx context.Context, token, chatID string) error {
	return uc.ActivateBot(ctx, token, chatID)
}

// GetByToken получает приложение по токену.
func (uc *TelegramBotUseCase) GetByToken(ctx context.Context, token string) (*domain.Application, error) {
	return uc.applicationRepo.GetByToken(ctx, token)
}

// GetSettings получает настройки приложения.
func (uc *TelegramBotUseCase) GetSettings(ctx context.Context, applicationID string) (*domain.ApplicationSettings, error) {
	return uc.applicationSettingsRepo.GetByApplicationID(ctx, applicationID)
}

// UpdateSettings обновляет настройки приложения.
func (uc *TelegramBotUseCase) UpdateSettings(ctx context.Context, settings *domain.ApplicationSettings) error {
	return uc.applicationSettingsRepo.Update(ctx, settings)
}

// ActivateBot активирует бота по внешнему ID чата.
func (uc *TelegramBotUseCase) ActivateBot(ctx context.Context, token, chatID string) error {
	// Получаем приложение по токену
	app, err := uc.applicationRepo.GetByToken(ctx, token)
	if err != nil {
		uc.logger.Errorf("Error getting application by token: %s", err)
		return err
	}

	if app == nil {
		uc.logger.Warnf("Application not found for token: %s", token)
		return nil // Не возвращаем ошибку, чтобы не раскрывать информацию
	}

	// Обновляем приложение
	app.ExternalID = chatID
	app.Status = "active"
	app.VerifiedAt = time.Now().UTC()
	app.UpdatedAt = time.Now().UTC()

	if err := uc.applicationRepo.Update(ctx, app); err != nil {
		uc.logger.Errorf("Error updating application: %s", err)
		return err
	}

	return nil
}

// DisableBot деактивирует бота.
func (uc *TelegramBotUseCase) DisableBot(ctx context.Context, applicationID string) error {
	// Получаем приложение
	app, err := uc.applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		uc.logger.Errorf("Error getting application: %s", err)
		return err
	}

	if app == nil {
		uc.logger.Warnf("Application not found: %s", applicationID)
		return nil
	}

	// Обновляем статус
	app.Status = "inactive"
	app.UpdatedAt = time.Now().UTC()

	if err := uc.applicationRepo.Update(ctx, app); err != nil {
		uc.logger.Errorf("Error updating application: %s", err)
		return err
	}

	return nil
}

// ListBots возвращает список ботов владельца.
func (uc *TelegramBotUseCase) ListBots(ctx context.Context, ownerID string) ([]*domain.Application, error) {
	return uc.applicationRepo.ListByOwner(ctx, ownerID)
}

// ListAccessibleBots возвращает боты, к которым у пользователя есть доступ (владелец или соадмин).
func (uc *TelegramBotUseCase) ListAccessibleBots(ctx context.Context, userID string) ([]*domain.Application, error) {
	return uc.applicationRepo.ListByOwnerOrAdmin(ctx, userID)
}

// AddAdmin добавляет соадмина по username. Только владелец может добавлять.
func (uc *TelegramBotUseCase) AddAdmin(ctx context.Context, ownerID, appID, targetUsername string) (*domain.Moderator, error) {
	app, err := uc.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app == nil || app.OwnerID != ownerID {
		return nil, fmt.Errorf("forbidden")
	}

	target, err := uc.moderatorRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("user not found")
	}
	if target.ID == ownerID {
		return nil, fmt.Errorf("cannot add yourself as admin")
	}

	if err := uc.applicationRepo.AddAdmin(ctx, appID, target.ID); err != nil {
		return nil, err
	}
	return target, nil
}

// RemoveAdmin удаляет соадмина по username. Только владелец может удалять.
func (uc *TelegramBotUseCase) RemoveAdmin(ctx context.Context, ownerID, appID, targetUsername string) error {
	app, err := uc.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return err
	}
	if app == nil || app.OwnerID != ownerID {
		return fmt.Errorf("forbidden")
	}

	target, err := uc.moderatorRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	return uc.applicationRepo.RemoveAdmin(ctx, appID, target.ID)
}

// GetAdmins возвращает список соадминов приложения.
func (uc *TelegramBotUseCase) GetAdmins(ctx context.Context, appID string) ([]*domain.Moderator, error) {
	ids, err := uc.applicationRepo.ListAdminIDs(ctx, appID)
	if err != nil {
		return nil, err
	}

	var admins []*domain.Moderator
	for _, id := range ids {
		mod, err := uc.moderatorRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if mod != nil {
			admins = append(admins, mod)
		}
	}
	return admins, nil
}

// GetByChatID возвращает приложение по внешнему ID чата (Telegram chat ID).
func (uc *TelegramBotUseCase) GetByChatID(ctx context.Context, chatID string) (*domain.Application, error) {
	return uc.applicationRepo.GetByExternalIDAndPlatform(ctx, chatID, "telegram")
}

// IsChatActive проверяет, зарегистрирован ли чат в системе.
func (uc *TelegramBotUseCase) IsChatActive(ctx context.Context, chatID string) (bool, error) {
	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, chatID, "telegram")
	if err != nil {
		return false, err
	}
	return app != nil && app.Status == "active", nil
}

// generateToken генерирует случайный токен.
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// VerifyChat проверяет, что бот находится в указанном чате и имеет необходимые права.
// chatID может быть числовым ID, @username или ссылкой https://t.me/username.
func (uc *TelegramBotUseCase) VerifyChat(ctx context.Context, applicationID, chatID string) error {
	app, err := uc.applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		uc.logger.Errorf("Error getting application: %s", err)
		return err
	}
	if app == nil {
		return fmt.Errorf("application not found")
	}

	var msg tgbotapi.MessageConfig
	normalizedID := chatID

	switch {
	case strings.HasPrefix(chatID, "https://t.me/"):
		// Извлекаем username из ссылки
		username := "@" + strings.TrimPrefix(chatID, "https://t.me/")
		// Убираем возможные query-параметры
		if idx := strings.IndexByte(username, '?'); idx != -1 {
			username = username[:idx]
		}
		normalizedID = username
		msg = tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{ChannelUsername: username},
			Text:     "✅ Бот SpamBreaker успешно подключён к чату!",
		}
	case strings.HasPrefix(chatID, "@"):
		normalizedID = chatID
		msg = tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{ChannelUsername: chatID},
			Text:     "✅ Бот SpamBreaker успешно подключён к чату!",
		}
	default:
		chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
		if err != nil {
			return fmt.Errorf("неверный формат: ожидается @username, ссылка https://t.me/username или числовой ID")
		}
		msg = tgbotapi.NewMessage(chatIDInt, "✅ Бот SpamBreaker успешно подключён к чату!")
	}

	if _, err := uc.telegramAPI.Send(msg); err != nil {
		uc.logger.Errorf("Error sending verification message to chat %q: %s", normalizedID, err)
		return fmt.Errorf("бот не найден в чате или нет прав на отправку сообщений")
	}

	app.ExternalID = normalizedID
	app.Status = "active"
	app.VerifiedAt = time.Now().UTC()
	app.UpdatedAt = time.Now().UTC()

	if err := uc.applicationRepo.Update(ctx, app); err != nil {
		uc.logger.Errorf("Error updating application: %s", err)
		return err
	}

	return nil
}
