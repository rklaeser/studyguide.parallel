#!/bin/bash

set -euo pipefail

echo "=== Starting FTQ Image Processing Job ==="

# Clean up previous outputs
echo "Clearing previous outputs..."
kubectl exec deploy/ftq-assembler -- sh -c 'mkdir -p /data/f/output && rm -f /data/f/output/*' 2>/dev/null || true
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
        
        # Copy processed images from shared storage
        TEMP_POD="output-copy-$$-$RANDOM"
        kubectl run $TEMP_POD --image=busybox --restart=Never \
            --overrides='{
                "spec": {
                    "containers": [{
                        "name": "copy",
                        "image": "busybox",
                        "command": ["sleep", "300"],
                        "volumeMounts": [{"name": "shared-data", "mountPath": "/data"}]
                    }],
                    "volumes": [{"name": "shared-data", "persistentVolumeClaim": {"claimName": "shared-storage-pvc"}}]
                }
            }' 2>/dev/null
        
        # Wait for pod to be ready
        sleep 3
        
        # List files and copy each one
        echo "Checking files in /data/f/output/..."
        kubectl exec $TEMP_POD -- ls /data/f/output/ 2>/dev/null | grep "\.png$" | while read filename; do
            echo "Copying $filename..."
            kubectl cp "$TEMP_POD:/data/f/output/$filename" "./output/$filename" 2>/dev/null
        done
        
        # Clean up temp pod
        kubectl delete pod $TEMP_POD --force --grace-period=0 2>/dev/null
        
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