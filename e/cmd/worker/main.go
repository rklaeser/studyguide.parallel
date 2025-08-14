package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"studyguide.parallel/pkg/common"
	"studyguide.parallel/e/pkg/queue"
)

func main() {
	var (
		redisAddr  = flag.String("redis", "redis:6379", "Redis server address")
		kernelSize = flag.Int("kernel", 15, "Gaussian kernel size")
		workerID   = flag.String("id", "", "Worker ID (defaults to hostname)")
		timeout    = flag.Duration("timeout", 30*time.Second, "Job poll timeout")
	)
	flag.Parse()

	// Set worker ID
	if *workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			*workerID = fmt.Sprintf("worker-%d", time.Now().Unix())
		} else {
			*workerID = hostname
		}
	}

	log.Printf("Worker %s starting...", *workerID)
	log.Printf("Redis address: %s", *redisAddr)
	log.Printf("Kernel size: %d", *kernelSize)

	// Connect to Redis
	redisQueue, err := queue.NewRedisQueue(*redisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	// Generate Gaussian kernel once
	kernel := common.GenerateGaussianKernel(*kernelSize)
	log.Printf("Generated Gaussian kernel of size %d", *kernelSize)

	tilesProcessed := 0

	// Main worker loop
	for {
		// Pop job from queue
		job, err := redisQueue.PopJob(*timeout)
		if err != nil {
			log.Printf("Error popping job: %v", err)
			continue
		}

		if job == nil {
			log.Printf("No job available, waiting...")
			continue
		}

		// Check for completion signal
		if job.Type == "complete" {
			log.Printf("Received completion signal. Processed %d tiles total.", tilesProcessed)
			break
		}

		if job.Type != "tile" || job.ImageTile == nil {
			log.Printf("Invalid job type or missing tile data")
			continue
		}

		tile := job.ImageTile
		startTime := time.Now()

		// Apply blur to tile
		blurredData := common.ApplyBlurToTile(tile.Data, kernel)

		// Extract center portion (remove padding)
		centerData := common.ExtractCenter(blurredData, tile.Padding, tile.Width, tile.Height)

		// Create processed tile
		processedTile := &common.ProcessedImageTile{
			ImageID: tile.ImageID,
			TileID:  tile.TileID,
			X:       tile.X,
			Y:       tile.Y,
			Width:   tile.Width,
			Height:  tile.Height,
			Data:    centerData,
		}

		// Push result back to queue
		result := &common.ResultMessage{
			ProcessedTile: processedTile,
			WorkerID:      *workerID,
			ProcessTime:   time.Since(startTime).Seconds(),
		}

		if err := redisQueue.PushResult(result); err != nil {
			log.Printf("Failed to push result: %v", err)
			continue
		}

		// Update progress
		_, err = redisQueue.IncrementProgress(tile.ImageID)
		if err != nil {
			log.Printf("Failed to update progress: %v", err)
		}

		tilesProcessed++
		
		if tilesProcessed%10 == 0 {
			log.Printf("Worker %s: Processed %d tiles so far...", *workerID, tilesProcessed)
		}
	}

	log.Printf("Worker %s shutting down. Processed %d tiles total.", *workerID, tilesProcessed)
}