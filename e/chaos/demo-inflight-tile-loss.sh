#!/bin/bash

set -euo pipefail

echo "=== Demonstrate In-Flight Tile Loss (single image: img2.png) ==="

# 0) Clean up previous outputs
echo "0) Clearing previous outputs..."
kubectl exec deploy/image-assembler -- sh -c 'rm -f /e/output/*' 2>/dev/null || true
rm -rf ./output-fault 2>/dev/null || true
mkdir -p ./output-fault

# 1) Preconditions
echo "This script assumes Chaos Mesh is installed and Redis/worker/assembler deployments exist."

# 1) Prepare single-image input path and slow processing
echo "1) Ensuring slow processing and single-image input..."

# Create a single-image directory inside the PVC by copying via assembler pod
ASM_POD=$(kubectl get pods -l app=image-assembler -o jsonpath='{.items[0].metadata.name}')
if [ -z "${ASM_POD}" ]; then
  echo "Assembler pod not found; ensure image-assembler deployment is running."
  exit 1
fi
kubectl exec "${ASM_POD}" -- mkdir -p /input-single
kubectl cp "$HOME/Code/studyguide.parallel/input/img2.png" "${ASM_POD}:/input-single/img2.png"

echo "Copied img2.png into PVC at /input-single via pod ${ASM_POD}."

# 2) Patch runtime config to use single-image input and keep same output path
echo "2) Patching runtime configmap to point INPUT_PATH to /input-single and slow kernel..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: image-processor-config
  namespace: default
data:
  KERNEL_SIZE: "51"
  INPUT_PATH: "/input-single"
  OUTPUT_PATH: "/e/output"
  REDIS_ADDR: "redis:6379"
  NUM_WORKERS: "4"
EOF

# 3) Restart workers so they pick up new config
echo "3) Restarting workers to pick up config..."
kubectl delete pods -l app=image-worker --ignore-not-found
kubectl rollout restart deployment image-worker
kubectl wait --for=condition=available --timeout=90s deployment/image-worker

# 4) Clear queues
echo "4) Clearing Redis queues (job and result) and progress..."
kubectl exec deploy/redis -- sh -c 'redis-cli del image:job:queue image:result:queue $(redis-cli keys "image:progress:*") >/dev/null 2>&1 || true'

# 5) Start coordinator
echo "5) Starting coordinator job..."
kubectl delete job image-coordinator 2>/dev/null || true
kubectl apply -f k8s/coordinator-job.yaml

echo "Waiting 4s for jobs to be enqueued..."
sleep 4

# 6) Determine expected tiles for image 0
EXPECTED=$(kubectl exec deploy/redis -- sh -c "redis-cli get 'image:info:0'" | sed 's/\\\\//g' | grep -o '"expected_tiles":[0-9]*' | cut -d: -f2 || true)
if [ -z "${EXPECTED}" ]; then
  echo "Failed to read expected_tiles from Redis. Aborting."
  exit 1
fi
echo "Expected tiles: ${EXPECTED}"

# 7) Wait until the job queue length drops (indicating in-flight tiles)
echo "7) Watching job queue for drop to indicate in-flight tiles..."
while true; do
  QLEN=$(kubectl exec deploy/redis -- redis-cli llen image:job:queue | tr -d '\r')
  echo "Queue length: ${QLEN}"
  # If some workers have popped jobs, QLEN will be < EXPECTED
  if [ "${QLEN}" -lt "${EXPECTED}" ]; then
    echo "Queue dropped from ${EXPECTED} -> ${QLEN}. Killing workers NOW to drop in-flight tiles."
    break
  fi
  sleep 1
done

# 8) Kill all workers immediately
kubectl delete pods -l app=image-worker --grace-period=0 --force

# 9) Let workers come back and drain remaining jobs
echo "9) Waiting for workers to restart..."
kubectl rollout status deployment/image-worker

# 10) Wait for queues to drain or time out (to rule out 'still processing')
echo "10) Waiting for queues to drain (up to 120s)..."
TIMEOUT=120
for ((i=1; i<=TIMEOUT; i++)); do
  QLEN=$(kubectl exec deploy/redis -- redis-cli llen image:job:queue | tr -d '\r')
  RLEN=$(kubectl exec deploy/redis -- redis-cli llen image:result:queue | tr -d '\r')
  echo "[${i}s] job_queue=${QLEN} result_queue=${RLEN}"
  if [ "${QLEN}" = "0" ] && [ "${RLEN}" = "0" ]; then
    echo "Queues drained."
    break
  fi
  sleep 1
done

# 11) Final state summary and missing tile detection
echo "\n11) Final state summary:"
QLEN=$(kubectl exec deploy/redis -- redis-cli llen image:job:queue | tr -d '\r')
RLEN=$(kubectl exec deploy/redis -- redis-cli llen image:result:queue | tr -d '\r')

# Progress uses image 0 for this single-image run
PROGRESS=$(kubectl exec deploy/redis -- sh -c "redis-cli get 'image:progress:0'" | tr -d '\r') || true
if [ -z "${PROGRESS}" ] || [ "${PROGRESS}" = "(nil)" ]; then PROGRESS=0; fi

MISSING=$((EXPECTED - PROGRESS - QLEN - RLEN))
echo "expected=${EXPECTED} processed=${PROGRESS} jobs_in_queue=${QLEN} results_in_queue=${RLEN} missing=${MISSING}"

# Check whether the output file exists inside PVC
OUT_PRESENT=$(kubectl exec "${ASM_POD}" -- sh -c 'ls /e/output 2>/dev/null | grep -c "img2_blurred.png"' || true)
if [ "${OUT_PRESENT}" = "0" ]; then
  echo "output_file=absent (/e/output/img2_blurred.png)"
else
  echo "output_file=present (/e/output/img2_blurred.png)"
fi

echo "\nAssembler recent logs:"
kubectl logs -l app=image-assembler --tail=30

if [ "${MISSING}" -gt 0 ]; then
  echo "\nRESULT: Detected tile loss. ${MISSING} tiles missing after queues drained."
  
  # Copy processed images even with tile loss (if any exist)
  echo "\nChecking for processed images to copy..."
  POD=$(kubectl get pods -l app=image-assembler -o jsonpath='{.items[0].metadata.name}')
  IMAGE_COUNT=$(kubectl exec "$POD" -- sh -c 'ls /e/output/*.png 2>/dev/null | wc -l' || echo "0")
  
  if [ "$IMAGE_COUNT" -gt 0 ]; then
      echo "Copying $IMAGE_COUNT processed images to ./output-fault/"
      kubectl exec "$POD" -- sh -c 'ls /e/output/*.png' 2>/dev/null | while read img; do
          filename=$(basename "$img")
          echo "Copying $filename..."
          kubectl cp "$POD:$img" "./output-fault/$filename" 2>/dev/null || true
      done
  else
      echo "No images to copy - assembler didn't save incomplete images"
      echo "This demonstrates tile loss: missing tiles prevent image completion"
  fi
  
  echo "Images available in ./output-fault/ directory:"
  ls -la ./output-fault/ 2>/dev/null || echo "Directory empty (expected with tile loss)"
  exit 1
fi

if [ "${OUT_PRESENT}" = "0" ]; then
  echo "\nRESULT: Output image not saved. System stalled before completion."
  
  # Copy any partial images
  echo "\nCopying any partial images to ./output-fault/"
  POD=$(kubectl get pods -l app=image-assembler -o jsonpath='{.items[0].metadata.name}')
  kubectl exec "$POD" -- sh -c 'ls /e/output/*.png' 2>/dev/null | while read img; do
      filename=$(basename "$img")
      echo "Copying $filename..."
      kubectl cp "$POD:$img" "./output-fault/$filename" 2>/dev/null || true
  done
  
  echo "Images available in ./output-fault/ directory:"
  ls -la ./output-fault/ 2>/dev/null || echo "No images copied"
  exit 1
fi

echo "\nRESULT: No missing tiles detected."

# Copy processed images to local output-fault directory
echo "\nCopying processed images to ./output-fault/"
POD=$(kubectl get pods -l app=image-assembler -o jsonpath='{.items[0].metadata.name}')
kubectl exec "$POD" -- sh -c 'ls /e/output/*.png' 2>/dev/null | while read img; do
    filename=$(basename "$img")
    echo "Copying $filename..."
    kubectl cp "$POD:$img" "./output-fault/$filename" 2>/dev/null || true
done

echo "Images available in ./output-fault/ directory:"
ls -la ./output-fault/ 2>/dev/null || echo "No images copied"
