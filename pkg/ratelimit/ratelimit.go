package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
}

func New(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	pipe := rl.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := incr.Val()
	return count <= int64(limit), nil
}

func (rl *RateLimiter) TokenBucket(ctx context.Context, key string, capacity int, refillRate float64) (bool, error) {
	now := time.Now().Unix()
	
	val, err := rl.client.HGetAll(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}

	tokens := float64(capacity)
	lastUpdate := now

	if len(val) > 0 && val["tokens"] != "" {
		var lastUpdateInt int64
		fmt.Sscanf(val["last_update"], "%d", &lastUpdateInt)
		lastUpdate = lastUpdateInt
		elapsed := float64(now - lastUpdate)
		fmt.Sscanf(val["tokens"], "%f", &tokens)
		tokens = tokens + elapsed*refillRate
		if tokens > float64(capacity) {
			tokens = float64(capacity)
		}
	}

	if tokens < 1 {
		return false, nil
	}

	tokens--
	rl.client.HSet(ctx, key, "tokens", tokens, "last_update", now)
	rl.client.Expire(ctx, key, time.Minute*10)
	return true, nil
}

