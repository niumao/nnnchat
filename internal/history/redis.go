package history

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Store backed by a capped Redis list per room.
type RedisStore struct {
	client *redis.Client
	cap    int64
}

// NewRedisStore returns a RedisStore that retains at most cap messages per room.
func NewRedisStore(client *redis.Client, cap int) *RedisStore {
	return &RedisStore{client: client, cap: int64(cap)}
}

// NewRedisStoreFromAddr dials addr and returns a RedisStore that retains at
// most cap messages per room, or an error if Redis is not reachable.
func NewRedisStoreFromAddr(addr string, cap int) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("history: redis not reachable at %s: %w", addr, err)
	}
	return NewRedisStore(rdb, cap), nil
}

func historyKey(room string) string {
	return "nnnchat:history:" + room
}

// Append pushes msg onto the room's history list and trims it back down to cap.
func (s *RedisStore) Append(ctx context.Context, room string, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	key := historyKey(room)
	if err := s.client.RPush(ctx, key, data).Err(); err != nil {
		return err
	}
	return s.client.LTrim(ctx, key, -s.cap, -1).Err()
}

// Recent returns up to the last n messages for room, oldest first.
func (s *RedisStore) Recent(ctx context.Context, room string, n int) ([]Message, error) {
	vals, err := s.client.LRange(ctx, historyKey(room), -int64(n), -1).Result()
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(vals))
	for _, v := range vals {
		var m Message
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
