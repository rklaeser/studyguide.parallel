package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "studyguide.parallel/pkg/common"
)

type RedisStreams struct {
    client *redis.Client
    ctx    context.Context
}

func NewRedisStreams(addr string) (*RedisStreams, error) {
    client := redis.NewClient(&redis.Options{Addr: addr})
    ctx := context.Background()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, err
    }
    rs := &RedisStreams{client: client, ctx: ctx}
    return rs, nil
}

func (r *RedisStreams) Close() error { return r.client.Close() }

func (r *RedisStreams) jobsStream() string    { return "ftq:jobs" }
func (r *RedisStreams) resultsStream() string { return "ftq:results" }
func (r *RedisStreams) dlqJobsStream() string { return "ftq:dlq:jobs" }

func (r *RedisStreams) imageInfoKey(imageID int) string   { return fmt.Sprintf("image:%d:info", imageID) }
func (r *RedisStreams) timingKey() string                 { return "timing" }
func (r *RedisStreams) receivedSetKey(imageID int) string { return fmt.Sprintf("image:%d:received", imageID) }

// Init consumer groups (idempotent)
func (r *RedisStreams) EnsureGroups() error {
    // Create consumer groups with MKSTREAM to create empty streams if needed
    // Using "$" means only new messages will be consumed (not existing ones)
    _ = r.client.XGroupCreateMkStream(r.ctx, r.jobsStream(), "workers", "$").Err()
    _ = r.client.XGroupCreateMkStream(r.ctx, r.resultsStream(), "assemblers", "$").Err()
    return nil
}

// Producer APIs
func (r *RedisStreams) AddJob(job *common.JobMessage) (string, error) {
    b, err := json.Marshal(job)
    if err != nil { return "", err }
    id := r.client.XAdd(r.ctx, &redis.XAddArgs{Stream: r.jobsStream(), Values: map[string]any{"data": b}}).Val()
    return id, nil
}

func (r *RedisStreams) AddResult(res *common.ResultMessage) (string, error) {
    b, err := json.Marshal(res)
    if err != nil { return "", err }
    id := r.client.XAdd(r.ctx, &redis.XAddArgs{Stream: r.resultsStream(), Values: map[string]any{"data": b}}).Val()
    return id, nil
}

// Consumer APIs
func (r *RedisStreams) ReadJob(consumer string, block time.Duration) (string, *common.JobMessage, error) {
    res := r.client.XReadGroup(r.ctx, &redis.XReadGroupArgs{
        Group:    "workers",
        Consumer: consumer,
        Streams:  []string{r.jobsStream(), ">"},
        Count:    1,
        Block:    block,
    }).Val()
    if len(res) == 0 || len(res[0].Messages) == 0 { return "", nil, nil }
    msg := res[0].Messages[0]
    raw := bytesFromAny(msg.Values["data"]) 
    var jm common.JobMessage
    if err := json.Unmarshal(raw, &jm); err != nil { return "", nil, err }
    return msg.ID, &jm, nil
}

func (r *RedisStreams) AckJob(id string) error {
    return r.client.XAck(r.ctx, r.jobsStream(), "workers", id).Err()
}

func (r *RedisStreams) ReadResult(consumer string, block time.Duration) (string, *common.ResultMessage, error) {
    res := r.client.XReadGroup(r.ctx, &redis.XReadGroupArgs{
        Group:    "assemblers",
        Consumer: consumer,
        Streams:  []string{r.resultsStream(), ">"},
        Count:    1,
        Block:    block,
    }).Val()
    if len(res) == 0 || len(res[0].Messages) == 0 { return "", nil, nil }
    msg := res[0].Messages[0]
    raw := bytesFromAny(msg.Values["data"]) 
    var rm common.ResultMessage
    if err := json.Unmarshal(raw, &rm); err != nil { return "", nil, err }
    return msg.ID, &rm, nil
}

func (r *RedisStreams) AckResult(id string) error {
    return r.client.XAck(r.ctx, r.resultsStream(), "assemblers", id).Err()
}

// Pending and claim for retries
func (r *RedisStreams) ClaimStaleJobs(consumer string, minIdle time.Duration, count int) ([]string, error) {
    pend := r.client.XPendingExt(r.ctx, &redis.XPendingExtArgs{
        Stream: r.jobsStream(), Group: "workers", Idle: minIdle, Count: int64(count), Start: "-", End: "+",
    }).Val()
    ids := make([]string, 0, len(pend))
    for _, p := range pend {
        ids = append(ids, p.ID)
    }
    if len(ids) == 0 { return ids, nil }
    claimed := r.client.XClaim(r.ctx, &redis.XClaimArgs{
        Stream:   r.jobsStream(), Group: "workers", Consumer: consumer, MinIdle: minIdle, Messages: ids,
    }).Val()
    out := make([]string, 0, len(claimed))
    for _, c := range claimed { out = append(out, c.ID) }
    return out, nil
}

// Metadata APIs
func (r *RedisStreams) StoreImageInfo(info *common.ImageInfo) error {
    key := r.imageInfoKey(info.ID)
    b, err := json.Marshal(info)
    if err != nil { return err }
    return r.client.Set(r.ctx, key, b, 24*time.Hour).Err()
}

func (r *RedisStreams) GetImageInfo(imageID int) (*common.ImageInfo, error) {
    key := r.imageInfoKey(imageID)
    data, err := r.client.Get(r.ctx, key).Result()
    if err != nil { return nil, err }
    var info common.ImageInfo
    if err := json.Unmarshal([]byte(data), &info); err != nil { return nil, err }
    return &info, nil
}

func (r *RedisStreams) StoreTiming(t *common.TimingData) error {
    b, err := json.Marshal(t)
    if err != nil { return err }
    return r.client.Set(r.ctx, r.timingKey(), b, 24*time.Hour).Err()
}

func (r *RedisStreams) GetTiming() (*common.TimingData, error) {
    data, err := r.client.Get(r.ctx, r.timingKey()).Result()
    if err != nil { return nil, err }
    var t common.TimingData
    if err := json.Unmarshal([]byte(data), &t); err != nil { return nil, err }
    return &t, nil
}

func (r *RedisStreams) MarkTileReceived(imageID int, tileID int) (int64, error) {
    return r.client.SAdd(r.ctx, r.receivedSetKey(imageID), tileID).Result()
}

func (r *RedisStreams) GetReceivedCount(imageID int) (int64, error) {
    return r.client.SCard(r.ctx, r.receivedSetKey(imageID)).Result()
}

// helper: handle Redis returning either string or []byte
func bytesFromAny(v any) []byte {
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

