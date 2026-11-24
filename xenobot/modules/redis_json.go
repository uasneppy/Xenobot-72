package modules

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"
)

// redisGetJSON reads key from client (bounded by raidRedisTimeout) and decodes it as JSON into a T.
// found is false for a missing key; a Redis error and a value that does not parse are errors. The
// caller checks that client is not nil.
func redisGetJSON[T any](ctx context.Context, client *redis.Client, key string) (T, bool, error) {
	var zero T
	getCtx, cancel := context.WithTimeout(ctx, raidRedisTimeout)
	defer cancel()
	raw, err := client.Get(getCtx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, false, err
	}
	return out, true, nil
}
