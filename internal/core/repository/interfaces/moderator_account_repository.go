package interfaces

import (
	"context"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

type ModeratorAccountRepository interface {
	// Create создаёт новую запись (все поля обязательны, ID генерируется в БД).
	Create(ctx context.Context, account *domain.ModeratorAccount) error

	// FindByVerificationToken ищет аккаунт по токену (без проверки срока).
	FindByVerificationToken(ctx context.Context, token string) (*domain.ModeratorAccount, error)

	// FindByID возвращает аккаунт по первичному ключу.
	FindByID(ctx context.Context, id string) (*domain.ModeratorAccount, error)

	// FindByPlatformAndModeratorID ищет аккаунт по платформе и идентификатору.
	FindByPlatformAndModeratorID(ctx context.Context, platform, moderatorID string) (*domain.ModeratorAccount, error)

	// FindByPlatformAndAccountID ищет аккаунт по платформе и идентификатору в соц сети.
	FindByPlatformAndAccountID(ctx context.Context, platform, accountID string) (*domain.ModeratorAccount, error)

	// ListByModeratorID возвращает аккаунты модератора с фильтрами по платформе и верификации.
	ListByModeratorID(ctx context.Context, moderatorID, platform string, active *bool) ([]domain.ModeratorAccount, error)

	// VerifyAccount подтверждает аккаунт: устанавливает verified_at = now, удаляет токен.
	VerifyAccount(ctx context.Context, id, accID string) error

	// UpdateToken обновляет токен и срок действия (для повторной отправки).
	UpdateToken(ctx context.Context, id string, token string, expiresAt time.Time) error

	// Delete удаляет привязку аккаунта.
	Delete(ctx context.Context, id string) error
}
