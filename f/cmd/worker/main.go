package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "time"

    "studyguide.parallel/pkg/common"
    ftqqueue "go-blur-ftq/pkg/queue"
    "studyguide.parallel/pkg/blur"
)

func main() {
    var (
        redisAddr  = flag.String("redis", "redis:6379", "Redis address")
        kernelSize = flag.Int("kernel", 15, "Gaussian kernel size")
        timeout    = flag.Duration("timeout", 5*time.Second, "Stream read block timeout")
        visTimeout = flag.Duration("visibility", 30*time.Second, "Visibility timeout for retries")
    )
    flag.Parse()

    hostname, _ := os.Hostname()
    consumer := fmt.Sprintf("worker-%s", hostname)
    
    rs, err := ftqqueue.NewRedisStreams(*redisAddr)
    if err != nil { log.Fatalf("redis: %v", err) }
    defer rs.Close()
    if err := rs.EnsureGroups(); err != nil { log.Printf("ensure groups: %v", err) }

    log.Printf("Worker %s ready - waiting for jobs on fixed streams...", consumer)

    kernel := blur.GenerateGaussianKernel(*kernelSize)

    for {
        // Claim stale jobs periodically
        if _, err := rs.ClaimStaleJobs(consumer, *visTimeout, 50); err != nil {
            log.Printf("claim stale: %v", err)
        }

        id, job, err := rs.ReadJob(consumer, *timeout)
        if err != nil { log.Printf("read job: %v", err); continue }
        if job == nil { continue }

        if job.Type != "tile" || job.ImageTile == nil { log.Printf("invalid job"); _ = rs.AckJob(id); continue }

        tile := job.ImageTile
        start := time.Now()
        blurred := blur.ApplyBlurToTile(tile.Data, kernel)
        center := blur.ExtractCenter(blurred, tile.Padding, tile.Width, tile.Height)
        processed := &common.ProcessedImageTile{ImageID: tile.ImageID, TileID: tile.TileID, X: tile.X, Y: tile.Y, Width: tile.Width, Height: tile.Height, Data: center}
        res := &common.ResultMessage{ProcessedTile: processed, WorkerID: consumer, ProcessTime: time.Since(start).Seconds()}
        if _, err := rs.AddResult(res); err != nil { log.Printf("push result: %v", err); continue }
        if err := rs.AckJob(id); err != nil { log.Printf("ack job: %v", err) }
    }
}

