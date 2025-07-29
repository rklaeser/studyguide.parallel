# Distributed Image Processing System

A Kubernetes-based distributed image processing system that applies Gaussian blur to images.

## Architecture

- **Image Processor Service**: Go application that reads images, applies Gaussian blur, and saves results
- **Kubernetes Job**: Runs the processor as a batch job
- **Persistent Storage**: Uses PV/PVC for shared storage between pods
- **ConfigMap**: Stores configuration parameters

## Prerequisites

- Kubernetes cluster (or Minikube/k3s)
- Docker
- kubectl configured
- Storage provisioner (for PV)

## Setup

### 1. Build the Docker Image

```bash
make build
```

### 2. Push to Registry (if using remote cluster)

```bash
make push
```

### 3. Prepare Storage on Proxmox Host

Create the directory that will be mounted:

```bash
# On your Proxmox host
mkdir -p /mnt/image-data/input
mkdir -p /mnt/image-data/output
chmod -R 777 /mnt/image-data
```

### 4. Deploy to Kubernetes

```bash
# Deploy storage and config
make deploy

# Or step by step:
kubectl apply -f k8s/pv-pvc.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/job.yaml
```

## Usage

### Option 1: Process All Images in Directory

```bash
# Copy images to input directory on host
cp *.jpg /mnt/image-data/input/

# Run the job
kubectl create -f k8s/job.yaml

# Check results in output directory
ls /mnt/image-data/output/
```

### Option 2: Use Trigger Script

```bash
# Process all images with default settings
./scripts/trigger-job.sh

# Process with custom kernel size
./scripts/trigger-job.sh -k 25

# Process specific file
./scripts/trigger-job.sh -f image1.jpg -k 15

# Show help
./scripts/trigger-job.sh -h
```

### Monitoring

```bash
# Check job status
make status

# View logs
make logs

# Clean up completed jobs
make clean
```

## Configuration

Edit `k8s/configmap.yaml` to change default settings:

- `KERNEL_SIZE`: Size of Gaussian kernel (default: 15)
- `INPUT_PATH`: Input directory path in container
- `OUTPUT_PATH`: Output directory path in container

## Storage Options

### HostPath (Default - Single Node)

Currently configured for testing on single-node clusters.

### NFS (Multi-Node)

For production clusters, uncomment and configure NFS in `k8s/pv-pvc.yaml`:

```yaml
nfs:
  server: nfs-server.example.com
  path: /exports/image-data
```

## Troubleshooting

1. **Permission Denied**: Ensure the host directory has proper permissions
2. **Image Not Found**: If using local image, set `imagePullPolicy: Never` in job.yaml
3. **PVC Pending**: Check if PV is available and matches PVC requirements

## Next Steps

- Add horizontal scaling with multiple parallel jobs
- Implement queue-based processing with message broker
- Add support for different image filters
- Create web UI for job submission