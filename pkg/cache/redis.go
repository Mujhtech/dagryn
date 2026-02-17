package cache

import (
	"context"
	"errors"
	"fmt"

	"time"

	"github.com/mujhtech/dagryn/pkg/redis"

	"github.com/go-redis/cache/v9"
)

var (
	ErrCacheKeyIsMissing = fmt.Errorf("cache: key is missing")
)

const cacheSize = 128000

type RedisCache struct {
	cache *cache.Cache
}

func NewRedisCache(redis *redis.Redis) (*RedisCache, error) {
	client := redis.Client()

	return &RedisCache{
		cache: cache.New(&cache.Options{
			Redis:      client,
			LocalCache: cache.NewTinyLFU(cacheSize, 24*time.Hour),
		}),
	}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string, value interface{}) error {

	if !r.cache.Exists(ctx, key) {
		return fmt.Errorf("cache: key %s does not exist", key)
	}

	err := r.cache.Get(ctx, key, &value)

	if errors.Is(err, cache.ErrCacheMiss) {
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.cache.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: value,
		TTL:   ttl,
	})
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.cache.Delete(ctx, key)
}
