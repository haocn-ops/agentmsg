package repository

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"agentmsg/internal/model"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(redisURL string) (*RedisClient, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Addr:     u.Host,
		PoolSize: 100,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisClient{client: client}, nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) Client() *redis.Client {
	return r.client
}

func (r *RedisClient) Publish(ctx context.Context, channel string, message interface{}) error {
	return r.client.Publish(ctx, channel, message).Err()
}

func (r *RedisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return r.client.Subscribe(ctx, channels...)
}

func (r *RedisClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.LPush(ctx, key, values).Err()
}

func (r *RedisClient) RPop(ctx context.Context, key string) (string, error) {
	return r.client.RPop(ctx, key).Result()
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	return r.client.HSet(ctx, key, values...).Err()
}

func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

func (r *RedisClient) Expire(ctx context.Context, key string, seconds int) error {
	return r.client.Expire(ctx, key, 0).Err()
}

func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return r.client.ZAdd(ctx, key, members...).Err()
}

func (r *RedisClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

func (r *RedisClient) CreateSubscription(ctx context.Context, sub *model.Subscription) error {
	key := "subscriptions:" + sub.AgentID.String()
	data, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	return r.client.HSet(ctx, key, sub.ID.String(), data).Err()
}

func (r *RedisClient) ListSubscriptions(ctx context.Context, agentID uuid.UUID) ([]model.Subscription, error) {
	key := "subscriptions:" + agentID.String()
	result, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	subs := make([]model.Subscription, 0, len(result))
	for _, data := range result {
		var sub model.Subscription
		if err := json.Unmarshal([]byte(data), &sub); err != nil {
			continue
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (r *RedisClient) DeleteSubscription(ctx context.Context, agentID, subID uuid.UUID) error {
	key := "subscriptions:" + agentID.String()
	return r.client.HDel(ctx, key, subID.String()).Err()
}

func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	return result > 0, err
}

func (r *RedisClient) SetWithExpiry(ctx context.Context, key, value string, seconds int) error {
	return r.client.Set(ctx, key, value, time.Duration(seconds)*time.Second).Err()
}
