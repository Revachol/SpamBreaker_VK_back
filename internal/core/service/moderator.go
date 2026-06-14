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
	logger               logger.Log
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
		logger:               l,
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
) (string, error) {
	existing, err := s.moderatorAccountRepo.FindByPlatformAndModeratorID(ctx, platform, moderatorID)
	if err != nil && !errors.Is(err, expectation.ErrNotFound) {
		return "", fmt.Errorf("find existing moderator_account: %w", err)
	}
	if existing != nil && existing.VerifiedAt != nil {
		return "", fmt.Errorf("account already verified")
	}

	if existing != nil && existing.VerifiedAt != nil && !existing.TokenExpiresAt.After(time.Now()) {
		return *existing.VerificationToken, nil
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
		AccountID:         nil,
		VerificationToken: &token,
		TokenExpiresAt:    &expires,
	}
	if err := s.moderatorAccountRepo.Create(ctx, acc); err != nil {
		return "", err
	}
	return token, nil
}

// VerifyAccount подтверждает Telegram-аккаунт.
func (s *ModeratorService) VerifyAccount(ctx context.Context, platform, token, accountID string) error {
	account, err := s.moderatorAccountRepo.FindByVerificationToken(ctx, token)
	if err != nil {
		if errors.Is(err, expectation.ErrNotFound) {
			return errors.New("invalid token")
		}
		return err
	}
	if account.Platform != platform {
		return errors.New("invalid platform")
	}
	if account.VerifiedAt != nil {
		return errors.New("account already verified")
	}
	if account.TokenExpiresAt != nil && time.Now().After(*account.TokenExpiresAt) {
		return errors.New("token expired")
	}
	return s.moderatorAccountRepo.VerifyAccount(ctx, account.ID, accountID)
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

// ListModeratorAccounts возвращает аккаунты модератора с фильтрами по платформе и верификации.
func (s *ModeratorService) ListModeratorAccounts(ctx context.Context, moderatorID, platform string, active *bool) ([]domain.ModeratorAccount, error) {
	return s.moderatorAccountRepo.ListByModeratorID(ctx, moderatorID, platform, active)
}

// GetModeratorAccountInfo возвращает аккаунт модератора, если он принадлежит текущему пользователю.
func (s *ModeratorService) GetModeratorAccountInfo(ctx context.Context, moderatorID, accID string) (*domain.ModeratorAccount, error) {
	account, err := s.moderatorAccountRepo.FindByID(ctx, accID)
	if err != nil {
		return nil, err
	}
	if account == nil || account.ModeratorID != moderatorID {
		return nil, fmt.Errorf("forbidden")
	}
	return account, nil
}

// GetOwnedAppInfo возвращает приложение, если текущий пользователь является владельцем.
func (s *ModeratorService) GetOwnedAppInfo(ctx context.Context, userID, appID string) (*domain.Application, error) {
	app, err := s.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("not found")
	}
	if app.OwnerID != userID {
		return nil, fmt.Errorf("forbidden")
	}
	return app, nil
}

// ListUserBots возвращает приложения, к которым пользователь относится как владелец или соадмин.
func (s *ModeratorService) ListUserBots(ctx context.Context, userID, platform, role string) ([]*domain.Application, error) {
	switch role {
	case "", "moderator", "admin":
	default:
		return nil, fmt.Errorf("unsupported role")
	}
	return s.applicationRepo.ListByOwnerOrAdmin(ctx, userID, platform, role)
}

// CheckUserOwnApp проверяет, что пользователь является владельцем приложения.
func (s *ModeratorService) CheckUserOwnApp(ctx context.Context, userID, appID string) error {
	app, err := s.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return err
	}
	if app == nil || app.OwnerID != userID {
		return fmt.Errorf("forbidden")
	}
	return nil
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
func (s *ModeratorService) RemoveAdmin(ctx context.Context, ownerID, appID, modID string) error {
	app, err := s.applicationRepo.GetByID(ctx, appID)
	if err != nil {
		return err
	}
	if app == nil || app.OwnerID != ownerID {
		return fmt.Errorf("forbidden")
	}

	return s.applicationRepo.RemoveAdmin(ctx, appID, modID)
}

// GetAdmins возвращает список соадминов приложения.
func (s *ModeratorService) GetAdmins(ctx context.Context, appID string) ([]domain.ApplicationAdminInfo, error) {
	return s.applicationRepo.ListAdmins(ctx, appID)
}

func generateVerificationToken() (string, error) {
	b := make([]byte, 4) // 8 hex символов
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
