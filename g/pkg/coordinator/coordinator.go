package coordinator

import (
    "fmt"
    "image"
    "image/color"
    "log"
    "os"
    "sync"
    "time"

    "go-blur-mt/pkg/queue"
    "studyguide.parallel/pkg/common"
)


type Coordinator struct {
    redisClient *queue.RedisClient
    kernelSize  int
}

func NewCoordinator(redisClient *queue.RedisClient, kernelSize int) *Coordinator {
    return &Coordinator{
        redisClient: redisClient,
        kernelSize:  kernelSize,
    }
}

func (c *Coordinator) ProcessImage(imageID int, inputPath, outputPath string) error {
    log.Printf("Coordinator: Processing image %d from %s", imageID, inputPath)
    startTime := time.Now()
    
    img, err := c.loadImage(inputPath)
    if err != nil {
        return fmt.Errorf("failed to load image: %w", err)
    }
    
    bounds := img.Bounds()
    width := bounds.Dx()
    height := bounds.Dy()
    
    tilesX := (width + common.TILE_SIZE - 1) / common.TILE_SIZE
    tilesY := (height + common.TILE_SIZE - 1) / common.TILE_SIZE
    expectedTiles := tilesX * tilesY
    
    imageInfo := &common.ImageInfo{
        ID:            imageID,
        InputPath:     inputPath,
        OutputPath:    outputPath,
        Width:         width,
        Height:        height,
        ExpectedTiles: expectedTiles,
        StartTime:     startTime,
    }
    
    if err := c.redisClient.StoreImageInfo(imageInfo); err != nil {
        return fmt.Errorf("failed to store image info: %w", err)
    }
    
    log.Printf("Coordinator: Image %d (%dx%d) will generate %d tiles", imageID, width, height, expectedTiles)
    
    if err := c.partitionAndQueue(imageID, img); err != nil {
        return fmt.Errorf("failed to partition image: %w", err)
    }
    
    log.Printf("Coordinator: Finished queuing tiles for image %d in %.2fs", 
        imageID, time.Since(startTime).Seconds())
    
    return nil
}

func (c *Coordinator) ProcessImages(imagePaths []string, outputDir string) error {
    var wg sync.WaitGroup
    errors := make(chan error, len(imagePaths))
    
    for i, inputPath := range imagePaths {
        wg.Add(1)
        go func(id int, path string) {
            defer wg.Done()
            
            outputPath := fmt.Sprintf("%s/img%d_blurred.png", outputDir, id+1)
            if err := c.ProcessImage(id, path, outputPath); err != nil {
                errors <- fmt.Errorf("image %d: %w", id, err)
            }
        }(i, inputPath)
    }
    
    wg.Wait()
    close(errors)
    
    var allErrors []error
    for err := range errors {
        allErrors = append(allErrors, err)
    }
    
    if len(allErrors) > 0 {
        return fmt.Errorf("failed to process %d images", len(allErrors))
    }
    
    return nil
}

func (c *Coordinator) loadImage(path string) (*image.RGBA, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    img, _, err := image.Decode(file)
    if err != nil {
        return nil, err
    }
    
    bounds := img.Bounds()
    rgba := image.NewRGBA(bounds)
    for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
        for x := bounds.Min.X; x < bounds.Max.X; x++ {
            rgba.Set(x, y, img.At(x, y))
        }
    }
    
    return rgba, nil
}

func (c *Coordinator) partitionAndQueue(imageID int, img *image.RGBA) error {
    bounds := img.Bounds()
    padding := c.kernelSize / 2
    tileID := 0
    
    for y := bounds.Min.Y; y < bounds.Max.Y; y += common.TILE_SIZE {
        for x := bounds.Min.X; x < bounds.Max.X; x += common.TILE_SIZE {
            tileWidth := common.TILE_SIZE
            if x+common.TILE_SIZE > bounds.Max.X {
                tileWidth = bounds.Max.X - x
            }
            
            tileHeight := common.TILE_SIZE
            if y+common.TILE_SIZE > bounds.Max.Y {
                tileHeight = bounds.Max.Y - y
            }
            
            tile := c.extractTileWithPadding(img, imageID, tileID, x, y, tileWidth, tileHeight, padding)
            
            job := &common.JobMessage{
                Type:      "tile",
                ImageTile: tile,
            }
            
            if _, err := c.redisClient.AddJob(job); err != nil {
                return fmt.Errorf("failed to queue tile %d: %w", tileID, err)
            }
            
            tileID++
        }
    }
    
    return nil
}

func (c *Coordinator) extractTileWithPadding(img *image.RGBA, imageID, tileID, tileX, tileY, tileWidth, tileHeight, padding int) *common.ImageTile {
    bounds := img.Bounds()
    
    startX := max(bounds.Min.X, tileX-padding)
    startY := max(bounds.Min.Y, tileY-padding)
    endX := min(bounds.Max.X, tileX+tileWidth+padding)
    endY := min(bounds.Max.Y, tileY+tileHeight+padding)
    
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

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}