package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "studyguide.parallel/pkg/common"
)

type RedisClient struct {
    client *redis.Client
    ctx    context.Context
}

func NewRedisClient(addr string) (*RedisClient, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         addr,
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
    })
    
    ctx := context.Background()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis ping failed: %w", err)
    }
    
    return &RedisClient{
        client: client,
        ctx:    ctx,
    }, nil
}

func (r *RedisClient) Close() error {
    return r.client.Close()
}

func (r *RedisClient) jobsStream() string {
    return "mt:jobs"
}

func (r *RedisClient) resultsStream() string {
    return "mt:results"
}

func (r *RedisClient) imageInfoKey(imageID int) string {
    return fmt.Sprintf("mt:image:%d:info", imageID)
}

func (r *RedisClient) imageStatusKey(imageID int) string {
    return fmt.Sprintf("mt:image:%d:status", imageID)
}


func (r *RedisClient) EnsureGroups() error {
    _ = r.client.XGroupCreateMkStream(r.ctx, r.jobsStream(), "workers", "$").Err()
    _ = r.client.XGroupCreateMkStream(r.ctx, r.resultsStream(), "assemblers", "$").Err()
    return nil
}

func (r *RedisClient) AddJob(job *common.JobMessage) (string, error) {
    b, err := json.Marshal(job)
    if err != nil {
        return "", err
    }
    
    result := r.client.XAdd(r.ctx, &redis.XAddArgs{
        Stream: r.jobsStream(),
        Values: map[string]interface{}{"data": b},
    })
    
    return result.Val(), result.Err()
}

func (r *RedisClient) AddResult(res *common.ResultMessage) (string, error) {
    b, err := json.Marshal(res)
    if err != nil {
        return "", err
    }
    
    result := r.client.XAdd(r.ctx, &redis.XAddArgs{
        Stream: r.resultsStream(),
        Values: map[string]interface{}{"data": b},
    })
    
    return result.Val(), result.Err()
}

func (r *RedisClient) ReadJob(consumer string, block time.Duration) (string, *common.JobMessage, error) {
    result, err := r.client.XReadGroup(r.ctx, &redis.XReadGroupArgs{
        Group:    "workers",
        Consumer: consumer,
        Streams:  []string{r.jobsStream(), ">"},
        Count:    1,
        Block:    block,
    }).Result()
    
    if err != nil || len(result) == 0 || len(result[0].Messages) == 0 {
        return "", nil, err
    }
    
    msg := result[0].Messages[0]
    data := r.bytesFromInterface(msg.Values["data"])
    
    var job common.JobMessage
    if err := json.Unmarshal(data, &job); err != nil {
        return "", nil, err
    }
    
    return msg.ID, &job, nil
}

func (r *RedisClient) AckJob(id string) error {
    return r.client.XAck(r.ctx, r.jobsStream(), "workers", id).Err()
}

func (r *RedisClient) ReadResult(consumer string, block time.Duration) (string, *common.ResultMessage, error) {
    result, err := r.client.XReadGroup(r.ctx, &redis.XReadGroupArgs{
        Group:    "assemblers",
        Consumer: consumer,
        Streams:  []string{r.resultsStream(), ">"},
        Count:    1,
        Block:    block,
    }).Result()
    
    if err != nil || len(result) == 0 || len(result[0].Messages) == 0 {
        return "", nil, err
    }
    
    msg := result[0].Messages[0]
    data := r.bytesFromInterface(msg.Values["data"])
    
    var res common.ResultMessage
    if err := json.Unmarshal(data, &res); err != nil {
        return "", nil, err
    }
    
    return msg.ID, &res, nil
}

func (r *RedisClient) AckResult(id string) error {
    return r.client.XAck(r.ctx, r.resultsStream(), "assemblers", id).Err()
}

func (r *RedisClient) StoreImageInfo(info *common.ImageInfo) error {
    b, err := json.Marshal(info)
    if err != nil {
        return err
    }
    return r.client.Set(r.ctx, r.imageInfoKey(info.ID), b, 24*time.Hour).Err()
}

func (r *RedisClient) GetImageInfo(imageID int) (*common.ImageInfo, error) {
    data, err := r.client.Get(r.ctx, r.imageInfoKey(imageID)).Result()
    if err != nil {
        return nil, err
    }
    
    var info common.ImageInfo
    if err := json.Unmarshal([]byte(data), &info); err != nil {
        return nil, err
    }
    
    return &info, nil
}

func (r *RedisClient) MarkImageCompleted(imageID int) error {
    return r.client.Set(r.ctx, r.imageStatusKey(imageID), "completed", 0).Err()
}

func (r *RedisClient) IsImageCompleted(imageID int) (bool, error) {
    result, err := r.client.Get(r.ctx, r.imageStatusKey(imageID)).Result()
    if err == redis.Nil {
        return false, nil
    }
    if err != nil {
        return false, err
    }
    return result == "completed", nil
}


func (r *RedisClient) ClaimStaleJobs(consumer string, minIdle time.Duration, count int) ([]string, error) {
    pending, err := r.client.XPendingExt(r.ctx, &redis.XPendingExtArgs{
        Stream:  r.jobsStream(),
        Group:   "workers",
        Idle:    minIdle,
        Count:   int64(count),
        Start:   "-",
        End:     "+",
    }).Result()
    
    if err != nil || len(pending) == 0 {
        return nil, err
    }
    
    ids := make([]string, 0, len(pending))
    for _, p := range pending {
        ids = append(ids, p.ID)
    }
    
    claimed, err := r.client.XClaim(r.ctx, &redis.XClaimArgs{
        Stream:   r.jobsStream(),
        Group:    "workers",
        Consumer: consumer,
        MinIdle:  minIdle,
        Messages: ids,
    }).Result()
    
    if err != nil {
        return nil, err
    }
    
    claimedIDs := make([]string, 0, len(claimed))
    for _, c := range claimed {
        claimedIDs = append(claimedIDs, c.ID)
    }
    
    return claimedIDs, nil
}

func (r *RedisClient) bytesFromInterface(v interface{}) []byte {
    switch t := v.(type) {
    case string:
        return []byte(t)
    case []byte:
        return t
    default:
        b, _ := json.Marshal(t)
        return b
    }
}