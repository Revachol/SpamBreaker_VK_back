package interfaces

import (
	"context"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

type BufferRepository interface {
	Add(ctx context.Context, appID string, record domain.BMessage) error
	List(ctx context.Context, applicationID string) ([]domain.BMessage, error)
	Replace(ctx context.Context, applicationID string, records []domain.BMessage) error
	Limit() int
}
