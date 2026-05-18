package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"github.com/google/uuid"
)

// VkBotUseCase реализует бизнес-логику для управления Vk ботами.
type VkBotUseCase struct {
	applicationRepo         interfaces.ApplicationRepository
	applicationSettingsRepo interfaces.ApplicationSettingsRepository
	moderatorRepo           interfaces.ModeratorRepository
	vkAPI                   *api.VK
	logger                  logger.Log
}

func NewVkBotUseCase(
	applicationRepo interfaces.ApplicationRepository,
	applicationSettingsRepo interfaces.ApplicationSettingsRepository,
	moderatorRepo interfaces.ModeratorRepository,
	vkAPI *api.VK,
	l logger.Log,
) *VkBotUseCase {
	return &VkBotUseCase{
		applicationRepo:         applicationRepo,
		applicationSettingsRepo: applicationSettingsRepo,
		moderatorRepo:           moderatorRepo,
		vkAPI:                   vkAPI,
		logger:                  l,
	}
}

// GenerateToken генерирует новый токен для активации Vk бота.
func (uc *VkBotUseCase) GenerateToken(ctx context.Context, ownerID string) (*domain.Application, error) {
	// Генерируем уникальный токен
	token, err := generateVkToken()
	if err != nil {
		uc.logger.Errorf("Error generating token: %s", err)
		return nil, err
	}

	// Создаем новое приложение для Vk бота
	app := &domain.Application{
		ID:        uuid.New().String(),
		Name:      "Vk Bot",
		Platform:  "vk",
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

// ActivateBotByChatID активирует бота по ID чата (вызывается Vk ботом).
func (uc *VkBotUseCase) ActivateBotByChatID(ctx context.Context, token, chatID string) error {
	return uc.ActivateBot(ctx, token, chatID)
}

// GetByToken получает приложение по токену.
func (uc *VkBotUseCase) GetByToken(ctx context.Context, token string) (*domain.Application, error) {
	return uc.applicationRepo.GetByToken(ctx, token)
}

// GetSettings получает настройки приложения.
func (uc *VkBotUseCase) GetSettings(ctx context.Context, applicationID string) (*domain.ApplicationSettings, error) {
	return uc.applicationSettingsRepo.GetByApplicationID(ctx, applicationID)
}

// UpdateSettings обновляет настройки приложения.
func (uc *VkBotUseCase) UpdateSettings(ctx context.Context, settings *domain.ApplicationSettings) error {
	return uc.applicationSettingsRepo.Update(ctx, settings)
}

// ActivateBot активирует бота по внешнему ID чата.
func (uc *VkBotUseCase) ActivateBot(ctx context.Context, token, chatID string) error {
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
func (uc *VkBotUseCase) DisableBot(ctx context.Context, applicationID string) error {
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
func (uc *VkBotUseCase) ListBots(ctx context.Context, ownerID string) ([]*domain.Application, error) {
	return uc.applicationRepo.ListByOwner(ctx, ownerID)
}

// ListAccessibleBots возвращает боты, к которым у пользователя есть доступ (владелец или соадмин).
func (uc *VkBotUseCase) ListAccessibleBots(ctx context.Context, userID string) ([]*domain.Application, error) {
	return uc.applicationRepo.ListByOwnerOrAdmin(ctx, userID)
}

// AddAdmin добавляет соадмина по username. Только владелец может добавлять.
func (uc *VkBotUseCase) AddAdmin(ctx context.Context, ownerID, appID, targetUsername string) (*domain.Moderator, error) {
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
func (uc *VkBotUseCase) RemoveAdmin(ctx context.Context, ownerID, appID, targetUsername string) error {
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
func (uc *VkBotUseCase) GetAdmins(ctx context.Context, appID string) ([]*domain.Moderator, error) {
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

// GetByChatID возвращает приложение по внешнему ID чата (Vk chat ID).
func (uc *VkBotUseCase) GetByChatID(ctx context.Context, chatID string) (*domain.Application, error) {
	return uc.applicationRepo.GetByExternalIDAndPlatform(ctx, chatID, "vk")
}

// IsChatActive проверяет, зарегистрирован ли чат в системе.
func (uc *VkBotUseCase) IsChatActive(ctx context.Context, chatID string) (bool, error) {
	app, err := uc.applicationRepo.GetByExternalIDAndPlatform(ctx, chatID, "vk")
	if err != nil {
		return false, err
	}
	return app != nil && app.Status == "active", nil
}

// generateVkToken генерирует случайный токен.
func generateVkToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// VerifyChat проверяет, что бот находится в указанном чате и может отправлять сообщения.
// chatID должен быть числовым peer_id беседы (например, 2000000001).
func (uc *VkBotUseCase) VerifyChat(ctx context.Context, applicationID, chatID string) error {
	app, err := uc.applicationRepo.GetByID(ctx, applicationID)
	if err != nil {
		uc.logger.Errorf("vk Error getting application: %s", err)
		return err
	}
	if app == nil {
		return fmt.Errorf("application not found")
	}

	peerID, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("неверный формат chatID: ожидается числовой идентификатор беседы (peer_id)")
	}

	msg := params.NewMessagesSendBuilder()
	msg.PeerID(int(peerID)) // VK SDK принимает int для peer_id
	msg.Message("✅ Бот SpamBreaker успешно подключён к чату!")
	msg.RandomID(0)

	if _, err := uc.vkAPI.MessagesSend(msg.Params); err != nil {
		uc.logger.Errorf("Error sending verification message to vk chat %d: %s", peerID, err)
		return fmt.Errorf("не удалось отправить сообщение: возможно, бот не состоит в чате или нет прав")
	}

	app.ExternalID = strconv.FormatInt(peerID, 10) // храним как строку
	app.Status = "active"
	app.VerifiedAt = time.Now().UTC()
	app.UpdatedAt = time.Now().UTC()

	if err := uc.applicationRepo.Update(ctx, app); err != nil {
		uc.logger.Errorf("Error updating application: %s", err)
		return err
	}

	return nil
}
