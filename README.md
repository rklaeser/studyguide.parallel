# studyguide.parallel
Implementing patterns from my C++ multi-threading course in Go

## Image Processing Approaches

### View the results of passed runs in `logs/`

### To run
`go run .`


This project demonstrates seven different approaches to processing images with Gaussian blur:

### a. Sequential Processing (~13 second average)
- **File**: `a_sequential.go`
- **Approach**: Processes images one at a time in sequence
- **Implementation**: Single-threaded, applies Gaussian blur to each image sequentially
- **Performance**: Slowest approach, baseline for comparison

![Sequential Processing](visualize/a_sequential.png)

### b. Parallel Tile Processing (~3.5 second average)
- **File**: `b_tile_parallel.go`
- **Approach**: Tile level parallelism
- **Implementation**: 
  - Divides each image into 256x256 tiles with padding for seamless blur
  - Uses 10 worker goroutines to process tiles concurrently
  - Processes images sequentially but uses parallelism within each image
- **Performance**: ~3x faster than sequential due to multi-threading

```
  Coordinator ──┐
                ├─→ tileQueue ──┬─→ Worker 1 ──┐
                │               ├─→ Worker 2 ──├─→ resultQueue ──→Assembler
                │               ├─→ Worker 3 ──┤
                │               └─→ ...     ──┘
                └─ (1 producer)    (10 consumers/producers)    (1 consumer)
```

![Parallel Tile Processing](visualize/b_tile_parallel.png)

### c. Parallel Tile and Image Processing (~2 seconds)
- **File**: `c_tile+image_parallel.go`
- **Approach**: Tile level + image level parallelism
- **Implementation**:
  - Image loading, tile processing, and image assembly run concurrently
  - Multiple images can be in different pipeline stages simultaneously
  - Uses the same tile-based approach with 10 workers as parallel version
  - Optimizes overall throughput by overlapping I/O and computation
- **Performance**: Fastest approach, ~5x faster than sequential


```
                                          ┌─→ Image 1
  PipelineReader ──┬─→ imageDataChannel ──┼─→ Image 2
  (Parallel Load)  │  (all images)        └─→ Image 3...
                   └─ (multiple producers)

                      ↓
```

```
  PipelineCoordinator ──┐
  (All images)          ├─→ tileQueue ──┬─→ Worker 1 ──┐
                        │               ├─→ Worker 2 ──├─→ resultQueue ──→ AssemblerManager
                        │               ├─→ Worker 3 ──┤                        │
                        │               └─→ ...     ──┘                         │
                 So if  └─ (1 producer)    (10 consumers/producers)             │
                                                                                │
                                                              ┌─────────────────┘
                                                              │
                                            ┌─→ Assembler 1 (Image 1) ──→ [Output 1]
                                            ├─→ Assembler 2 (Image 2) ──→ [Output 2]
                                            └─→ Assembler 3 (Image 3) ──→ [Output 3]
                                                (parallel assemblers)
```

  Key Differences from b_tile_parallel.go:

  1. Parallel Image Loading - Multiple images loaded concurrently (Performance Opportunity: We don't kick off processing until all images are loaded. Doing so would save more time.)
  2. Image-aware Tiles - Each tile carries ImageID for routing so that workers can process any tile that is in the Queue regardless of what Image it is from.
  3. Assembler Manager - Routes tiles to correct assembler based on ImageID
  4. Multiple Assemblers - One per image, running in parallel, assemblers are created ahead of time and wait until AssemblerManager routes a processed tile from the resultQueue to them. Assembler processes image until their channels are closed.
  5. True Pipeline - All stages can run concurrently:
    - Loading image 2 while processing tiles from image 1
    - Assembling image 1 while blurring tiles from image 2


![Parallel Tile and Image Processing](visualize/c_tile+image_parallel.png)

Concurrent or Parallel?
  - In C++ threads will run in parallel until the computer's cores all become busy, at which point the processing will happen concurrently. Go routines are not operating system threads, they are managed by the go runtime so I would need to do some studying and testing to understand the optimal number of routines and number of tiles to divide images into. Either way, the performance gains show that we are finding some parallelism in this implementation.

## Distributed Processing Systems

### d. Distributed Sequential Processing
- **Directory**: `d/`
- **Approach**: Kubernetes-based sequential processing
- **Implementation**: Single containerized service that processes images sequentially in a distributed environment
- **Output**: Images to `d/output/`, timing results to `logs/d_*.txt`

**To run:**
```bash
cd d
make minikube-deploy    # Build and deploy to minikube
make logs              # Monitor processing
make status            # Check job status
make clean             # Clean up when done
```

**Prerequisites:** minikube, Docker, kubectl, and minikube mount running:
```bash
minikube mount ~/Code/studyguide.parallel:/mnt/image-data &
```

### e. Distributed Queue-Based Processing  
- **Directory**: `e/`
- **Approach**: Redis-based job queue with distributed workers
- **Implementation**: 
  - **Redis**: Job queue and result storage
  - **Coordinator**: Splits images into tiles, queues work
  - **Workers (4 replicas)**: Process tiles in parallel
  - **Assembler**: Reconstructs completed images
- **Output**: Images to `e/output/`, timing results to `logs/e_*.txt`

**To run:**
```bash
cd e
make minikube-deploy    # Build and deploy infrastructure
make run-coordinator    # Trigger image processing
make logs-coordinator   # Monitor image splitting
make logs-workers      # Monitor tile processing  
make logs-assembler    # Monitor image reconstruction
make status            # Check all services
make clean             # Clean up when done
```

**Architecture Flow:**
1. Coordinator splits images into tiles → Redis queue
2. Workers pull tiles from queue → process in parallel → return to Redis
3. Assembler monitors Redis → reconstructs completed images → saves to output

**Prerequisites:** Same as d


Discussion:

Expected: 35 tiles for img1
  - Received: 33 tiles (missing 2 tiles)
  - Result: Image not saved because it's incomplete

  This demonstrates a critical flaw in the e system:
  - Even without deliberate worker killing, the system can lose tiles due to race conditions, network issues, or timing
  problems
  - The assembler requires ALL tiles before saving, so any loss means no output
  - This happens in normal operation, not just during fault injection

  This makes the comparison even more compelling:
  - e: Loses tiles even in normal processing (img1 missing)
  - f: Should process all images reliably

### f. Fault-Tolerant Queue-Based Processing
- **Directory**: `f/`
- **Approach**: Enhanced version of e with fault tolerance mechanisms
- **Implementation**: 
  - **Redis with persistence**: Durable job queue and result storage
  - **Coordinator**: Splits images into tiles with retry logic
  - **Workers**: Process tiles with exponential backoff and health checks
  - **Assembler**: Fault-tolerant image reconstruction with partial recovery
- **Output**: Images to `f/output/`, timing results to `logs/f_*.txt`

**To run:**
```bash
cd f
make deploy            # Deploy fault-tolerant infrastructure
make run              # Process images with fault tolerance
make chaos-test       # Test fault injection scenarios
make clean            # Clean up resources
```

**Key Improvements over e:**
- Persistent Redis storage survives pod restarts
- Worker health monitoring and automatic restarts
- Retry mechanisms for failed tile processing
- Graceful handling of partial failures
- Comprehensive logging and metrics

### g. Multithreaded Fault-Tolerant Service
- **Directory**: `g/`
- **Approach**: Hybrid service combining multithreading with Redis-based fault tolerance
- **Implementation**:
  - **Thread Pool**: Multiple worker threads per service instance
  - **Redis Streams**: Persistent job queues with at-least-once delivery
  - **Checkpointing**: Redis Sets track processed tiles for crash recovery
  - **Component Modes**: Can run as coordinator, worker, assembler, or all-in-one
- **Output**: Images to `g/output/`, Redis checkpoints for fault tolerance

**To run:**
```bash
cd g
# Local development
go run cmd/service/main.go -mode=all -workers=10

# Kubernetes deployment
kubectl apply -f k8s/  # Deploy all components
./run.sh               # Start processing with monitoring
```

**Key Features:**
- **Multithreaded performance** with fault tolerance
- **Crash recovery** from Redis checkpoints
- **Horizontal scaling** across multiple instances
- **Idempotent processing** prevents duplicate work
- **Automatic retries** with exponential backoff

