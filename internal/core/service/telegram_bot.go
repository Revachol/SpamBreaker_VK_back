package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/google/uuid"
)

// TelegramBotUseCase реализует бизнес-логику для управления Telegram ботами.
type TelegramBotUseCase struct {
	applicationRepo         interfaces.ApplicationRepository
	applicationSettingsRepo interfaces.ApplicationSettingsRepository
	logger                  logger.Log
}

func NewTelegramBotUseCase(
	applicationRepo interfaces.ApplicationRepository,
	applicationSettingsRepo interfaces.ApplicationSettingsRepository,
	l logger.Log,
) *TelegramBotUseCase {
	return &TelegramBotUseCase{
		applicationRepo:         applicationRepo,
		applicationSettingsRepo: applicationSettingsRepo,
		logger:                  l,
	}
}

// GenerateToken генерирует новый токен для активации Telegram бота.
func (uc *TelegramBotUseCase) GenerateToken(ctx context.Context, ownerID string) (*domain.Application, error) {
	// Генерируем уникальный токен
	token, err := generateToken()
	if err != nil {
		uc.logger.Errorf("Error generating token: %s", err)
		return nil, err
	}

	// Создаем новое приложение для Telegram бота
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

	// Сохраняем приложение
	if err := uc.applicationRepo.Create(ctx, app); err != nil {
		uc.logger.Errorf("Error creating application: %s", err)
		return nil, err
	}

	// Создаем настройки по умолчанию
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
		// Откатываем создание приложения
		_ = uc.applicationRepo.Delete(ctx, app.ID)
		return nil, err
	}

	return app, nil
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

// generateToken генерирует случайный токен.
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
