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
	"studyguide.parallel/pkg/blur"
	"studyguide.parallel/pkg/stats"
)

const (
	TILE_SIZE    = 256
	NUM_WORKERS  = 10
	QUEUE_SIZE   = 100
)

// Enhanced data structures for pipelined processing

// ImageTile represents a tile that belongs to a specific image
type ImageTile struct {
	ImageID int
	TileID  int
	X, Y    int
	Width   int
	Height  int
	Data    [][]color.RGBA
	Padding int
}

// ProcessedImageTile represents a processed tile with image association
type ProcessedImageTile struct {
	ImageID int
	TileID  int
	X, Y    int
	Width   int
	Height  int
	Data    [][]color.RGBA
}

// ImageCommand represents work commands in the pipeline
type ImageCommand struct {
	Type      string // "process", "done"
	ImageTile *ImageTile
}

// ImageInfo holds metadata about an image being processed
type ImageInfo struct {
	ID           int
	InputPath    string
	OutputPath   string
	Width        int
	Height       int
	ExpectedTiles int
	LoadTime     time.Time
	StartTime    time.Time
}

// ImageData combines image info with RGBA data to eliminate matching issues
type ImageData struct {
	Info *ImageInfo
	RGBA *image.RGBA
}

// PipelineReader loads multiple images concurrently
func pipelineReader(imagePaths []string, imageDataChannel chan<- *ImageData) {
	fmt.Println("PipelineReader: Starting...")
	
	var wg sync.WaitGroup
	
	for i, path := range imagePaths {
		wg.Add(1)
		go func(imageID int, imagePath string) {
			defer wg.Done()
			
			startTime := time.Now()
			
			// Load image
			file, err := os.Open(imagePath)
			if err != nil {
				log.Printf("PipelineReader: Failed to open image %d: %v", imageID, err)
				return
			}
			defer file.Close()
			
			img, _, err := image.Decode(file)
			if err != nil {
				log.Printf("PipelineReader: Failed to decode image %d: %v", imageID, err)
				return
			}
			
			// Convert to RGBA
			bounds := img.Bounds()
			rgba := image.NewRGBA(bounds)
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					rgba.Set(x, y, img.At(x, y))
				}
			}
			
			// Calculate expected tiles
			imgWidth := bounds.Dx()
			imgHeight := bounds.Dy()
			tilesX := (imgWidth + TILE_SIZE - 1) / TILE_SIZE
			tilesY := (imgHeight + TILE_SIZE - 1) / TILE_SIZE
			expectedTiles := tilesX * tilesY
			
			// Create output path
			outputPath := fmt.Sprintf("/data/c/output/img%d_blurred.png", imageID+1)
			
			// Create image info
			imageInfo := &ImageInfo{
				ID:           imageID,
				InputPath:    imagePath,
				OutputPath:   outputPath,
				Width:        imgWidth,
				Height:       imgHeight,
				ExpectedTiles: expectedTiles,
				LoadTime:     time.Now(),
				StartTime:    startTime,
			}
			
			// Send combined data - no matching needed!
			imageDataChannel <- &ImageData{
				Info: imageInfo,
				RGBA: rgba,
			}
			
			fmt.Printf("PipelineReader: Loaded image %d (%dx%d) in %.2fms\n", 
				imageID+1, imgWidth, imgHeight,
				float64(time.Since(startTime).Microseconds())/1000.0)
				
		}(i, path)
	}
	
	// Wait for all images to load, then close channels
	go func() {
		wg.Wait()
		close(imageDataChannel)
		fmt.Println("PipelineReader: All images loaded")
	}()
}

// PipelineCoordinator manages tile creation for multiple images
func pipelineCoordinator(imageDataList []*ImageData, tileQueue chan<- ImageCommand, kernelSize int) {
	fmt.Println("PipelineCoordinator: Starting...")
	
	totalImages := len(imageDataList)
	fmt.Printf("PipelineCoordinator: Processing %d images\n", totalImages)
	
	padding := kernelSize / 2
	totalTiles := 0
	
	// Process all images to create tiles
	for _, imgData := range imageDataList {
		imageID := imgData.Info.ID
		img := imgData.RGBA
		
		fmt.Printf("PipelineCoordinator: Creating tiles for image %d\n", imageID+1)
		
		tileID := 0
		bounds := img.Bounds()
		
		// Create tiles with ImageID
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
				imageTile := extractImageTileWithPadding(img, imageID, tileID, x, y, tileWidth, tileHeight, padding)
				
				// Send tile to workers
				tileQueue <- ImageCommand{Type: "process", ImageTile: imageTile}
				tileID++
				totalTiles++
			}
		}
	}
	
	// Send done commands to all workers
	for i := 0; i < NUM_WORKERS; i++ {
		tileQueue <- ImageCommand{Type: "done", ImageTile: nil}
	}
	
	fmt.Printf("PipelineCoordinator: Created %d tiles across %d images\n", totalTiles, totalImages)
}

// extractImageTileWithPadding extracts a tile with image ID tracking
func extractImageTileWithPadding(img *image.RGBA, imageID, tileID, tileX, tileY, tileWidth, tileHeight, padding int) *ImageTile {
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
	
	return &ImageTile{
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

// PipelineWorker processes tiles from multiple images
func pipelineWorker(id int, tileQueue <-chan ImageCommand, resultQueue chan<- *ProcessedImageTile, 
	kernelSize int, wg *sync.WaitGroup) {
	
	defer wg.Done()
	
	fmt.Printf("PipelineWorker %d: Starting...\n", id)
	tilesProcessed := 0
	kernel := blur.GenerateGaussianKernel(kernelSize)
	
	for cmd := range tileQueue {
		if cmd.Type == "done" {
			fmt.Printf("PipelineWorker %d: Processed %d tiles, shutting down\n", id, tilesProcessed)
			return
		}
		
		tile := cmd.ImageTile
		
		// Apply blur to tile
		blurredData := blur.ApplyBlurToTile(tile.Data, kernel)
		
		// Extract center portion (remove padding)
		centerData := blur.ExtractCenter(blurredData, tile.Padding, tile.Width, tile.Height)
		
		// Send processed tile with image ID
		resultQueue <- &ProcessedImageTile{
			ImageID: tile.ImageID,
			TileID:  tile.TileID,
			X:       tile.X,
			Y:       tile.Y,
			Width:   tile.Width,
			Height:  tile.Height,
			Data:    centerData,
		}
		
		tilesProcessed++
	}
}

// PipelineAssemblerManager manages multiple assemblers
func pipelineAssemblerManager(resultQueue <-chan *ProcessedImageTile, imageInfos []*ImageInfo, assemblerWG *sync.WaitGroup) {
	defer assemblerWG.Done()
	
	fmt.Println("PipelineAssemblerManager: Starting...")
	
	// Create assemblers for each image
	assemblerChannels := make(map[int]chan *ProcessedImageTile)
	var internalAssemblerWG sync.WaitGroup
	
	for _, info := range imageInfos {
		assemblerChan := make(chan *ProcessedImageTile, 100)
		assemblerChannels[info.ID] = assemblerChan
		
		internalAssemblerWG.Add(1)
		go func(imageInfo *ImageInfo, tileChan <-chan *ProcessedImageTile) {
			defer internalAssemblerWG.Done()
			pipelineAssembler(imageInfo, tileChan)
		}(info, assemblerChan)
	}
	
	// Route tiles to appropriate assemblers
	for tile := range resultQueue {
		if assemblerChan, exists := assemblerChannels[tile.ImageID]; exists {
			assemblerChan <- tile
		}
	}
	
	// Close all assembler channels
	for _, ch := range assemblerChannels {
		close(ch)
	}
	
	// Wait for all assemblers to finish
	internalAssemblerWG.Wait()
	fmt.Println("PipelineAssemblerManager: All assemblers finished")
}

// PipelineAssembler reconstructs a single image
func pipelineAssembler(imageInfo *ImageInfo, tileChannel <-chan *ProcessedImageTile) {
	fmt.Printf("PipelineAssembler: Starting for image %d\n", imageInfo.ID+1)
	startTime := time.Now()
	
	// Create output image
	output := image.NewRGBA(image.Rect(0, 0, imageInfo.Width, imageInfo.Height))
	tilesReceived := 0
	
	// Collect all tiles for this image
	for tile := range tileChannel {
		// Place tile in output image
		for y := 0; y < tile.Height; y++ {
			for x := 0; x < tile.Width; x++ {
				output.Set(tile.X+x, tile.Y+y, tile.Data[y][x])
			}
		}
		
		tilesReceived++
	}
	
	// Save output
	outFile, err := os.Create(imageInfo.OutputPath)
	if err != nil {
		log.Printf("PipelineAssembler: Failed to create output file for image %d: %v", imageInfo.ID+1, err)
		return
	}
	defer outFile.Close()
	
	err = png.Encode(outFile, output)
	if err != nil {
		log.Printf("PipelineAssembler: Failed to encode image %d: %v", imageInfo.ID+1, err)
		return
	}
	
	totalTime := time.Since(imageInfo.StartTime).Seconds()
	assemblerTime := time.Since(startTime).Seconds()
	
	fmt.Printf("PipelineAssembler: Image %d complete - %d tiles in %.2fs (total: %.2fs)\n", 
		imageInfo.ID+1, tilesReceived, assemblerTime, totalTime)
}

// RunPipelined executes the pipelined blur pipeline
func Run_c(inputPaths []string, kernelSize int) stats.PerformanceData {
	fmt.Println("=== Starting Pipelined Multi-Image Gaussian Blur ===")
	startTime := time.Now()
	
	// Create channels
	imageDataChannel := make(chan *ImageData, len(inputPaths))
	tileQueue := make(chan ImageCommand, QUEUE_SIZE*2) // Larger queue for multiple images
	resultQueue := make(chan *ProcessedImageTile, QUEUE_SIZE*2)
	
	// Start pipeline reader
	go pipelineReader(inputPaths, imageDataChannel) // Concurrent loading of images
	
	// Collect image data and infos for coordinator and assembler manager
	var imageDataList []*ImageData // Used by coordinator
	var imageInfos []*ImageInfo // Used by assembler manager	
	for imgData := range imageDataChannel {
		imageDataList = append(imageDataList, imgData) 
		imageInfos = append(imageInfos, imgData.Info) 
	}
	
	// Start coordinator with collected data
	go pipelineCoordinator(imageDataList, tileQueue, kernelSize)
	
	// Start workers
	var workerWG sync.WaitGroup
	workerWG.Add(NUM_WORKERS)
	for i := 0; i < NUM_WORKERS; i++ {
		go pipelineWorker(i, tileQueue, resultQueue, kernelSize, &workerWG)
	}
	
	// Start assembler manager
	var assemblerWG sync.WaitGroup
	assemblerWG.Add(1)
	go pipelineAssemblerManager(resultQueue, imageInfos, &assemblerWG)
	
	// Wait for workers to finish
	workerWG.Wait()
	close(resultQueue)
	
	// Wait for assemblers to finish (proper synchronization!)
	assemblerWG.Wait()
	
	totalTime := time.Since(startTime).Seconds()
	fmt.Printf("\n=== Pipelined Multi-Image Blur Complete ===\n")
	fmt.Printf("Images processed: %d\n", len(inputPaths))
	fmt.Printf("Total execution time: %.2fs\n", totalTime)
	fmt.Printf("Average time per image: %.2fs\n", totalTime/float64(len(inputPaths)))
	
	// Generate output paths for return data
	var outputPaths []string
	for i := range inputPaths {
		outputName := fmt.Sprintf("/data/c/output/img%d_blurred.png", i+1)
		outputPaths = append(outputPaths, outputName)
	}
	
	workers := NUM_WORKERS
	tileSize := TILE_SIZE
	queueSize := QUEUE_SIZE
	
	return stats.PerformanceData{
		AlgorithmName:   "Pipelined",
		ImagesProcessed: len(inputPaths),
		KernelSize:      kernelSize,
		TotalTime:       totalTime,
		AverageTime:     totalTime / float64(len(inputPaths)),
		InputPaths:      inputPaths,
		OutputPaths:     outputPaths,
		Timestamp:       startTime,
		Workers:         &workers,
		TileSize:        &tileSize,
		QueueSize:       &queueSize,
	}
}