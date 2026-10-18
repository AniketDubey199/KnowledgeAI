package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client.
type Client struct {
	RDB *redis.Client
}

// NewClient initializes a Redis connection client.
func NewClient(ctx context.Context, addr, password string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("failed to ping redis at %s: %w", addr, err)
	}

	return &Client{RDB: rdb}, nil
}

// Ping verifies that Redis is responsive.
func (c *Client) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.RDB.Ping(pingCtx).Err()
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c.RDB != nil {
		return c.RDB.Close()
	}
	return nil
}
