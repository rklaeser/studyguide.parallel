package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"studyguide.parallel/pkg/blur"
	"studyguide.parallel/pkg/stats"
)

func main() {
	var (
		inputPath  = flag.String("input", "/input", "Input directory path")
		outputPath = flag.String("output", "/data/b/output", "Output directory path")
		kernelSize = flag.Int("kernel", 15, "Gaussian kernel size")
	)
	flag.Parse()

	startTime := time.Now()
	log.Printf("=== Starting Tile Parallel Image Processing ===")
	log.Printf("Start time: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("Kernel size: %d", *kernelSize)
	log.Printf("Input path: %s", *inputPath)
	log.Printf("Output path: %s", *outputPath)

	// Create output directory
	if err := os.MkdirAll(*outputPath, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Find all PNG files in input directory
	files, err := filepath.Glob(filepath.Join(*inputPath, "*.png"))
	if err != nil {
		log.Fatalf("Failed to find input files: %v", err)
	}

	if len(files) == 0 {
		log.Fatalf("No PNG files found in %s", *inputPath)
	}

	// Create input and output paths for Run_b
	var inputPaths []string
	var outputPaths []string

	for _, file := range files {
		inputPaths = append(inputPaths, file)
		
		// Generate output filename
		filename := filepath.Base(file)
		name := strings.TrimSuffix(filename, filepath.Ext(filename))
		outputFile := filepath.Join(*outputPath, name+"_blurred.png")
		outputPaths = append(outputPaths, outputFile)
	}

	log.Printf("Found %d images to process", len(inputPaths))

	// Process images with tile parallelism
	result := processTileParallel(inputPaths, outputPaths, *kernelSize)

	// Write performance results
	results := []stats.PerformanceData{result}
	stats.WritePerformanceResults(results)

	log.Printf("=== Processing Complete ===")
	log.Printf("Total execution time: %.2fs", time.Since(startTime).Seconds())
}

const (
	TILE_SIZE    = 256
	NUM_WORKERS  = 10
	QUEUE_SIZE   = 100
)

// Tile represents a portion of the image with padding for blur
type Tile struct {
	ID     int
	X, Y   int              // Position in original image
	Width  int              // Actual tile dimensions (without padding)
	Height int              
	Data   [][]color.RGBA   // Padded tile data
	Padding int             // Overlap amount
}

// ProcessedTile represents a tile after blur has been applied
type ProcessedTile struct {
	ID     int
	X, Y   int
	Width  int
	Height int
	Data   [][]color.RGBA  // Center portion only (padding removed)
}

// Command represents work to be done
type Command struct {
	Type string  // "process", "done"
	Tile *Tile
}

func processTileParallel(inputPaths []string, outputPaths []string, kernelSize int) stats.PerformanceData {
	fmt.Println("=== Starting Parallel Multi-Image Gaussian Blur ===")
	startTime := time.Now()
	
	if len(inputPaths) != len(outputPaths) {
		log.Fatalf("Input and output path arrays must have same length")
	}

	totalBlurTime := 0.0
	
	for i, inputPath := range inputPaths {
		imageTime, err := runTileParallelSingle(inputPath, outputPaths[i], kernelSize)
		if err != nil {
			log.Fatalf("Error processing image %d: %v", i+1, err)
		}
		totalBlurTime += imageTime
	}

	totalTime := time.Since(startTime).Seconds()
	fmt.Printf("\n=== Parallel Multi-Image Blur Complete ===\n")
	fmt.Printf("Images processed: %d\n", len(inputPaths))
	fmt.Printf("Total blur time: %.2fs\n", totalBlurTime)
	fmt.Printf("Total execution time: %.2fs\n", totalTime)
	fmt.Printf("Average time per image: %.2fs\n", totalTime/float64(len(inputPaths)))
	
	return stats.PerformanceData{
		AlgorithmName:   "Parallel",
		ImagesProcessed: len(inputPaths),
		KernelSize:      kernelSize,
		TotalTime:       totalTime,
		AverageTime:     totalTime / float64(len(inputPaths)),
		InputPaths:      inputPaths,
		OutputPaths:     outputPaths,
	}
}

func runTileParallelSingle(inputPath, outputPath string, kernelSize int) (float64, error) {
	startTime := time.Now()
	
	// Load image
	img, err := loadImage(inputPath)
	if err != nil {
		return 0, err
	}

	fmt.Printf("  Processing %s (%dx%d)...", filepath.Base(inputPath), img.Bounds().Dx(), img.Bounds().Dy())

	// Process with tile parallelism
	result := processImageWithTiles(img, kernelSize)

	// Save result
	err = saveImage(result, outputPath)
	if err != nil {
		return 0, err
	}

	elapsed := time.Since(startTime).Seconds()
	fmt.Printf(" %.2fs\n", elapsed)
	
	return elapsed, nil
}

func loadImage(imagePath string) (*image.RGBA, error) {
	file, err := os.Open(imagePath)
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

func saveImage(img *image.RGBA, outputPath string) error {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	return png.Encode(outputFile, img)
}

func processImageWithTiles(img *image.RGBA, kernelSize int) *image.RGBA {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	padding := kernelSize / 2

	// Create tile work queue
	tileQueue := make(chan *Tile, QUEUE_SIZE)
	resultQueue := make(chan *ProcessedTile, QUEUE_SIZE)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < NUM_WORKERS; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			tileWorker(workerID, tileQueue, resultQueue, kernelSize)
		}(i)
	}

	// Start coordinator
	go tileCoordinator(img, tileQueue, padding)

	// Collect results
	result := image.NewRGBA(bounds)
	tilesX := (width + TILE_SIZE - 1) / TILE_SIZE
	tilesY := (height + TILE_SIZE - 1) / TILE_SIZE
	totalTiles := tilesX * tilesY

	for i := 0; i < totalTiles; i++ {
		processedTile := <-resultQueue
		assembleProcessedTile(result, processedTile)
	}

	// Close workers
	close(tileQueue)
	wg.Wait()
	close(resultQueue)

	return result
}

func tileCoordinator(img *image.RGBA, tileQueue chan<- *Tile, padding int) {
	fmt.Println("Coordinator: Starting...")
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	tileID := 0
	tilesX := (width + TILE_SIZE - 1) / TILE_SIZE
	tilesY := (height + TILE_SIZE - 1) / TILE_SIZE
	
	fmt.Printf("Coordinator: Creating %d tiles (%dx%d grid) with %d pixel padding\n", 
		tilesX*tilesY, tilesX, tilesY, padding)
	
	for tileY := 0; tileY < tilesY; tileY++ {
		for tileX := 0; tileX < tilesX; tileX++ {
			startX := tileX * TILE_SIZE
			startY := tileY * TILE_SIZE
			
			endX := startX + TILE_SIZE
			if endX > width {
				endX = width
			}
			
			endY := startY + TILE_SIZE
			if endY > height {
				endY = height
			}
			
			tileWidth := endX - startX
			tileHeight := endY - startY
			
			// Extract tile with padding
			tileData := extractTileWithPadding(img, startX, startY, tileWidth, tileHeight, padding)
			
			tile := &Tile{
				ID: tileID,
				X: startX,
				Y: startY,
				Width: tileWidth,
				Height: tileHeight,
				Data: tileData,
				Padding: padding,
			}
			
			tileQueue <- tile
			tileID++
		}
	}
	
	fmt.Printf("Coordinator: Finished creating %d tiles\n", tileID)
}

func extractTileWithPadding(img *image.RGBA, startX, startY, width, height, padding int) [][]color.RGBA {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()
	
	paddedWidth := width + 2*padding
	paddedHeight := height + 2*padding
	
	tileData := make([][]color.RGBA, paddedHeight)
	for i := range tileData {
		tileData[i] = make([]color.RGBA, paddedWidth)
	}
	
	for y := 0; y < paddedHeight; y++ {
		for x := 0; x < paddedWidth; x++ {
			srcX := startX + x - padding
			srcY := startY + y - padding
			
			// Clamp to image boundaries
			if srcX < 0 { srcX = 0 }
			if srcY < 0 { srcY = 0 }
			if srcX >= imgWidth { srcX = imgWidth - 1 }
			if srcY >= imgHeight { srcY = imgHeight - 1 }
			
			tileData[y][x] = img.RGBAAt(srcX, srcY)
		}
	}
	
	return tileData
}

func tileWorker(workerID int, tileQueue <-chan *Tile, resultQueue chan<- *ProcessedTile, kernelSize int) {
	fmt.Printf("Worker %d: Starting...\n", workerID)
	tilesProcessed := 0
	
	for tile := range tileQueue {
		// Apply blur to tile
		blurredData := blur.ApplyBlurToTile(tile.Data, blur.GenerateGaussianKernel(kernelSize))
		
		// Remove padding (extract center)
		centerData := blur.ExtractCenter(blurredData, tile.Padding, tile.Width, tile.Height)
		
		processedTile := &ProcessedTile{
			ID: tile.ID,
			X: tile.X,
			Y: tile.Y,
			Width: tile.Width,
			Height: tile.Height,
			Data: centerData,
		}
		
		resultQueue <- processedTile
		tilesProcessed++
	}
	
	fmt.Printf("Worker %d: Processed %d tiles, shutting down\n", workerID, tilesProcessed)
}


func assembleProcessedTile(result *image.RGBA, processedTile *ProcessedTile) {
	for y := 0; y < processedTile.Height; y++ {
		for x := 0; x < processedTile.Width; x++ {
			pixelData := processedTile.Data[y][x]
			result.Set(processedTile.X+x, processedTile.Y+y, pixelData)
		}
	}
}