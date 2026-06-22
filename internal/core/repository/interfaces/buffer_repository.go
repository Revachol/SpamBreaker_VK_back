package interfaces

import (
	"context"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

type BufferRepository interface {
	Add(ctx context.Context, record *domain.CheckRecord) error
	List(ctx context.Context, applicationID string, limit int) ([]*domain.CheckRecord, error)
	Replace(ctx context.Context, applicationID string, records []*domain.CheckRecord) error
	Limit() int
}
