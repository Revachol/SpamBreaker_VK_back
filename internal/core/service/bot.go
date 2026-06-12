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

type BotUseCase struct {
	applicationRepo         interfaces.ApplicationRepository
	applicationSettingsRepo interfaces.ApplicationSettingsRepository
	moderatorAccountRepo    interfaces.ModeratorAccountRepository
	telegramAPI             *tgbotapi.BotAPI
	logger                  logger.Log
}

func NewBotUseCase(
	applicationRepo interfaces.ApplicationRepository,
	applicationSettingsRepo interfaces.ApplicationSettingsRepository,
	moderatorAccountRepo interfaces.ModeratorAccountRepository,
	telegramAPI *tgbotapi.BotAPI,
	l logger.Log,
) *BotUseCase {
	return &BotUseCase{
		applicationRepo:         applicationRepo,
		applicationSettingsRepo: applicationSettingsRepo,
		moderatorAccountRepo:    moderatorAccountRepo,
		telegramAPI:             telegramAPI,
		logger:                  l,
	}
}

// ActivateBotByChatID активирует бота в чате, проверяя права верифицированного пользователя.
func (uc *BotUseCase) ActivateBotByChatID(ctx context.Context, token, chatID string, fromUserID int64) error {
	// 1. Проверяем, что пользователь верифицирован через moderator_account
	accountID := strconv.FormatInt(fromUserID, 10)
	acc, err := uc.moderatorAccountRepo.FindByPlatformAndAccountID(ctx, "telegram", accountID)
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

// AddChat создаёт приложение для подключённого чата или обновляет владельца существующего.
func (uc *BotUseCase) AddChat(ctx context.Context, platform, name, accID, chatID string) (*domain.Application, error) {
	acc, err := uc.moderatorAccountRepo.FindByPlatformAndAccountID(ctx, platform, accID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		uc.logger.Warnf("%s account %s not found", platform, accID)
		return nil, fmt.Errorf("account not found")
	}

	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, platform, chatID)
	if err != nil {
		return nil, err
	}
	if app != nil {
		if app.OwnerID != "" && app.OwnerID != acc.ModeratorID {
			return nil, fmt.Errorf("chat already connected")
		}
		app.Name = name
		app.OwnerID = acc.ModeratorID
		app.OwnAccID = acc.ID
		app.UpdatedAt = time.Now().UTC()
		if err := uc.applicationRepo.Update(ctx, app); err != nil {
			return nil, err
		}
		return app, nil
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	app = &domain.Application{
		ID:         uuid.New().String(),
		Name:       name,
		Platform:   platform,
		ExternalID: chatID,
		Token:      token,
		OwnerID:    acc.ModeratorID,
		OwnAccID:   acc.ID,
		Status:     "inactive",
		CreatedAt:  now,
		UpdatedAt:  now,
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
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := uc.applicationSettingsRepo.Create(ctx, settings); err != nil {
		_ = uc.applicationRepo.Delete(ctx, app.ID)
		return nil, err
	}

	return app, nil
}

// GetSettings получает настройки приложения.
func (uc *BotUseCase) GetSettings(ctx context.Context, applicationID string) (*domain.ApplicationSettings, error) {
	return uc.applicationSettingsRepo.GetByApplicationID(ctx, applicationID)
}

// UpdateSettings обновляет настройки приложения.
func (uc *BotUseCase) UpdateSettings(ctx context.Context, settings *domain.ApplicationSettings) error {
	return uc.applicationSettingsRepo.Update(ctx, settings)
}

// DisableBot деактивирует бота.
func (uc *BotUseCase) DisableBot(ctx context.Context, applicationID string) error {
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

// UpdateChatName обновляет имя подключённого чата.
func (uc *BotUseCase) UpdateChatName(ctx context.Context, platform, chatID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("chat name is required")
	}

	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, platform, chatID)
	if err != nil {
		return err
	}
	if app == nil {
		return fmt.Errorf("no application found")
	}

	app.Name = name
	app.UpdatedAt = time.Now().UTC()
	return uc.applicationRepo.Update(ctx, app)
}

// ActivateBot активирует бота.
func (uc *BotUseCase) ActivateBot(ctx context.Context, platform, chatID string) error {
	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, platform, chatID)
	if err != nil {
		return err
	}
	if app == nil {
		uc.logger.Warnf("no app with id: %s found for platform: %s", chatID, platform)
		return errors.New("no application found")
	}
	app.Status = "active"
	app.VerifiedAt = time.Now().UTC()
	app.UpdatedAt = time.Now().UTC()
	return uc.applicationRepo.Update(ctx, app)
}

// GetByChatID возвращает приложение по внешнему ID чата.
func (uc *BotUseCase) GetByChatID(ctx context.Context, platform, chatID string) (*domain.Application, error) {
	return uc.applicationRepo.GetByExternalIDAndPlatform(ctx, platform, chatID)
}

// IsChatActive проверяет, активен ли чат.
func (uc *BotUseCase) IsChatActive(ctx context.Context, platform, chatID string) (bool, error) {
	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, platform, chatID)
	if err != nil {
		return false, err
	}
	return app != nil && app.Status == "active", nil
}

// VerifyChat проверяет, что бот находится в чате и активирует его.
func (uc *BotUseCase) VerifyChat(ctx context.Context, applicationID, chatID string) error {
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
