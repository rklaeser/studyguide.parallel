# E_Distributed_Queue - Distributed Tile-Based Image Processing

This system implements distributed tile-based image processing using Redis as a job queue and Kubernetes for orchestration. It's based on the architecture from c_tile+image_parallel.go but uses containers instead of threads.

## Architecture

### Components

1. **Redis** - Message queue for job distribution
   - Stores tile jobs and processed results
   - Tracks processing progress

2. **Coordinator** - Image splitting service (Job)
   - Loads images from input directory
   - Splits images into tiles
   - Pushes tile jobs to Redis queue

3. **Workers** - Tile processing service (Deployment, 4 replicas)
   - Pull tile jobs from Redis
   - Apply Gaussian blur
   - Push results back to Redis

4. **Assembler** - Image reconstruction service (Deployment)
   - Monitors Redis for completed tiles
   - Reassembles tiles into complete images
   - Saves to output directory

## Quick Start

```bash
# 1. Start minikube and build images
make minikube-deploy

# 2. Wait for services to be ready
make status

# 3. Process images
make run-coordinator

# 4. Monitor progress
make logs-workers     # In one terminal
make logs-assembler   # In another terminal
make logs-coordinator # In a third terminal
```

## Workflow

1. Place images in `~/Code/studyguide.parallel/data/input/`
2. Run the coordinator to split images into tiles
3. Workers process tiles in parallel
4. Assembler reconstructs and saves images to `data/e_output/`

## Scaling

```bash
# Scale workers up or down
make scale-workers REPLICAS=8
```

## Monitoring

```bash
# Check status
make status

# View logs
make logs-workers
make logs-assembler
make logs-coordinator
make logs-redis
```

## Clean Up

```bash
# Remove jobs and deployments
make clean

# Remove everything including storage
make clean-all
```

## Configuration

Edit `k8s/configmap.yaml` to change:
- `KERNEL_SIZE`: Gaussian blur kernel size (default: 15)
- `NUM_WORKERS`: Expected number of workers (default: 4)
- `INPUT_PATH`: Input directory path
- `OUTPUT_PATH`: Output directory path