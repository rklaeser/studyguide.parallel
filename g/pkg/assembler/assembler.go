package assembler

import (
    "context"
    "fmt"
    "image"
    "image/png"
    "log"
    "os"
    "sync"
    "time"

    "go-blur-mt/pkg/queue"
    "studyguide.parallel/pkg/common"
)

type Assembler struct {
    redisClient *queue.RedisClient
    assemblerID string
    imageMap    map[int]*ImageAssembly
    mutex       sync.RWMutex
    ctx         context.Context
    cancel      context.CancelFunc
}

type ImageAssembly struct {
    info          *common.ImageInfo
    outputImage   *image.RGBA
    tilesReceived int
    processedTiles map[int]bool  // In-memory duplicate tracking
    completed     bool
    mutex         sync.Mutex
}

func NewAssembler(redisClient *queue.RedisClient, assemblerID string) *Assembler {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Assembler{
        redisClient: redisClient,
        assemblerID: assemblerID,
        imageMap:    make(map[int]*ImageAssembly),
        ctx:         ctx,
        cancel:      cancel,
    }
}

func (a *Assembler) Start() {
    var wg sync.WaitGroup
    
    wg.Add(1)
    go a.resultProcessor(&wg)
    
    wg.Add(1)
    go a.checkpointMonitor(&wg)
    
    log.Printf("Assembler %s started", a.assemblerID)
    wg.Wait()
}

func (a *Assembler) Stop() {
    log.Println("Assembler: Shutting down...")
    a.cancel()
}

func (a *Assembler) resultProcessor(wg *sync.WaitGroup) {
    defer wg.Done()
    
    consumer := fmt.Sprintf("assembler-%s", a.assemblerID)
    
    for {
        select {
        case <-a.ctx.Done():
            return
        default:
            msgID, result, err := a.redisClient.ReadResult(consumer, 5*time.Second)
            if err != nil {
                if err.Error() != "redis: nil" {
                    log.Printf("Assembler read error: %v", err)
                }
                continue
            }
            
            if result == nil || result.ProcessedTile == nil {
                continue
            }
            
            if err := a.processTile(result.ProcessedTile); err != nil {
                log.Printf("Failed to process tile: %v", err)
            } else {
                _ = a.redisClient.AckResult(msgID)
            }
        }
    }
}

func (a *Assembler) processTile(tile *common.ProcessedImageTile) error {
    
    assembly, err := a.getOrCreateAssembly(tile.ImageID)
    if err != nil {
        return fmt.Errorf("failed to get assembly: %w", err)
    }
    
    assembly.mutex.Lock()
    defer assembly.mutex.Unlock()
    
    if assembly.completed {
        return nil
    }
    
    // Check for duplicate tiles (in-memory idempotency)
    if assembly.processedTiles[tile.TileID] {
        log.Printf("Tile %d for image %d already processed (idempotent)", tile.TileID, tile.ImageID)
        return nil
    }
    
    // Mark tile as processed
    assembly.processedTiles[tile.TileID] = true
    
    for y := 0; y < tile.Height && y < len(tile.Data); y++ {
        for x := 0; x < tile.Width && x < len(tile.Data[y]); x++ {
            assembly.outputImage.Set(tile.X+x, tile.Y+y, tile.Data[y][x])
        }
    }
    
    assembly.tilesReceived++
    
    if assembly.tilesReceived == assembly.info.ExpectedTiles {
        if err := a.saveImage(assembly); err != nil {
            return fmt.Errorf("failed to save image: %w", err)
        }
        assembly.completed = true
        
        // Mark image as completed in Redis
        if err := a.redisClient.MarkImageCompleted(tile.ImageID); err != nil {
            log.Printf("Warning: failed to mark image %d as completed in Redis: %v", tile.ImageID, err)
        }
        
        duration := time.Since(assembly.info.StartTime).Seconds()
        log.Printf("Image %d assembled: %d tiles in %.2fs", 
            tile.ImageID, assembly.tilesReceived, duration)
    } else if assembly.tilesReceived%10 == 0 {
        log.Printf("Image %d progress: %d/%d tiles", 
            tile.ImageID, assembly.tilesReceived, assembly.info.ExpectedTiles)
    }
    
    return nil
}

func (a *Assembler) getOrCreateAssembly(imageID int) (*ImageAssembly, error) {
    a.mutex.RLock()
    if assembly, exists := a.imageMap[imageID]; exists {
        a.mutex.RUnlock()
        return assembly, nil
    }
    a.mutex.RUnlock()
    
    a.mutex.Lock()
    defer a.mutex.Unlock()
    
    if assembly, exists := a.imageMap[imageID]; exists {
        return assembly, nil
    }
    
    info, err := a.redisClient.GetImageInfo(imageID)
    if err != nil {
        return nil, fmt.Errorf("failed to get image info: %w", err)
    }
    
    assembly := &ImageAssembly{
        info:           info,
        outputImage:    image.NewRGBA(image.Rect(0, 0, info.Width, info.Height)),
        tilesReceived:  0,
        processedTiles: make(map[int]bool),
        completed:      false,
    }
    
    a.imageMap[imageID] = assembly
    
    log.Printf("Created assembly for image %d (%dx%d, %d tiles expected)",
        imageID, info.Width, info.Height, info.ExpectedTiles)
    
    return assembly, nil
}

func (a *Assembler) saveImage(assembly *ImageAssembly) error {
    file, err := os.Create(assembly.info.OutputPath)
    if err != nil {
        return fmt.Errorf("failed to create output file: %w", err)
    }
    defer file.Close()
    
    if err := png.Encode(file, assembly.outputImage); err != nil {
        return fmt.Errorf("failed to encode PNG: %w", err)
    }
    
    return nil
}

func (a *Assembler) checkpointMonitor(wg *sync.WaitGroup) {
    defer wg.Done()
    
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-a.ctx.Done():
            return
        case <-ticker.C:
            a.mutex.RLock()
            activeImages := len(a.imageMap)
            var incompleteCount int
            for _, assembly := range a.imageMap {
                if !assembly.completed {
                    incompleteCount++
                    log.Printf("Image %d progress: %d/%d tiles received",
                        assembly.info.ID, assembly.tilesReceived, assembly.info.ExpectedTiles)
                }
            }
            a.mutex.RUnlock()
            
            if activeImages > 0 {
                log.Printf("Assembler status: %d active images, %d incomplete", 
                    activeImages, incompleteCount)
            }
        }
    }
}