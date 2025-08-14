package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "path/filepath"
    "strings"
    "sync"
    "syscall"
    "time"

    "go-blur-mt/pkg/assembler"
    "go-blur-mt/pkg/coordinator"
    "go-blur-mt/pkg/processor"
    "go-blur-mt/pkg/queue"
)

func main() {
    var (
        redisAddr    = flag.String("redis", "localhost:6379", "Redis address")
        inputDir     = flag.String("input", "/data/input", "Input directory")
        outputDir    = flag.String("output", "/data/output", "Output directory")
        kernelSize   = flag.Int("kernel", 15, "Gaussian kernel size")
        numWorkers   = flag.Int("workers", 10, "Number of worker threads")
        mode         = flag.String("mode", "all", "Mode: coordinator, worker, assembler, or all")
    )
    flag.Parse()
    
    hostname, _ := os.Hostname()
    serviceID := fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
    
    log.Printf("Starting multithreaded blur service")
    log.Printf("Mode: %s, Service ID: %s", *mode, serviceID)
    log.Printf("Redis: %s, Workers: %d, Kernel: %d", *redisAddr, *numWorkers, *kernelSize)
    
    redisClient, err := queue.NewRedisClient(*redisAddr)
    if err != nil {
        log.Fatalf("Failed to connect to Redis: %v", err)
    }
    defer redisClient.Close()
    
    if err := redisClient.EnsureGroups(); err != nil {
        log.Printf("Failed to ensure Redis groups: %v", err)
    }
    
    if err := os.MkdirAll(*outputDir, 0755); err != nil {
        log.Fatalf("Failed to create output directory: %v", err)
    }
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    var wg sync.WaitGroup
    
    switch *mode {
    case "coordinator":
        runCoordinator(redisClient, *inputDir, *outputDir, *kernelSize)
        
    case "worker":
        workerPool := processor.NewWorkerPool(redisClient, *numWorkers, *kernelSize, serviceID)
        
        wg.Add(1)
        go func() {
            defer wg.Done()
            workerPool.Start()
        }()
        
        <-sigChan
        workerPool.Stop()
        
    case "assembler":
        imageAssembler := assembler.NewAssembler(redisClient, serviceID)
        
        wg.Add(1)
        go func() {
            defer wg.Done()
            imageAssembler.Start()
        }()
        
        <-sigChan
        imageAssembler.Stop()
        
    case "all":
        imagePaths := findImages(*inputDir)
        if len(imagePaths) == 0 {
            log.Fatalf("No images found in %s", *inputDir)
        }
        
        log.Printf("Found %d images to process", len(imagePaths))
        
        wg.Add(1)
        go func() {
            defer wg.Done()
            time.Sleep(2 * time.Second)
            runCoordinator(redisClient, *inputDir, *outputDir, *kernelSize)
        }()
        
        workerPool := processor.NewWorkerPool(redisClient, *numWorkers, *kernelSize, serviceID)
        wg.Add(1)
        go func() {
            defer wg.Done()
            workerPool.Start()
        }()
        
        imageAssembler := assembler.NewAssembler(redisClient, serviceID)
        wg.Add(1)
        go func() {
            defer wg.Done()
            imageAssembler.Start()
        }()
        
        <-sigChan
        log.Println("Shutting down all components...")
        workerPool.Stop()
        imageAssembler.Stop()
        
    default:
        log.Fatalf("Invalid mode: %s. Use coordinator, worker, assembler, or all", *mode)
    }
    
    wg.Wait()
    log.Println("Service shutdown complete")
}

func runCoordinator(redisClient *queue.RedisClient, inputDir, outputDir string, kernelSize int) {
    imagePaths := findImages(inputDir)
    if len(imagePaths) == 0 {
        log.Printf("No images found in %s", inputDir)
        return
    }
    
    log.Printf("Coordinator: Processing %d images", len(imagePaths))
    
    coord := coordinator.NewCoordinator(redisClient, kernelSize)
    
    startTime := time.Now()
    if err := coord.ProcessImages(imagePaths, outputDir); err != nil {
        log.Printf("Coordinator failed: %v", err)
    } else {
        duration := time.Since(startTime).Seconds()
        log.Printf("Coordinator: All images queued in %.2fs", duration)
    }
}

func findImages(dir string) []string {
    var images []string
    
    patterns := []string{"*.png", "*.jpg", "*.jpeg"}
    for _, pattern := range patterns {
        matches, err := filepath.Glob(filepath.Join(dir, pattern))
        if err == nil {
            images = append(images, matches...)
        }
    }
    
    for i, path := range images {
        if strings.Contains(path, "blurred") {
            images = append(images[:i], images[i+1:]...)
        }
    }
    
    return images
}