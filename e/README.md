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

## Happy Path Demo (baseline)

This shows the naive queue working under normal conditions.

Prereqs (first-time only):
- Apply the storage used by the deployments (PVC v2):
```bash
kubectl apply -f k8s/pv-pvc-v2.yaml
```

Run the baseline flow:
```bash
# Build in minikube and deploy infra
make minikube-deploy

# Trigger processing with default config (kernel=15, INPUT_PATH=/data/input)
make run-coordinator

# Watch logs
make logs-assembler
```
Expected:
- All tiles for each image are assembled; output PNGs appear in `data/e_output/`.
- Assembler logs reach `Progress: n/n` for each image and print "Saved image ...".

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

## Fault Demo: Prove Tile Loss With Naive Queue

This deterministically demonstrates in-flight tile loss when workers are killed mid-processing (due to BRPOP without ack/visibility timeout).

One-time setup (ensure PVC v2 exists and services are running):
```bash
kubectl apply -f k8s/pv-pvc-v2.yaml
make deploy-config
make deploy-redis
kubectl apply -f k8s/worker-deployment.yaml
kubectl apply -f k8s/assembler-deployment.yaml
kubectl wait --for=condition=available --timeout=90s deployment/image-worker
kubectl wait --for=condition=available --timeout=60s deployment/image-assembler
```

Run the deterministic loss demo using only `img2.png` (largest image):
```bash
cd e
# Optional: remove any prior output to avoid confusion in the summary
ASM_POD=$(kubectl get pods -l app=image-assembler -o jsonpath='{.items[0].metadata.name}')
kubectl exec "$ASM_POD" -- rm -f /data/e_output/img2_blurred.png || true

bash chaos/demo-inflight-tile-loss.sh
```

What the script does:
- Sets slow processing (`KERNEL_SIZE=51`) and restricts input to a single image (`/data/input-single/img2.png`).
- Starts the coordinator and waits for `image:job:queue` length to drop below `expectedTiles` (proving some tiles are in-flight).
- Immediately force-deletes all workers to drop those in-flight tiles.
- Waits for the queues to drain, then prints a final summary including `missing` tiles.

Interpreting results:
- If you see `missing > 0`, the system lost tiles. The assembler will not reach `n/n` and may not save the output image.
- This compares against the happy path where the assembler reaches `n/n` and saves all outputs.

Reset to defaults after the demo:
```bash
# Restore default config (kernel=15, INPUT_PATH=/data/input) and restart workers
kubectl apply -f k8s/configmap.yaml
kubectl delete pods -l app=image-worker
kubectl delete pods -l app=image-assembler
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