package main

import (
    "flag"
    "fmt"
    "image"
    "image/png"
    "log"
    "os"
    "path/filepath"
    "time"

    "studyguide.parallel/pkg/common"
    ftqqueue "go-blur-ftq/pkg/queue"
)

type ImageAssembler struct {
    info   *common.ImageInfo
    img    *image.RGBA
}

func main() {
    var (
        redisAddr = flag.String("redis", "redis:6379", "Redis address")
        timeout   = flag.Duration("timeout", 5*time.Second, "Stream read block timeout")
    )
    flag.Parse()

    rs, err := ftqqueue.NewRedisStreams(*redisAddr)
    if err != nil { log.Fatalf("redis: %v", err) }
    defer rs.Close()
    if err := rs.EnsureGroups(); err != nil { log.Printf("ensure groups: %v", err) }

    log.Printf("Assembler ready - waiting for results on fixed streams...")

    assemblers := map[int]*ImageAssembler{}
    consumer := "assembler"

    for {
        id, res, err := rs.ReadResult(consumer, *timeout)
        if err != nil { log.Printf("read result: %v", err); continue }
        if res == nil { continue }

        if res.ProcessedTile == nil {
            log.Printf("invalid result: nil ProcessedTile from worker %s; ack+skip", res.WorkerID)
            _ = rs.AckResult(id)
            continue
        }

        tile := res.ProcessedTile
        asm := assemblers[tile.ImageID]
        if asm == nil {
            info, err := rs.GetImageInfo(tile.ImageID)
            if err != nil { log.Printf("image info: %v", err); _ = rs.AckResult(id); continue }
            asm = &ImageAssembler{info: info, img: image.NewRGBA(image.Rect(0, 0, info.Width, info.Height))}
            assemblers[tile.ImageID] = asm
        }

        // Idempotency: mark received, skip duplicates
        added, err := rs.MarkTileReceived(tile.ImageID, tile.TileID)
        if err != nil { log.Printf("mark received: %v", err); continue }
        if added == 0 {
            // already processed; just ack
            _ = rs.AckResult(id)
            continue
        }

        // Persist each tile to disk (durable) before ack
        if err := persistTile("ftq", asm.info, tile); err != nil { log.Printf("persist: %v", err); continue }

        // Apply into image buffer
        for y := 0; y < tile.Height && y < len(tile.Data); y++ {
            for x := 0; x < tile.Width && x < len(tile.Data[y]); x++ {
                asm.img.SetRGBA(tile.X+x, tile.Y+y, tile.Data[y][x])
            }
        }

        // Ack only after durable write and in-memory apply
        if err := rs.AckResult(id); err != nil { log.Printf("ack result: %v", err) }

        // Check completion
        count, _ := rs.GetReceivedCount(tile.ImageID)
        if int(count) >= asm.info.ExpectedTiles {
            if err := saveImage(asm.img, asm.info.OutputPath); err != nil {
                log.Printf("save image: %v", err)
            } else {
                log.Printf("Saved image %d to %s", tile.ImageID+1, asm.info.OutputPath)
            }
            delete(assemblers, tile.ImageID)
        }
    }
}

func persistTile(runID string, info *common.ImageInfo, tile *common.ProcessedImageTile) error {
    base := filepath.Join("/data/run", runID, fmt.Sprintf("img_%d", info.ID), "tiles")
    if err := os.MkdirAll(base, 0755); err != nil { return err }
    // minimal durable marker per tile
    f := filepath.Join(base, fmt.Sprintf("tile_%d.ok", tile.TileID))
    return os.WriteFile(f, []byte("ok"), 0644)
}

func saveImage(img *image.RGBA, outputPath string) error {
    if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil { return err }
    f, err := os.Create(outputPath)
    if err != nil { return err }
    defer f.Close()
    return png.Encode(f, img)
}

