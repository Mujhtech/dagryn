// Package redis provides a Redis client wrapper for the Dagryn application.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection configuration.
type Config struct {
	// Host is the Redis server hostname.
	Host string `toml:"host"`
	// Port is the Redis server port.
	Port int `toml:"port"`
	// Password is the Redis password (optional).
	Password string `toml:"password"`
	// DB is the Redis database number.
	DB int `toml:"db"`
	// PoolSize is the maximum number of socket connections.
	PoolSize int `toml:"pool_size"`
	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int `toml:"min_idle_conns"`
	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration `toml:"dial_timeout"`
	// ReadTimeout is the timeout for socket reads.
	ReadTimeout time.Duration `toml:"read_timeout"`
	// WriteTimeout is the timeout for socket writes.
	WriteTimeout time.Duration `toml:"write_timeout"`
}

// DefaultConfig returns sensible defaults for Redis configuration.
func DefaultConfig() Config {
	return Config{
		Host:         "localhost",
		Port:         6379,
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// Address returns the Redis server address (host:port).
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Redis is a wrapper around the Redis client that implements asynq.RedisConnOpt.
type Redis struct {
	client *redis.Client
	config Config
}

// New creates a new Redis client wrapper.
func New(cfg Config) *Redis {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	return &Redis{
		client: client,
		config: cfg,
	}
}

// Connect verifies the connection to Redis.
func (r *Redis) Connect(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

// Client returns the underlying Redis client.
func (r *Redis) Client() *redis.Client {
	return r.client
}

// MakeRedisClient implements asynq.RedisConnOpt interface.
// It returns the underlying Redis universal client.
func (r *Redis) MakeRedisClient() interface{} {
	return r.client
}

// Ensure Redis implements asynq.RedisConnOpt interface.
var _ asynq.RedisConnOpt = (*Redis)(nil)
