package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
)

// Repository — потокобезопасная in-memory реализация domain.MessageRepository.
// Используется для разработки и тестов; в prod заменяется PostgreSQL-реализацией.
type Repository struct {
	mu      sync.RWMutex
	records []*domain.CheckRecord
	index   map[string]*domain.CheckRecord
}

func NewRepository() *Repository {
	return &Repository{
		index: make(map[string]*domain.CheckRecord),
	}
}

func (r *Repository) Save(_ context.Context, record *domain.CheckRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records = append(r.records, record)
	r.index[record.ID] = record
	return nil
}

func (r *Repository) List(_ context.Context, limit, offset int) ([]*domain.CheckRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.records)
	if offset >= total {
		return []*domain.CheckRecord{}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	// Возвращаем в обратном хронологическом порядке (новые первые).
	slice := make([]*domain.CheckRecord, end-offset)
	for i, rec := range r.records[offset:end] {
		slice[i] = rec
	}
	return slice, nil
}

func (r *Repository) GetByID(_ context.Context, id string) (*domain.CheckRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.index[id]
	if !ok {
		return nil, fmt.Errorf("record %q not found", id)
	}
	return rec, nil
}
