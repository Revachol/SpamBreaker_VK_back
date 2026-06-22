package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	goredis "github.com/redis/go-redis/v9"
)

type Client = goredis.Client

func NewConnect(ctx context.Context, cfg *config.RedisConfig) (*Client, error) {
	if cfg.Port == "" {
		return nil, fmt.Errorf("Redis port is empty")
	}

	port, err := strconv.Atoi(cfg.Port)
	if err != nil {
		return nil, fmt.Errorf("failed to convert port to int: %w", err)
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}
