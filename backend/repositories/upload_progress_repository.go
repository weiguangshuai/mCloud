package repositories

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisUploadProgressRepository struct {
	redis *redis.Client
}

func NewRedisUploadProgressRepository(redisClient *redis.Client) *RedisUploadProgressRepository {
	return &RedisUploadProgressRepository{redis: redisClient}
}

func uploadChunkKey(uploadID string) string {
	return fmt.Sprintf("upload:%s:chunks", uploadID)
}

func (r *RedisUploadProgressRepository) IsChunkUploaded(ctx context.Context, uploadID string, chunkIndex int) (bool, error) {
	return r.redis.SIsMember(ctx, uploadChunkKey(uploadID), chunkIndex).Result()
}

func (r *RedisUploadProgressRepository) AddChunk(ctx context.Context, uploadID string, chunkIndex int, expireSeconds int) error {
	key := uploadChunkKey(uploadID)
	if err := r.redis.SAdd(ctx, key, chunkIndex).Err(); err != nil {
		return err
	}
	if expireSeconds > 0 {
		return r.redis.Expire(ctx, key, timeDurationSeconds(expireSeconds)).Err()
	}
	return nil
}

func (r *RedisUploadProgressRepository) UploadedCount(ctx context.Context, uploadID string) (int64, error) {
	return r.redis.SCard(ctx, uploadChunkKey(uploadID)).Result()
}

func (r *RedisUploadProgressRepository) UploadedChunks(ctx context.Context, uploadID string) ([]int, error) {
	members, err := r.redis.SMembers(ctx, uploadChunkKey(uploadID)).Result()
	if err != nil {
		return nil, err
	}
	result := make([]int, 0, len(members))
	for _, member := range members {
		idx, convErr := strconv.Atoi(member)
		if convErr != nil {
			continue
		}
		result = append(result, idx)
	}
	return result, nil
}

func (r *RedisUploadProgressRepository) Clear(ctx context.Context, uploadID string) error {
	return r.redis.Del(ctx, uploadChunkKey(uploadID)).Err()
}

func timeDurationSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
