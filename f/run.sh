#!/bin/bash

set -euo pipefail

echo "=== Starting FTQ Image Processing Job ==="

# Clean up previous outputs
echo "Clearing previous outputs..."
kubectl exec deploy/ftq-assembler -- sh -c 'rm -f /data/f_output/*' 2>/dev/null || true
rm -rf ./output 2>/dev/null || true
mkdir -p ./output

# Clear Redis streams for clean run
kubectl exec deploy/redis -- redis-cli DEL ftq:jobs ftq:results ftq:dlq:jobs 2>/dev/null || true

# Start coordinator job
kubectl delete job ftq-coordinator --ignore-not-found=true
kubectl apply -f k8s/coordinator-job.yaml

echo "Waiting for coordinator to complete..."
kubectl wait --for=condition=complete --timeout=30s job/ftq-coordinator 2>/dev/null || true

# Get total jobs count
TOTAL=$(kubectl exec deploy/redis -- redis-cli XLEN "ftq:jobs" 2>/dev/null | tr -d '\r' || echo "0")
echo "Total tiles to process: $TOTAL"

echo "Monitoring processing progress..."
TIMEOUT=120
for i in $(seq 1 $TIMEOUT); do
    RESULTS=$(kubectl exec deploy/redis -- redis-cli XLEN "ftq:results" 2>/dev/null | tr -d '\r' || echo "0")
    PENDING=$(kubectl exec deploy/redis -- redis-cli XPENDING "ftq:jobs" workers 2>/dev/null | head -1 | awk '{print $1}' || echo "0")
    echo "[$i s] Completed: $RESULTS/$TOTAL, In-flight: $PENDING"
    
    if [ "$RESULTS" = "$TOTAL" ]; then
        echo "✅ Processing complete! All $TOTAL tiles processed."
        echo "Copying images to ./output/"
        
        # Copy processed images
        POD=$(kubectl get pods -l app=ftq-assembler -o jsonpath='{.items[0].metadata.name}')
        if [ -n "$POD" ]; then
            kubectl exec "$POD" -- sh -c 'ls /data/f_output/*.png 2>/dev/null' | while read img; do
                filename=$(basename "$img")
                echo "Copying $filename..."
                kubectl cp "$POD:$img" "./output/$filename" 2>/dev/null || true
            done
        fi
        
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