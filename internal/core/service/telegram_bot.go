package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type TelegramBotUseCase struct {
	applicationRepo         interfaces.ApplicationRepository
	applicationSettingsRepo interfaces.ApplicationSettingsRepository
	moderatorAccountRepo    interfaces.ModeratorAccountRepository // прямая зависимость от репозитория
	telegramAPI             *tgbotapi.BotAPI
	logger                  logger.Log
}

func NewTelegramBotUseCase(
	applicationRepo interfaces.ApplicationRepository,
	applicationSettingsRepo interfaces.ApplicationSettingsRepository,
	moderatorAccountRepo interfaces.ModeratorAccountRepository,
	telegramAPI *tgbotapi.BotAPI,
	l logger.Log,
) *TelegramBotUseCase {
	return &TelegramBotUseCase{
		applicationRepo:         applicationRepo,
		applicationSettingsRepo: applicationSettingsRepo,
		moderatorAccountRepo:    moderatorAccountRepo,
		telegramAPI:             telegramAPI,
		logger:                  l,
	}
}

// GenerateToken генерирует новый токен для активации Telegram бота.
func (uc *TelegramBotUseCase) GenerateToken(ctx context.Context, ownerID string) (*domain.Application, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	app := &domain.Application{
		ID:        uuid.New().String(),
		Name:      "Telegram Bot",
		Platform:  "telegram",
		Token:     token,
		OwnerID:   ownerID,
		Status:    "inactive",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := uc.applicationRepo.Create(ctx, app); err != nil {
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
		_ = uc.applicationRepo.Delete(ctx, app.ID)
		return nil, err
	}

	return app, nil
}

// ActivateBotByChatID активирует бота в чате, проверяя права верифицированного пользователя.
func (uc *TelegramBotUseCase) ActivateBotByChatID(ctx context.Context, token, chatID string, fromUserID int64) error {
	// 1. Проверяем, что пользователь верифицирован через moderator_account
	accountID := strconv.FormatInt(fromUserID, 10)
	acc, err := uc.moderatorAccountRepo.FindVerifiedByPlatformAndAccountID(ctx, "telegram", accountID)
	if err != nil {
		if errors.Is(err, expectation.ErrNotFound) {
			return fmt.Errorf("user not verified")
		}
		return fmt.Errorf("check verification: %w", err)
	}
	moderatorID := acc.ModeratorID

	// 2. Получаем приложение по токену
	app, err := uc.applicationRepo.GetByToken(ctx, token)
	if err != nil {
		return err
	}
	if app == nil {
		return nil // не раскрываем информацию
	}
	if app.OwnerID != moderatorID {
		return fmt.Errorf("only the bot owner can activate it")
	}

	app.ExternalID = chatID
	app.Status = "active"
	app.VerifiedAt = time.Now().UTC()
	app.UpdatedAt = time.Now().UTC()
	return uc.applicationRepo.Update(ctx, app)
}

// HandleBotAddedToChat вызывается при событии my_chat_member (бот добавлен в группу).
func (uc *TelegramBotUseCase) HandleBotAddedToChat(ctx context.Context, chatID int64, fromUserID int64) error {
	// Проверяем верификацию добавившего
	accountID := strconv.FormatInt(fromUserID, 10)
	_, err := uc.moderatorAccountRepo.FindVerifiedByPlatformAndAccountID(ctx, "telegram", accountID)
	if err != nil {
		if errors.Is(err, expectation.ErrNotFound) {
			return fmt.Errorf("user not verified")
		}
		return fmt.Errorf("check verification: %w", err)
	}

	// Проверка прав администратора в чате выполняется ботом до вызова этого метода.
	// Здесь можно добавить логику создания связи модератор-чат через отдельную таблицу.
	return nil
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

// DisableBot деактивирует бота.
func (uc *TelegramBotUseCase) DisableBot(ctx context.Context, applicationID string) error {
	app, err := uc.applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		return err
	}
	if app == nil {
		return nil
	}
	app.Status = "inactive"
	app.UpdatedAt = time.Now().UTC()
	return uc.applicationRepo.Update(ctx, app)
}

// ListBots возвращает ботов владельца.
func (uc *TelegramBotUseCase) ListBots(ctx context.Context, ownerID string) ([]*domain.Application, error) {
	return uc.applicationRepo.ListByOwner(ctx, ownerID)
}

// ListAccessibleBots возвращает боты, доступные пользователю.
func (uc *TelegramBotUseCase) ListAccessibleBots(ctx context.Context, userID string) ([]*domain.Application, error) {
	return uc.applicationRepo.ListByOwnerOrAdmin(ctx, userID)
}

// GetByChatID возвращает приложение по внешнему ID чата.
func (uc *TelegramBotUseCase) GetByChatID(ctx context.Context, chatID string) (*domain.Application, error) {
	return uc.applicationRepo.GetByExternalIDAndPlatform(ctx, chatID, "telegram")
}

// IsChatActive проверяет, активен ли чат.
func (uc *TelegramBotUseCase) IsChatActive(ctx context.Context, chatID string) (bool, error) {
	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, chatID, "telegram")
	if err != nil {
		return false, err
	}
	return app != nil && app.Status == "active", nil
}

// IsUserAdminOfChat проверяет, является ли пользователь администратором или создателем чата.
func (uc *TelegramBotUseCase) IsUserAdminOfChat(ctx context.Context, userID, chatID int64) (bool, error) {
	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	}
	member, err := uc.telegramAPI.GetChatMember(config)
	if err != nil {
		return false, fmt.Errorf("getChatMember: %w", err)
	}
	return member.IsAdministrator() || member.IsCreator(), nil
}

// VerifyChat проверяет, что бот находится в чате и активирует его.
func (uc *TelegramBotUseCase) VerifyChat(ctx context.Context, applicationID, chatID string) error {
	app, err := uc.applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		return err
	}
	if app == nil {
		return fmt.Errorf("application not found")
	}

	var msg tgbotapi.MessageConfig
	normalizedID := chatID

	switch {
	case strings.HasPrefix(chatID, "https://t.me/"):
		username := "@" + strings.TrimPrefix(chatID, "https://t.me/")
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

	return uc.applicationRepo.Update(ctx, app)
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
