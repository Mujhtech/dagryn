package cache

import (
	"context"

	"github.com/mujhtech/dagryn/pkg/config"

	"github.com/mujhtech/dagryn/pkg/redis"

	"time"
)

type Cache interface {
	Get(ctx context.Context, key string, value interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

func NewCache(cfg *config.Config, redis *redis.Redis) (Cache, error) {
	switch cfg.Cache.Provider {
	case config.RedisCacheProvider:
		return NewRedisCache(redis)
	default:
		return nil, nil
	}
}
