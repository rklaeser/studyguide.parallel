package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"sync"
	"time"
)

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

// Command represents work to be done (like the C++ Command pattern)
type Command struct {
	Type string  // "process", "done"
	Tile *Tile
}

// ImageReader loads the image (like FileThread)
func imageReader(imagePath string, imageChannel chan<- *image.RGBA) {
	fmt.Println("ImageReader: Starting...")
	startTime := time.Now()
	
	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatalf("ImageReader: Failed to open image: %v", err)
	}
	defer file.Close()
	
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatalf("ImageReader: Failed to decode image: %v", err)
	}
	
	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	
	fmt.Printf("ImageReader: Loaded image %dx%d in %.2fms\n", 
		bounds.Dx(), bounds.Dy(), 
		float64(time.Since(startTime).Microseconds())/1000.0)
	
	imageChannel <- rgba
	close(imageChannel)
}

// Coordinator partitions the image into tiles (like CoordThread)
func coordinator(imageChannel <-chan *image.RGBA, tileQueue chan<- Command, kernelSize int) {
	fmt.Println("Coordinator: Starting...")
	
	img := <-imageChannel
	if img == nil {
		fmt.Println("Coordinator: No image received")
		return
	}
	
	bounds := img.Bounds()
	padding := kernelSize / 2
	tileID := 0
	
	// Calculate number of tiles
	tilesX := (bounds.Dx() + TILE_SIZE - 1) / TILE_SIZE
	tilesY := (bounds.Dy() + TILE_SIZE - 1) / TILE_SIZE
	totalTiles := tilesX * tilesY
	
	fmt.Printf("Coordinator: Creating %d tiles (%dx%d grid) with %d pixel padding\n", 
		totalTiles, tilesX, tilesY, padding)
	
	// Create tiles with overlap
	for y := bounds.Min.Y; y < bounds.Max.Y; y += TILE_SIZE {
		for x := bounds.Min.X; x < bounds.Max.X; x += TILE_SIZE {
			// Calculate tile dimensions
			tileWidth := TILE_SIZE
			if x+TILE_SIZE > bounds.Max.X {
				tileWidth = bounds.Max.X - x
			}
			tileHeight := TILE_SIZE
			if y+TILE_SIZE > bounds.Max.Y {
				tileHeight = bounds.Max.Y - y
			}
			
			// Extract tile with padding
			tile := extractTileWithPadding(img, x, y, tileWidth, tileHeight, padding)
			tile.ID = tileID
			tileID++
			
			// Send tile to workers
			tileQueue <- Command{Type: "process", Tile: tile}
		}
	}
	
	// Send done commands to all workers
	for i := 0; i < NUM_WORKERS; i++ {
		tileQueue <- Command{Type: "done", Tile: nil}
	}
	
	fmt.Printf("Coordinator: Finished creating %d tiles\n", tileID)
}

// extractTileWithPadding extracts a tile with padding for seamless blur
func extractTileWithPadding(img *image.RGBA, tileX, tileY, tileWidth, tileHeight, padding int) *Tile {
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
	
	return &Tile{
		X:       tileX,
		Y:       tileY,
		Width:   tileWidth,
		Height:  tileHeight,
		Data:    data,
		Padding: padding,
	}
}

// blurWorker processes tiles (like WaveThread)
func blurWorker(id int, tileQueue <-chan Command, resultQueue chan<- *ProcessedTile, kernelSize int, wg *sync.WaitGroup) {
	defer wg.Done()
	
	fmt.Printf("Worker %d: Starting...\n", id)
	tilesProcessed := 0
	kernel := generateGaussianKernel(kernelSize)
	
	for cmd := range tileQueue {
		if cmd.Type == "done" {
			fmt.Printf("Worker %d: Processed %d tiles, shutting down\n", id, tilesProcessed)
			return
		}
		
		// Apply blur to tile
		blurredData := applyBlurToTile(cmd.Tile.Data, kernel)
		
		// Extract center portion (remove padding)
		centerData := extractCenter(blurredData, cmd.Tile.Padding, cmd.Tile.Width, cmd.Tile.Height)
		
		// Send processed tile
		resultQueue <- &ProcessedTile{
			ID:     cmd.Tile.ID,
			X:      cmd.Tile.X,
			Y:      cmd.Tile.Y,
			Width:  cmd.Tile.Width,
			Height: cmd.Tile.Height,
			Data:   centerData,
		}
		
		tilesProcessed++
	}
}

// applyBlurToTile applies Gaussian blur to tile data
func applyBlurToTile(data [][]color.RGBA, kernel [][]float64) [][]color.RGBA {
	height := len(data)
	width := len(data[0])
	kernelSize := len(kernel)
	offset := kernelSize / 2
	
	result := make([][]color.RGBA, height)
	for i := range result {
		result[i] = make([]color.RGBA, width)
	}
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rSum, gSum, bSum, aSum float64
			
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					sx := x + kx - offset
					sy := y + ky - offset
					
					// Handle boundaries
					if sx < 0 {
						sx = 0
					}
					if sx >= width {
						sx = width - 1
					}
					if sy < 0 {
						sy = 0
					}
					if sy >= height {
						sy = height - 1
					}
					
					pixel := data[sy][sx]
					weight := kernel[ky][kx]
					
					rSum += float64(pixel.R) * weight
					gSum += float64(pixel.G) * weight
					bSum += float64(pixel.B) * weight
					aSum += float64(pixel.A) * weight
				}
			}
			
			result[y][x] = color.RGBA{
				R: uint8(rSum),
				G: uint8(gSum),
				B: uint8(bSum),
				A: uint8(aSum),
			}
		}
	}
	
	return result
}

// extractCenter removes padding from blurred tile
func extractCenter(data [][]color.RGBA, padding, width, height int) [][]color.RGBA {
	result := make([][]color.RGBA, height)
	
	for y := 0; y < height; y++ {
		result[y] = make([]color.RGBA, width)
		for x := 0; x < width; x++ {
			if y+padding < len(data) && x+padding < len(data[0]) {
				result[y][x] = data[y+padding][x+padding]
			}
		}
	}
	
	return result
}

// assembler reconstructs the final image
func assembler(resultQueue <-chan *ProcessedTile, outputPath string, imgWidth, imgHeight, expectedTiles int) {
	fmt.Println("Assembler: Starting...")
	startTime := time.Now()
	
	// Create output image
	output := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	tilesReceived := 0
	
	// Collect all tiles
	for tilesReceived < expectedTiles {
		tile := <-resultQueue
		
		// Place tile in output image
		for y := 0; y < tile.Height; y++ {
			for x := 0; x < tile.Width; x++ {
				output.Set(tile.X+x, tile.Y+y, tile.Data[y][x])
			}
		}
		
		tilesReceived++
		if tilesReceived%10 == 0 {
			fmt.Printf("Assembler: Processed %d/%d tiles\n", tilesReceived, expectedTiles)
		}
	}
	
	// Save output
	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("Assembler: Failed to create output file: %v", err)
	}
	defer outFile.Close()
	
	err = png.Encode(outFile, output)
	if err != nil {
		log.Fatalf("Assembler: Failed to encode image: %v", err)
	}
	
	fmt.Printf("Assembler: Completed in %.2fms\n", 
		float64(time.Since(startTime).Microseconds())/1000.0)
}

// RunParallelSingle executes the parallel blur pipeline for a single image
func RunParallelSingle(inputPath, outputPath string, kernelSize int) (float64, error) {
	startTime := time.Now()
	
	// Create channels
	imageChannel := make(chan *image.RGBA, 1)
	tileQueue := make(chan Command, QUEUE_SIZE)
	resultQueue := make(chan *ProcessedTile, QUEUE_SIZE)
	
	// Start image reader
	go imageReader(inputPath, imageChannel)
	
	// Wait for image to get dimensions
	img := <-imageChannel
	if img == nil {
		return 0, fmt.Errorf("failed to load image")
	}
	
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()
	
	fmt.Printf("  Processing %s (%dx%d)...", inputPath, imgWidth, imgHeight)
	
	// Calculate expected tiles
	tilesX := (imgWidth + TILE_SIZE - 1) / TILE_SIZE
	tilesY := (imgHeight + TILE_SIZE - 1) / TILE_SIZE
	expectedTiles := tilesX * tilesY
	
	// Re-send image to coordinator
	coordImageChannel := make(chan *image.RGBA, 1)
	coordImageChannel <- img
	close(coordImageChannel)
	
	// Start coordinator
	go coordinator(coordImageChannel, tileQueue, kernelSize)
	
	// Start workers
	var wg sync.WaitGroup
	wg.Add(NUM_WORKERS)
	for i := 0; i < NUM_WORKERS; i++ {
		go blurWorker(i, tileQueue, resultQueue, kernelSize, &wg)
	}
	
	// Start assembler in goroutine
	assemblerDone := make(chan bool)
	go func() {
		assemblerQuiet(resultQueue, outputPath, imgWidth, imgHeight, expectedTiles)
		assemblerDone <- true
	}()
	
	// Wait for assembler to finish
	<-assemblerDone
	
	// Wait for workers to finish
	wg.Wait()
	close(resultQueue)
	
	duration := time.Since(startTime).Seconds()
	fmt.Printf(" %.2fs\n", duration)
	return duration, nil
}

// assemblerQuiet is like assembler but with minimal output for multi-image processing
func assemblerQuiet(resultQueue <-chan *ProcessedTile, outputPath string, imgWidth, imgHeight, expectedTiles int) {
	// Create output image
	output := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	tilesReceived := 0
	
	// Collect all tiles
	for tilesReceived < expectedTiles {
		tile := <-resultQueue
		
		// Place tile in output image
		for y := 0; y < tile.Height; y++ {
			for x := 0; x < tile.Width; x++ {
				output.Set(tile.X+x, tile.Y+y, tile.Data[y][x])
			}
		}
		
		tilesReceived++
	}
	
	// Save output
	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Printf("Failed to create output file: %v", err)
		return
	}
	defer outFile.Close()
	
	err = png.Encode(outFile, output)
	if err != nil {
		log.Printf("Failed to encode image: %v", err)
		return
	}
}

// RunParallelMultiple executes parallel blur for multiple images
func RunParallelMultiple(inputPaths []string, outputPaths []string, kernelSize int) {
	fmt.Println("=== Starting Parallel Multi-Image Gaussian Blur ===")
	startTime := time.Now()
	
	if len(inputPaths) != len(outputPaths) {
		log.Fatalf("Input and output path arrays must have same length")
	}

	totalBlurTime := 0.0
	
	for i, inputPath := range inputPaths {
		imageTime, err := RunParallelSingle(inputPath, outputPaths[i], kernelSize)
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
}