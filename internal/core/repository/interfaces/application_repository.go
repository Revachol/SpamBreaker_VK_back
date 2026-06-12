package interfaces

import (
	"context"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

// ApplicationRepository — порт для работы с приложениями (Telegram боты, VK боты, API клиенты).
type ApplicationRepository interface {
	Create(ctx context.Context, app *domain.Application) error
	GetByID(ctx context.Context, id string) (*domain.Application, error)
	GetByToken(ctx context.Context, token string) (*domain.Application, error)
	GetByExternalIDAndPlatform(ctx context.Context, platform string, externalID string) (*domain.Application, error)
	Update(ctx context.Context, app *domain.Application) error
	Delete(ctx context.Context, id string) error
	ListByOwner(ctx context.Context, ownerID string) ([]*domain.Application, error)
	ListByOwnerOrAdmin(ctx context.Context, userID, platform, role string) ([]*domain.Application, error)
	AddAdmin(ctx context.Context, appID, moderatorID string) error
	RemoveAdmin(ctx context.Context, appID, moderatorID string) error
	ListAdminIDs(ctx context.Context, appID string) ([]string, error)
}
