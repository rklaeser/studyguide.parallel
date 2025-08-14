# g

A high-performance, fault-tolerant image processing microservice that combines multithreading with Redis-based fault tolerance for parallel Gaussian blur operations.

## Architecture Overview

This service implements a hybrid approach that combines the best aspects of parallel processing and fault-tolerant distributed systems:

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│  Coordinator │────▶│Redis Streams│◀────│   Workers    │
│              │     │   (Queue)   │     │  (Thread     │
│  - Partition │     │             │     │   Pool)      │
│  - Queue     │     └─────────────┘     │              │
└──────────────┘            │            └──────────────┘
                            │                    │
                            ▼                    ▼
                    ┌─────────────┐      ┌──────────────┐
                    │Redis Sets   │◀─────│  Assembler   │
                    │(Checkpoints)│      │              │
                    └─────────────┘      └──────────────┘
```

### Key Components

1. **Coordinator**: Partitions images into tiles and queues them in Redis
2. **Worker Pool**: Multithreaded processors that consume and blur tiles
3. **Assembler**: Reconstructs images from processed tiles with checkpointing
4. **Redis**: Provides persistent queues and checkpointing for fault tolerance

### Features

- **Multithreaded Processing**: Each service instance runs multiple worker threads
- **Fault Tolerance**: Redis Streams ensure at-least-once delivery with retries
- **Idempotent Assembly**: Redis Sets prevent duplicate tile processing
- **Horizontal Scaling**: Deploy multiple worker/assembler instances
- **Crash Recovery**: Checkpoints allow resuming from last known state
- **Automatic Retries**: Failed tiles are retried with exponential backoff

## Quick Start

### Local Development

```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Build and run
cd g
go mod download
go run cmd/service/main.go \
  -mode=all \
  -redis=localhost:6379 \
  -input=../data/input \
  -output=../data/output \
  -workers=10
```

### Kubernetes Deployment

```bash
# Build image (with minikube)
eval $(minikube docker-env)
docker build -f g/Dockerfile -t mt-service:latest .

# Deploy infrastructure
kubectl apply -f g/k8s/pv-pvc.yaml
kubectl apply -f g/k8s/redis-deployment.yaml

# Copy test images to shared volume
kubectl cp ../data/input <any-pod>:/data/input

# Deploy service components separately
kubectl apply -f g/k8s/worker-deployment.yaml
kubectl apply -f g/k8s/assembler-deployment.yaml
kubectl apply -f g/k8s/coordinator-job.yaml

# Or deploy all-in-one service
kubectl apply -f g/k8s/service-deployment.yaml
```

## Configuration

### Command Line Arguments

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | `all` | Service mode: `coordinator`, `worker`, `assembler`, or `all` |
| `-redis` | `localhost:6379` | Redis server address |
| `-input` | `/data/input` | Input directory for images |
| `-output` | `/data/output` | Output directory for processed images |
| `-workers` | `10` | Number of worker threads per instance |
| `-kernel` | `15` | Gaussian blur kernel size |
| `-run` | auto-generated | Run ID for namespacing |

### Deployment Modes

1. **All-in-One** (`-mode=all`): Single service runs all components
2. **Distributed**: Deploy coordinator, workers, and assembler separately
3. **Hybrid**: Multiple worker deployments with single coordinator/assembler

## Fault Tolerance Mechanisms

### 1. Job Queue Persistence
- Jobs stored in Redis Streams survive service crashes
- Consumer groups track message acknowledgment

### 2. Retry Logic
- Failed tiles automatically retried up to 3 times
- Exponential backoff prevents retry storms
- Stale job reclaim after 30-second timeout

### 3. Idempotent Processing
- Redis Sets track processed tiles
- Duplicate results are safely ignored
- Assembly can resume from any point

### 4. Checkpoint Recovery
```go
// Each tile completion is checkpointed
redisClient.MarkTileProcessed(imageID, tileID)

// Assembly checks existing progress on startup
processedCount := redisClient.GetProcessedCount(imageID)
```

## Performance Characteristics

### Advantages vs Pure Parallel (b/c implementations)
- Fault tolerance without full restart
- Can scale horizontally across machines
- Survives worker crashes

### Advantages vs Pure Distributed (e/f implementations)
- Lower latency with in-process threading
- Reduced network overhead
- Simpler deployment for small scales

### Trade-offs
- Memory pressure from concurrent image processing
- Thread synchronization complexity
- Redis becomes single point of failure

## Monitoring

Monitor service health through logs:
```bash
kubectl logs -l app=mt-worker -f
kubectl logs -l app=mt-assembler -f
```

Key metrics to watch:
- Tiles processed per second
- Retry rates
- Assembly completion times
- Redis memory usage

## Testing Fault Tolerance

```bash
# Start processing
kubectl apply -f g/k8s/coordinator-job.yaml

# Kill a worker mid-processing
kubectl delete pod <worker-pod>

# Observe retry and recovery in logs
kubectl logs -l app=mt-worker -f
```

## Comparison with Other Implementations

| Feature | g_multithreaded | b_tile_parallel | f_fault_tolerant |
|---------|-----------------|-----------------|------------------|
| Parallelism | Thread pool | Thread pool | Distributed |
| Fault Tolerance | Yes (Redis) | No | Yes (Redis) |
| Horizontal Scale | Yes | No | Yes |
| Memory Efficiency | Medium | High | Low |
| Deployment Complexity | Medium | Low | High |
| Best For | Medium-scale resilient processing | Single-machine performance | Large-scale distributed |

## Architecture Decision Record

### Why This Hybrid Approach?

1. **Multithreading for Performance**: Local threads eliminate network overhead for tile processing
2. **Redis for Reliability**: Persistent queues survive crashes without losing work
3. **Microservice for Flexibility**: Can scale components independently
4. **Checkpointing for Efficiency**: Resume from failure point rather than restart

### When to Use This Pattern

✅ **Good fit for:**
- Medium-scale image processing (10-1000 images)
- Requirements for fault tolerance
- Variable workloads needing elastic scaling
- Environments where Redis is already available

❌ **Consider alternatives for:**
- Very large images (>10GB) - memory constraints
- Real-time processing - checkpoint overhead
- Simple batch jobs - unnecessary complexity
- Extremely high throughput - Redis bottleneck

## Future Enhancements

1. **Adaptive Threading**: Adjust worker count based on load
2. **Priority Queues**: Process urgent images first
3. **Progress API**: REST endpoint for job status
4. **Metrics Export**: Prometheus/Grafana integration
5. **S3 Integration**: Direct cloud storage support
6. **GPU Acceleration**: CUDA kernels for blur operation