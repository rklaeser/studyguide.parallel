package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"studyguide.parallel/pkg/common"
)

const (
	JobQueueKey    = "image:job:queue"
	ResultQueueKey = "image:result:queue"
	ImageInfoKey   = "image:info:%d"
	ProgressKey    = "image:progress:%d"
	TimingDataKey  = "timing:data"
)

type RedisQueue struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisQueue(addr string) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	ctx := context.Background()
	
	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisQueue{
		client: client,
		ctx:    ctx,
	}, nil
}

// PushJob adds a job to the queue
func (q *RedisQueue) PushJob(job *common.JobMessage) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}
	
	return q.client.LPush(q.ctx, JobQueueKey, data).Err()
}

// PopJob retrieves and removes a job from the queue (blocking)
func (q *RedisQueue) PopJob(timeout time.Duration) (*common.JobMessage, error) {
	result, err := q.client.BRPop(q.ctx, timeout, JobQueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Timeout, no job available
		}
		return nil, fmt.Errorf("failed to pop job: %w", err)
	}
	
	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected result format")
	}
	
	var job common.JobMessage
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}
	
	return &job, nil
}

// PushResult adds a processed result to the result queue
func (q *RedisQueue) PushResult(result *common.ResultMessage) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	
	return q.client.LPush(q.ctx, ResultQueueKey, data).Err()
}

// PopResult retrieves a processed result (blocking)
func (q *RedisQueue) PopResult(timeout time.Duration) (*common.ResultMessage, error) {
	result, err := q.client.BRPop(q.ctx, timeout, ResultQueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Timeout
		}
		return nil, fmt.Errorf("failed to pop result: %w", err)
	}
	
	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected result format")
	}
	
	var msg common.ResultMessage
	if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}
	
	return &msg, nil
}

// StoreImageInfo stores metadata about an image
func (q *RedisQueue) StoreImageInfo(info *common.ImageInfo) error {
	key := fmt.Sprintf(ImageInfoKey, info.ID)
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal image info: %w", err)
	}
	
	return q.client.Set(q.ctx, key, data, 24*time.Hour).Err()
}

// GetImageInfo retrieves metadata about an image
func (q *RedisQueue) GetImageInfo(imageID int) (*common.ImageInfo, error) {
	key := fmt.Sprintf(ImageInfoKey, imageID)
	data, err := q.client.Get(q.ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get image info: %w", err)
	}
	
	var info common.ImageInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal image info: %w", err)
	}
	
	return &info, nil
}

// IncrementProgress increments the progress counter for an image
func (q *RedisQueue) IncrementProgress(imageID int) (int64, error) {
	key := fmt.Sprintf(ProgressKey, imageID)
	return q.client.Incr(q.ctx, key).Result()
}

// GetProgress gets the current progress for an image
func (q *RedisQueue) GetProgress(imageID int) (int64, error) {
	key := fmt.Sprintf(ProgressKey, imageID)
	result, err := q.client.Get(q.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	
	var progress int64
	fmt.Sscanf(result, "%d", &progress)
	return progress, nil
}

// StoreTiming stores timing data in Redis
func (q *RedisQueue) StoreTiming(timing *common.TimingData) error {
	data, err := json.Marshal(timing)
	if err != nil {
		return fmt.Errorf("failed to marshal timing data: %w", err)
	}
	
	return q.client.Set(q.ctx, TimingDataKey, data, 24*time.Hour).Err()
}

// GetTiming retrieves timing data from Redis
func (q *RedisQueue) GetTiming() (*common.TimingData, error) {
	data, err := q.client.Get(q.ctx, TimingDataKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No timing data exists yet
		}
		return nil, fmt.Errorf("failed to get timing data: %w", err)
	}
	
	var timing common.TimingData
	if err := json.Unmarshal([]byte(data), &timing); err != nil {
		return nil, fmt.Errorf("failed to unmarshal timing data: %w", err)
	}
	
	return &timing, nil
}

// UpdateImageEndTime updates the end time for a specific image
func (q *RedisQueue) UpdateImageEndTime(imageID int, endTime time.Time) error {
	timing, err := q.GetTiming()
	if err != nil {
		return err
	}
	if timing == nil {
		return fmt.Errorf("no timing data found")
	}
	
	if timing.ImageEndTimes == nil {
		timing.ImageEndTimes = make(map[int]*time.Time)
	}
	timing.ImageEndTimes[imageID] = &endTime
	
	return q.StoreTiming(timing)
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}