package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-blur/pkg/common"
	"go-blur/pkg/queue"
	"studyguide.parallel/pkg/stats"
)

type ImageAssembler struct {
	imageInfo     *common.ImageInfo
	tiles         map[int]*common.ProcessedImageTile
	tilesReceived int
	mutex         sync.Mutex
	outputImage   *image.RGBA
}

func main() {
	var (
		redisAddr   = flag.String("redis", "redis:6379", "Redis server address")
		timeout     = flag.Duration("timeout", 30*time.Second, "Result poll timeout")
		maxImages   = flag.Int("max-images", 100, "Maximum number of images to track")
	)
	flag.Parse()

	log.Printf("Assembler starting...")
	log.Printf("Redis address: %s", *redisAddr)

	// Connect to Redis
	redisQueue, err := queue.NewRedisQueue(*redisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	// Track active image assemblies
	assemblers := make(map[int]*ImageAssembler)
	completedImages := 0

	// Main assembler loop
	for {
		// Pop result from queue
		result, err := redisQueue.PopResult(*timeout)
		if err != nil {
			log.Printf("Error popping result: %v", err)
			continue
		}

		if result == nil {
			// Check if all known images are complete
			allComplete := true
			for imageID, assembler := range assemblers {
				progress, _ := redisQueue.GetProgress(imageID)
				log.Printf("Image %d: %d/%d tiles", imageID+1, progress, assembler.imageInfo.ExpectedTiles)
				if int(progress) < assembler.imageInfo.ExpectedTiles {
					allComplete = false
				}
			}
			if allComplete && len(assemblers) > 0 {
				log.Printf("All images appear to be complete. Waiting for final tiles...")
			}
			continue
		}

		tile := result.ProcessedTile
		
		// Get or create assembler for this image
		assembler, exists := assemblers[tile.ImageID]
		if !exists {
			// Fetch image info from Redis
			imageInfo, err := redisQueue.GetImageInfo(tile.ImageID)
			if err != nil {
				log.Printf("Failed to get image info for ID %d: %v", tile.ImageID, err)
				continue
			}

			// Create new assembler
			assembler = &ImageAssembler{
				imageInfo:   imageInfo,
				tiles:       make(map[int]*common.ProcessedImageTile),
				outputImage: image.NewRGBA(image.Rect(0, 0, imageInfo.Width, imageInfo.Height)),
			}
			assemblers[tile.ImageID] = assembler
			log.Printf("Created assembler for image %d (%s)", tile.ImageID+1, imageInfo.InputPath)
		}

		// Add tile to assembler
		assembler.mutex.Lock()
		assembler.tiles[tile.TileID] = tile
		assembler.tilesReceived++

		// Place tile in output image
		for y := 0; y < tile.Height && y < len(tile.Data); y++ {
			for x := 0; x < tile.Width && x < len(tile.Data[y]); x++ {
				assembler.outputImage.SetRGBA(tile.X+x, tile.Y+y, tile.Data[y][x])
			}
		}

		tilesReceived := assembler.tilesReceived
		expectedTiles := assembler.imageInfo.ExpectedTiles
		assembler.mutex.Unlock()

		log.Printf("Image %d: Received tile %d (worker: %s, %.3fs) - Progress: %d/%d",
			tile.ImageID+1, tile.TileID, result.WorkerID, result.ProcessTime, tilesReceived, expectedTiles)

		// Check if image is complete
		if tilesReceived >= expectedTiles {
			log.Printf("Image %d complete! Saving...", tile.ImageID+1)
			
			// Save the assembled image
			if err := saveImage(assembler.outputImage, assembler.imageInfo.OutputPath); err != nil {
				log.Printf("Failed to save image %d: %v", tile.ImageID+1, err)
			} else {
				processingTime := time.Since(assembler.imageInfo.StartTime)
				log.Printf("Saved image %d to %s (Total time: %.2fs)", 
					tile.ImageID+1, assembler.imageInfo.OutputPath, processingTime.Seconds())
				completedImages++

				// Update image end time in Redis
				endTime := time.Now()
				if err := redisQueue.UpdateImageEndTime(tile.ImageID, endTime); err != nil {
					log.Printf("Failed to update end time for image %d: %v", tile.ImageID+1, err)
				}
			}

			// Clean up
			delete(assemblers, tile.ImageID)

			// Check if we're done
			if completedImages >= *maxImages || (len(assemblers) == 0 && completedImages > 0) {
				log.Printf("All images processed. Total: %d", completedImages)
				
				// Generate and output final performance stats
				if err := outputFinalStats(redisQueue); err != nil {
					log.Printf("Failed to output final stats: %v", err)
				}
				
				break
			}
		}
	}

	log.Printf("Assembler shutting down. Completed %d images.", completedImages)
}

func outputFinalStats(redisQueue *queue.RedisQueue) error {
	// Get timing data from Redis
	timingData, err := redisQueue.GetTiming()
	if err != nil {
		return fmt.Errorf("failed to get timing data: %w", err)
	}
	if timingData == nil {
		return fmt.Errorf("no timing data found")
	}

	// Calculate final end time (latest of all image end times)
	var finalEndTime time.Time
	for _, endTime := range timingData.ImageEndTimes {
		if endTime != nil && endTime.After(finalEndTime) {
			finalEndTime = *endTime
		}
	}

	// Set final end time
	timingData.EndTime = &finalEndTime
	
	// Update timing data in Redis
	if err := redisQueue.StoreTiming(timingData); err != nil {
		log.Printf("Failed to update final timing data: %v", err)
	}

	// Calculate total time
	totalTime := finalEndTime.Sub(timingData.StartTime).Seconds()
	
	// Create performance data for stats package
	performanceData := stats.PerformanceData{
		AlgorithmName:   "Distributed Queue",
		ImagesProcessed: timingData.TotalImages,
		KernelSize:      timingData.KernelSize,
		TotalTime:       totalTime,
		AverageTime:     totalTime / float64(timingData.TotalImages),
		InputPaths:      timingData.InputPaths,
		OutputPaths:     timingData.OutputPaths,
		Timestamp:       timingData.StartTime,
	}

	// Output performance summary
	log.Printf("=== Distributed Queue Processing Complete ===")
	log.Printf("Start time: %s", timingData.StartTime.Format("2006-01-02 15:04:05"))
	log.Printf("End time: %s", finalEndTime.Format("2006-01-02 15:04:05"))
	log.Printf("Total time: %.2fs", totalTime)
	log.Printf("Images processed: %d", timingData.TotalImages)
	log.Printf("Average time per image: %.2fs", performanceData.AverageTime)
	log.Printf("Kernel size: %d", timingData.KernelSize)

	// Individual image times
	for imageID, startTime := range timingData.ImageStartTimes {
		if endTime, exists := timingData.ImageEndTimes[imageID]; exists && endTime != nil {
			imageTime := endTime.Sub(startTime).Seconds()
			log.Printf("Image %d time: %.2fs", imageID+1, imageTime)
		}
	}

	// Write performance results file
	results := []stats.PerformanceData{performanceData}
	stats.WritePerformanceResultsWithPrefix(results, "e_")
	log.Printf("Performance results written to logs/e_*.txt")

	return nil
}

func saveImage(img *image.RGBA, outputPath string) error {
	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Encode and save as PNG
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	return nil
}