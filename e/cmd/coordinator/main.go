package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-blur/pkg/common"
	"go-blur/pkg/queue"
)

func main() {
	var (
		inputPath   = flag.String("input", "/input", "Input directory path")
		outputPath  = flag.String("output", "/e/output", "Output directory path")
		kernelSize  = flag.Int("kernel", 15, "Gaussian kernel size")
		redisAddr   = flag.String("redis", "redis:6379", "Redis server address")
		numWorkers  = flag.Int("workers", 4, "Number of workers to wait for")
	)
	flag.Parse()

	log.Printf("Coordinator starting...")
	log.Printf("Input path: %s", *inputPath)
	log.Printf("Output path: %s", *outputPath)
	log.Printf("Kernel size: %d", *kernelSize)
	log.Printf("Redis address: %s", *redisAddr)

	// Connect to Redis
	redisQueue, err := queue.NewRedisQueue(*redisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	// Get list of images
	imagePaths, err := getImagePaths(*inputPath)
	if err != nil {
		log.Fatalf("Failed to get image paths: %v", err)
	}

	if len(imagePaths) == 0 {
		log.Println("No images found to process")
		return
	}

	log.Printf("Found %d images to process", len(imagePaths))

	// Initialize timing data
	startTime := time.Now()
	
	timingData := &common.TimingData{
		StartTime:       startTime,
		KernelSize:      *kernelSize,
		TotalImages:     len(imagePaths),
		InputPaths:      make([]string, 0, len(imagePaths)),
		OutputPaths:     make([]string, 0, len(imagePaths)),
		ImageStartTimes: make(map[int]time.Time),
		ImageEndTimes:   make(map[int]*time.Time),
	}

	// Process each image
	padding := *kernelSize / 2
	totalTiles := 0

	for imageID, imagePath := range imagePaths {
		log.Printf("Processing image %d: %s", imageID+1, imagePath)
		imageStartTime := time.Now()
		
		// Load image
		img, err := loadImage(imagePath)
		if err != nil {
			log.Printf("Failed to load image %s: %v", imagePath, err)
			continue
		}

		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()

		// Calculate tiles
		tilesX := (width + common.TILE_SIZE - 1) / common.TILE_SIZE
		tilesY := (height + common.TILE_SIZE - 1) / common.TILE_SIZE
		expectedTiles := tilesX * tilesY

		// Create output path
		baseName := filepath.Base(imagePath)
		nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		outputFile := filepath.Join(*outputPath, fmt.Sprintf("%s_blurred.png", nameWithoutExt))

		// Add to timing data
		timingData.InputPaths = append(timingData.InputPaths, imagePath)
		timingData.OutputPaths = append(timingData.OutputPaths, outputFile)
		timingData.ImageStartTimes[imageID] = imageStartTime

		// Store image info
		imageInfo := &common.ImageInfo{
			ID:            imageID,
			InputPath:     imagePath,
			OutputPath:    outputFile,
			Width:         width,
			Height:        height,
			ExpectedTiles: expectedTiles,
			LoadTime:      time.Now(),
			StartTime:     imageStartTime,
		}

		if err := redisQueue.StoreImageInfo(imageInfo); err != nil {
			log.Printf("Failed to store image info: %v", err)
			continue
		}

		// Create tiles and push to queue
		tileID := 0
		for y := bounds.Min.Y; y < bounds.Max.Y; y += common.TILE_SIZE {
			for x := bounds.Min.X; x < bounds.Max.X; x += common.TILE_SIZE {
				// Calculate tile dimensions
				tileWidth := common.TILE_SIZE
				if x+common.TILE_SIZE > bounds.Max.X {
					tileWidth = bounds.Max.X - x
				}
				tileHeight := common.TILE_SIZE
				if y+common.TILE_SIZE > bounds.Max.Y {
					tileHeight = bounds.Max.Y - y
				}

				// Extract tile with padding
				tile := extractTileWithPadding(img, imageID, tileID, x, y, tileWidth, tileHeight, padding)

				// Push to queue
				job := &common.JobMessage{
					Type:      "tile",
					ImageTile: tile,
				}

				if err := redisQueue.PushJob(job); err != nil {
					log.Printf("Failed to push tile job: %v", err)
				} else {
					totalTiles++
				}

				tileID++
			}
		}

		log.Printf("Created %d tiles for image %d", expectedTiles, imageID+1)
	}

	// Send done signals for workers
	for i := 0; i < *numWorkers; i++ {
		job := &common.JobMessage{
			Type: "complete",
		}
		if err := redisQueue.PushJob(job); err != nil {
			log.Printf("Failed to push done signal: %v", err)
		}
	}

	// Store timing data in Redis for assembler to use
	if err := redisQueue.StoreTiming(timingData); err != nil {
		log.Printf("Failed to store timing data: %v", err)
	}

	log.Printf("Coordinator finished. Created %d total tiles across %d images", totalTiles, len(imagePaths))
	log.Printf("Coordination time: %.2fs", time.Since(startTime).Seconds())
}

func getImagePaths(inputDir string) ([]string, error) {
	var paths []string

	files, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			paths = append(paths, filepath.Join(inputDir, file.Name()))
		}
	}

	return paths, nil
}

func loadImage(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	return rgba, nil
}

func extractTileWithPadding(img *image.RGBA, imageID, tileID, tileX, tileY, tileWidth, tileHeight, padding int) *common.ImageTile {
	bounds := img.Bounds()

	// Calculate padded dimensions
	startX := tileX - padding
	startY := tileY - padding
	endX := tileX + tileWidth + padding
	endY := tileY + tileHeight + padding

	// Clamp to image bounds
	if startX < bounds.Min.X {
		startX = bounds.Min.X
	}
	if startY < bounds.Min.Y {
		startY = bounds.Min.Y
	}
	if endX > bounds.Max.X {
		endX = bounds.Max.X
	}
	if endY > bounds.Max.Y {
		endY = bounds.Max.Y
	}

	// Create tile data
	paddedWidth := endX - startX
	paddedHeight := endY - startY
	data := make([][]color.RGBA, paddedHeight)

	for y := 0; y < paddedHeight; y++ {
		data[y] = make([]color.RGBA, paddedWidth)
		for x := 0; x < paddedWidth; x++ {
			data[y][x] = img.RGBAAt(startX+x, startY+y)
		}
	}

	return &common.ImageTile{
		ImageID: imageID,
		TileID:  tileID,
		X:       tileX,
		Y:       tileY,
		Width:   tileWidth,
		Height:  tileHeight,
		Data:    data,
		Padding: padding,
	}
}