#!/bin/bash

set -euo pipefail

echo "=== Starting Image Processing Job ==="

# Clean up previous outputs
echo "Clearing previous outputs..."
kubectl exec deploy/image-assembler -- sh -c 'rm -f /e/output/*' 2>/dev/null || true
rm -rf ./output 2>/dev/null || true
mkdir -p ./output

# Reset ConfigMap to process all images
echo "Resetting ConfigMap to process all images..."
kubectl create configmap image-processor-config --dry-run=client -o yaml \
    --from-literal=INPUT_PATH=/input \
    --from-literal=OUTPUT_PATH=/e/output \
    --from-literal=KERNEL_SIZE=15 \
    --from-literal=REDIS_ADDR=redis:6379 \
    --from-literal=NUM_WORKERS=4 | kubectl apply -f -

# Restart workers to pick up new config
echo "Restarting workers to pick up new config..."
kubectl rollout restart deployment image-worker >/dev/null
kubectl wait --for=condition=available --timeout=60s deployment/image-worker >/dev/null

# Start coordinator job
kubectl delete job image-coordinator --ignore-not-found=true
kubectl apply -f k8s/coordinator-job.yaml

echo "Waiting for coordinator to enqueue tiles..."
sleep 5

# Monitor processing progress
echo "Monitoring processing progress..."
TIMEOUT=120
for i in $(seq 1 $TIMEOUT); do
    JOB_Q=$(kubectl exec deploy/redis -- redis-cli llen image:job:queue 2>/dev/null | tr -d '\r' || echo "0")
    RESULT_Q=$(kubectl exec deploy/redis -- redis-cli llen image:result:queue 2>/dev/null | tr -d '\r' || echo "0")
    echo "[$i s] Jobs in queue: $JOB_Q, Results waiting: $RESULT_Q"
    
    if [ "$JOB_Q" = "0" ] && [ "$RESULT_Q" = "0" ]; then
        echo "✅ Processing complete! Copying images to ./output/"
        
        # Copy processed images
        POD=$(kubectl get pods -l app=image-assembler -o jsonpath='{.items[0].metadata.name}')
        kubectl exec "$POD" -- sh -c 'ls /e/output/*.png' 2>/dev/null | while read img; do
            filename=$(basename "$img")
            echo "Copying $filename..."
            kubectl cp "$POD:$img" "./output/$filename" 2>/dev/null || true
        done
        
        echo "Images available in ./output/ directory:"
        ls -la ./output/ 2>/dev/null || echo "No images copied"
        break
    fi
    
    sleep 2
done

if [ $i -eq $TIMEOUT ]; then
    echo "⚠️ Processing timed out after ${TIMEOUT}s"
    exit 1
fi