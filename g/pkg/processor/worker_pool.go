package processor

import (
    "context"
    "fmt"
    "log"
    "sync"
    "sync/atomic"
    "time"

    "go-blur-mt/pkg/queue"
    "studyguide.parallel/pkg/common"
    "studyguide.parallel/pkg/blur"
)


type WorkerPool struct {
    redisClient   *queue.RedisClient
    numWorkers    int
    kernelSize    int
    kernel        [][]float64
    workerID      string
    tilesProcessed atomic.Int64
    ctx           context.Context
    cancel        context.CancelFunc
}

func NewWorkerPool(redisClient *queue.RedisClient, numWorkers, kernelSize int, workerID string) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &WorkerPool{
        redisClient: redisClient,
        numWorkers:  numWorkers,
        kernelSize:  kernelSize,
        kernel:      blur.GenerateGaussianKernel(kernelSize),
        workerID:    workerID,
        ctx:         ctx,
        cancel:      cancel,
    }
}

func (wp *WorkerPool) Start() {
    var wg sync.WaitGroup
    
    for i := 0; i < wp.numWorkers; i++ {
        wg.Add(1)
        go wp.worker(i, &wg)
    }
    
    wg.Add(1)
    go wp.retryMonitor(&wg)
    
    log.Printf("WorkerPool: Started %d workers", wp.numWorkers)
    wg.Wait()
}

func (wp *WorkerPool) Stop() {
    log.Println("WorkerPool: Shutting down...")
    wp.cancel()
}

func (wp *WorkerPool) worker(id int, wg *sync.WaitGroup) {
    defer wg.Done()
    
    consumer := fmt.Sprintf("%s-worker-%d", wp.workerID, id)
    log.Printf("Worker %d started as consumer %s", id, consumer)
    
    for {
        select {
        case <-wp.ctx.Done():
            log.Printf("Worker %d shutting down", id)
            return
        default:
            msgID, job, err := wp.redisClient.ReadJob(consumer, 5*time.Second)
            if err != nil {
                if err.Error() != "redis: nil" {
                    log.Printf("Worker %d read error: %v", id, err)
                }
                continue
            }
            
            if job == nil {
                continue
            }
            
            if job.Type != "tile" || job.ImageTile == nil {
                log.Printf("Worker %d: invalid job type", id)
                _ = wp.redisClient.AckJob(msgID)
                continue
            }
            
            if err := wp.processTile(job.ImageTile, msgID); err != nil {
                log.Printf("Worker %d failed to process tile: %v", id, err)
                // Don't ACK the message - let it be reclaimed after visibility timeout
            } else {
                _ = wp.redisClient.AckJob(msgID)
                wp.tilesProcessed.Add(1)
                
                if count := wp.tilesProcessed.Load(); count%100 == 0 {
                    log.Printf("WorkerPool: Processed %d tiles total", count)
                }
            }
        }
    }
}

func (wp *WorkerPool) processTile(tile *common.ImageTile, msgID string) error {
    startTime := time.Now()
    
    blurred := blur.ApplyBlurToTile(tile.Data, wp.kernel)
    
    center := blur.ExtractCenter(blurred, tile.Padding, tile.Width, tile.Height)
    
    processed := &common.ProcessedImageTile{
        ImageID: tile.ImageID,
        TileID:  tile.TileID,
        X:       tile.X,
        Y:       tile.Y,
        Width:   tile.Width,
        Height:  tile.Height,
        Data:    center,
    }
    
    result := &common.ResultMessage{
        ProcessedTile: processed,
        WorkerID:      wp.workerID,
        ProcessTime:   time.Since(startTime).Seconds(),
    }
    
    if _, err := wp.redisClient.AddResult(result); err != nil {
        return fmt.Errorf("failed to add result: %w", err)
    }
    
    return nil
}


func (wp *WorkerPool) retryMonitor(wg *sync.WaitGroup) {
    defer wg.Done()
    
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    consumer := fmt.Sprintf("%s-retry-monitor", wp.workerID)
    
    for {
        select {
        case <-wp.ctx.Done():
            return
        case <-ticker.C:
            claimedIDs, err := wp.redisClient.ClaimStaleJobs(consumer, 30*time.Second, 50)
            if err != nil {
                log.Printf("Failed to claim stale jobs: %v", err)
                continue
            }
            
            if len(claimedIDs) > 0 {
                log.Printf("Claimed %d stale jobs for retry", len(claimedIDs))
            }
        }
    }
}