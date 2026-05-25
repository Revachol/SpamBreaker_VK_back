package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
)

type ModeratorService struct {
	moderatorRepo        interfaces.ModeratorRepository
	moderatorAccountRepo interfaces.ModeratorAccountRepository
	applicationRepo      interfaces.ApplicationRepository // для управления соадминами
	log                  logger.Log
}

func NewModeratorService(
	moderatorRepo interfaces.ModeratorRepository,
	moderatorAccountRepo interfaces.ModeratorAccountRepository,
	applicationRepo interfaces.ApplicationRepository,
	l logger.Log,
) *ModeratorService {
	return &ModeratorService{
		moderatorRepo:        moderatorRepo,
		moderatorAccountRepo: moderatorAccountRepo,
		applicationRepo:      applicationRepo,
		log:                  l,
	}
}

// GetByID возвращает модератора по ID.
func (s *ModeratorService) GetByID(ctx context.Context, id string) (*domain.Moderator, error) {
	return s.moderatorRepo.GetByID(ctx, id)
}

// GetByUsername возвращает модератора по username.
func (s *ModeratorService) GetByUsername(ctx context.Context, username string) (*domain.Moderator, error) {
	return s.moderatorRepo.GetByUsername(ctx, username)
}

// InitiateVerification создаёт запись moderator_account с verification_token.
// Возвращает токен для отправки боту в личные сообщения.
func (s *ModeratorService) InitiateVerification(
	ctx context.Context,
	moderatorID string,
	platform string,
	accountID string,
) (string, error) {
	existing, err := s.moderatorAccountRepo.FindByPlatformAndAccountID(ctx, platform, accountID)
	if err != nil && !errors.Is(err, expectation.ErrNotFound) {
		return "", fmt.Errorf("find existing moderator_account: %w", err)
	}
	if existing != nil && existing.VerifiedAt != nil {
		return "", fmt.Errorf("account already verified")
	}

	token, err := generateVerificationToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(1 * time.Hour)

	if existing != nil {
		if err := s.moderatorAccountRepo.UpdateToken(ctx, existing.ID, token, expires); err != nil {
			return "", err
		}
		return token, nil
	}

	acc := &domain.ModeratorAccount{
		ModeratorID:       moderatorID,
		Platform:          platform,
		AccountID:         accountID,
		VerificationToken: &token,
		TokenExpiresAt:    &expires,
	}
	if err := s.moderatorAccountRepo.Create(ctx, acc); err != nil {
		return "", err
	}
	return token, nil
}

// VerifyTelegramAccount подтверждает Telegram-аккаунт.
func (s *ModeratorService) VerifyTelegramAccount(ctx context.Context, token string, fromUserID int64) error {
	accountID := fmt.Sprintf("%d", fromUserID)
	account, err := s.moderatorAccountRepo.FindByVerificationToken(ctx, token)
	if err != nil {
		if errors.Is(err, expectation.ErrNotFound) {
			return errors.New("invalid token")
		}
		return err
	}
	if account.Platform != "telegram" {
		return errors.New("invalid platform")
	}
	if account.AccountID != accountID {
		return errors.New("token is for another telegram user")
	}
	if account.VerifiedAt != nil {
		return errors.New("account already verified")
	}
	if account.TokenExpiresAt != nil && time.Now().After(*account.TokenExpiresAt) {
		return errors.New("token expired")
	}
	return s.moderatorAccountRepo.VerifyAccount(ctx, account.ID)
}

// GetModeratorIDByVerifiedTelegramID возвращает ID модератора, если указанный Telegram ID верифицирован.
func (s *ModeratorService) GetModeratorIDByVerifiedTelegramID(ctx context.Context, telegramID int64) (string, error) {
	accountID := fmt.Sprintf("%d", telegramID)
	account, err := s.moderatorAccountRepo.FindVerifiedByPlatformAndAccountID(ctx, "telegram", accountID)
	if err != nil {
		if errors.Is(err, expectation.ErrNotFound) {
			return "", expectation.ErrNotVerified
		}
		return "", err
	}
	return account.ModeratorID, nil
}

// IsVerified проверяет, верифицирован ли аккаунт на платформе для данного модератора.
func (s *ModeratorService) IsVerified(ctx context.Context, moderatorID, platform, accountID string) (bool, error) {
	account, err := s.moderatorAccountRepo.FindByPlatformAndAccountID(ctx, platform, accountID)
	if err != nil {
		if errors.Is(err, expectation.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if account.ModeratorID != moderatorID {
		return false, nil
	}
	return account.VerifiedAt != nil, nil
}

// AddAdmin добавляет соадмина приложения. Только владелец может добавлять.
func (s *ModeratorService) AddAdmin(ctx context.Context, ownerID, appID, targetUsername string) (*domain.Moderator, error) {
	app, err := s.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app == nil || app.OwnerID != ownerID {
		return nil, fmt.Errorf("forbidden")
	}

	target, err := s.moderatorRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("user not found")
	}
	if target.ID == ownerID {
		return nil, fmt.Errorf("cannot add yourself as admin")
	}

	if err := s.applicationRepo.AddAdmin(ctx, appID, target.ID); err != nil {
		return nil, err
	}
	return target, nil
}

// RemoveAdmin удаляет соадмина. Только владелец может удалять.
func (s *ModeratorService) RemoveAdmin(ctx context.Context, ownerID, appID, targetUsername string) error {
	app, err := s.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return err
	}
	if app == nil || app.OwnerID != ownerID {
		return fmt.Errorf("forbidden")
	}

	target, err := s.moderatorRepo.GetByUsername(ctx, targetUsername)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("user not found")
	}

	return s.applicationRepo.RemoveAdmin(ctx, appID, target.ID)
}

// GetAdmins возвращает список соадминов приложения.
func (s *ModeratorService) GetAdmins(ctx context.Context, appID string) ([]*domain.Moderator, error) {
	ids, err := s.applicationRepo.ListAdminIDs(ctx, appID)
	if err != nil {
		return nil, err
	}
	var admins []*domain.Moderator
	for _, id := range ids {
		mod, err := s.moderatorRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if mod != nil {
			admins = append(admins, mod)
		}
	}
	return admins, nil
}

func generateVerificationToken() (string, error) {
	b := make([]byte, 4) // 8 hex символов
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
