package interfaces

import (
	"context"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

// ModeratorRepository — порт для работы с модераторами.
type ModeratorRepository interface {
	Create(ctx context.Context, mod *domain.Moderator) error
	GetByUsername(ctx context.Context, username string) (*domain.Moderator, error)
	GetByID(ctx context.Context, id string) (*domain.Moderator, error)
}
