package interfaces

import (
	"context"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

// ---------- Port: хранилище ----------

type MessageRepository interface {
	Save(ctx context.Context, record *domain.CheckRecord) error
	List(ctx context.Context, limit, offset int) ([]*domain.CheckRecord, error)
	GetByID(ctx context.Context, id string) (*domain.CheckRecord, error)
}
