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

    "studyguide.parallel/pkg/common"
    ftqqueue "go-blur-ftq/pkg/queue"
)

func main() {
    var (
        inputPath  = flag.String("input", "/data/input", "Input directory path")
        outputPath = flag.String("output", "/data/e_output", "Output directory path")
        kernelSize = flag.Int("kernel", 15, "Gaussian kernel size")
        redisAddr  = flag.String("redis", "redis:6379", "Redis server address")
    )
    flag.Parse()

    log.Printf("FTQ Coordinator starting...")

    rs, err := ftqqueue.NewRedisStreams(*redisAddr)
    if err != nil { log.Fatalf("redis: %v", err) }
    defer rs.Close()
    if err := rs.EnsureGroups(); err != nil { log.Printf("ensure groups: %v", err) }

    paths, err := getImagePaths(*inputPath)
    if err != nil { log.Fatalf("images: %v", err) }
    if len(paths) == 0 { log.Printf("no images found"); return }

    start := time.Now()
    timing := &common.TimingData{
        StartTime:      start,
        KernelSize:     *kernelSize,
        TotalImages:    len(paths),
        InputPaths:     make([]string, 0, len(paths)),
        OutputPaths:    make([]string, 0, len(paths)),
        ImageStartTimes: map[int]time.Time{},
        ImageEndTimes:   map[int]*time.Time{},
    }

    padding := *kernelSize / 2
    for imageID, p := range paths {
        img, err := loadImage(p)
        if err != nil { log.Printf("load %s: %v", p, err); continue }
        b := img.Bounds()
        tilesX := (b.Dx() + common.TILE_SIZE - 1) / common.TILE_SIZE
        tilesY := (b.Dy() + common.TILE_SIZE - 1) / common.TILE_SIZE
        expected := tilesX * tilesY

        base := filepath.Base(p)
        name := strings.TrimSuffix(base, filepath.Ext(base))
        out := filepath.Join(*outputPath, fmt.Sprintf("%s_blurred.png", name))

        timing.InputPaths = append(timing.InputPaths, p)
        timing.OutputPaths = append(timing.OutputPaths, out)
        timing.ImageStartTimes[imageID] = time.Now()

        info := &common.ImageInfo{ID: imageID, InputPath: p, OutputPath: out, Width: b.Dx(), Height: b.Dy(), ExpectedTiles: expected, LoadTime: time.Now(), StartTime: time.Now()}
        if err := rs.StoreImageInfo(info); err != nil { log.Printf("store image info: %v", err) }

        // enqueue tiles
        tileID := 0
        for y := b.Min.Y; y < b.Max.Y; y += common.TILE_SIZE {
            for x := b.Min.X; x < b.Max.X; x += common.TILE_SIZE {
                tw := min(common.TILE_SIZE, b.Max.X-x)
                th := min(common.TILE_SIZE, b.Max.Y-y)
                tile := extractTileWithPadding(img, imageID, tileID, x, y, tw, th, padding)
                job := &common.JobMessage{Type: "tile", ImageTile: tile}
                if _, err := rs.AddJob(job); err != nil { log.Printf("add job: %v", err) }
                tileID++
            }
        }
        log.Printf("Enqueued %d tiles for image %d", expected, imageID+1)
    }

    if err := rs.StoreTiming(timing); err != nil { log.Printf("store timing: %v", err) }
    log.Printf("Coordinator finished")
}

func getImagePaths(dir string) ([]string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil { return nil, err }
    var paths []string
    for _, e := range entries {
        if e.IsDir() { continue }
        ext := strings.ToLower(filepath.Ext(e.Name()))
        if ext == ".jpg" || ext == ".jpeg" || ext == ".png" { paths = append(paths, filepath.Join(dir, e.Name())) }
    }
    return paths, nil
}

func loadImage(path string) (*image.RGBA, error) {
    f, err := os.Open(path)
    if err != nil { return nil, err }
    defer f.Close()
    im, _, err := image.Decode(f)
    if err != nil { return nil, err }
    b := im.Bounds()
    rgba := image.NewRGBA(b)
    for y := b.Min.Y; y < b.Max.Y; y++ {
        for x := b.Min.X; x < b.Max.X; x++ { rgba.Set(x, y, im.At(x, y)) }
    }
    return rgba, nil
}

func extractTileWithPadding(img *image.RGBA, imageID, tileID, tileX, tileY, tileW, tileH, padding int) *common.ImageTile {
    b := img.Bounds()
    sx := max(tileX-padding, b.Min.X)
    sy := max(tileY-padding, b.Min.Y)
    ex := min(tileX+tileW+padding, b.Max.X)
    ey := min(tileY+tileH+padding, b.Max.Y)
    pw := ex - sx
    ph := ey - sy
    pdata := make([][]color.RGBA, ph)
    for y := 0; y < ph; y++ {
        row := make([]color.RGBA, pw)
        for x := 0; x < pw; x++ { row[x] = img.RGBAAt(sx+x, sy+y) }
        pdata[y] = row
    }
    return &common.ImageTile{ImageID: imageID, TileID: tileID, X: tileX, Y: tileY, Width: tileW, Height: tileH, Data: pdata, Padding: padding}
}


func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }

