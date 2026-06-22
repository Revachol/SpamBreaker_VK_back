package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	redis2 "github.com/Revachol/SpamBreaker_VK_back/pkg/redis"
)

var _ interfaces.BufferRepository = (*BufferRepository)(nil)

type BufferRepository struct {
	redis *redis2.Client
	limit int
	ttl   time.Duration
	log   logger.Log
}

func NewBufferRepository(
	redis *redis2.Client,
	limit int,
	ttl time.Duration,
	log logger.Log,
) *BufferRepository {
	if limit <= 0 {
		limit = 50
	}

	return &BufferRepository{
		redis: redis,
		limit: limit,
		ttl:   ttl,
		log:   log,
	}
}

func (r *BufferRepository) Add(ctx context.Context, record *domain.CheckRecord) error {
	if record.ApplicationID == "" {
		return nil
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	key := cacheKey(record.ApplicationID)
	if err := r.redis.LPush(ctx, key, string(payload)).Err(); err != nil {
		return err
	}
	if err := r.redis.LTrim(ctx, key, 0, int64(r.limit-1)).Err(); err != nil {
		return err
	}
	return r.redis.Expire(ctx, key, r.ttl).Err()
}

func (r *BufferRepository) List(ctx context.Context, applicationID string, limit int) ([]*domain.CheckRecord, error) {
	if limit <= 0 || limit > r.limit {
		limit = r.limit
	}

	items, err := r.redis.LRange(ctx, cacheKey(applicationID), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	records := make([]*domain.CheckRecord, 0, len(items))
	for _, item := range items {
		var record domain.CheckRecord
		if err := json.Unmarshal([]byte(item), &record); err != nil {
			return nil, err
		}
		records = append(records, &record)
	}

	return records, nil
}

func (r *BufferRepository) Replace(ctx context.Context, applicationID string, records []*domain.CheckRecord) error {
	key := cacheKey(applicationID)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	values := make([]any, 0, min(len(records), r.limit))
	for i, record := range records {
		if i >= r.limit {
			break
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return err
		}
		values = append(values, string(payload))
	}
	if err := r.redis.RPush(ctx, key, values...).Err(); err != nil {
		return err
	}
	return r.redis.Expire(ctx, key, r.ttl).Err()
}

func (r *BufferRepository) Limit() int {
	return r.limit
}

func cacheKey(applicationID string) string {
	return fmt.Sprintf("core:chat_messages:%s", applicationID)
}
