package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"goods-service/internal/models"
	"time"
)

const (
	totalCountKey   = "goods:total_count"
	removedCountKey = "goods:removed_count"
	countTTL        = time.Minute
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}

func (r *RedisRepository) SetGood(ctx context.Context, good *models.Good) error {
	data, err := json.Marshal(good)
	if err != nil {
		return err
	}

	key := r.getGoodKey(good.ID, good.ProjectID)
	return r.client.Set(ctx, key, data, time.Minute).Err()
}

func (r *RedisRepository) GetGood(ctx context.Context, id, projectID int) (*models.Good, error) {
	key := r.getGoodKey(id, projectID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var good models.Good
	if err := json.Unmarshal(data, &good); err != nil {
		return nil, err
	}

	return &good, nil
}

func (r *RedisRepository) InvalidateGood(ctx context.Context, id, projectID int) error {
	key := r.getGoodKey(id, projectID)
	return r.client.Del(ctx, key).Err()
}

func (r *RedisRepository) GetTotalCount(ctx context.Context) (int, error) {
	val, err := r.client.Get(ctx, totalCountKey).Int()
	if errors.Is(err, redis.Nil) {
		return 0, errors.New("total count not found in cache")
	}
	return val, err
}

func (r *RedisRepository) SetTotalCount(ctx context.Context, count int) error {
	return r.client.Set(ctx, totalCountKey, count, countTTL).Err()
}

func (r *RedisRepository) GetRemovedCount(ctx context.Context) (int, error) {
	val, err := r.client.Get(ctx, removedCountKey).Int()
	if errors.Is(err, redis.Nil) {
		return 0, errors.New("removed count not found in cache")
	}
	return val, err
}

func (r *RedisRepository) SetRemovedCount(ctx context.Context, count int) error {
	return r.client.Set(ctx, removedCountKey, count, countTTL).Err()
}

// InvalidateCounts Инвалидирует кэш счетчиков
func (r *RedisRepository) InvalidateCounts(ctx context.Context) error {
	pipe := r.client.Pipeline()
	pipe.Del(ctx, totalCountKey)
	pipe.Del(ctx, removedCountKey)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisRepository) getGoodKey(id, projectID int) string {
	return fmt.Sprintf("good:%d:%d", projectID, id)
}
