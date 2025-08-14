#!/bin/bash

set -euo pipefail

echo "=== Starting Image Processing Job ==="

# Clean up previous outputs
echo "Clearing previous outputs..."
kubectl exec deploy/image-assembler -- sh -c 'mkdir -p /data/e/output && rm -f /data/e/output/*' 2>/dev/null || true
rm -rf ./output 2>/dev/null || true
mkdir -p ./output

# Reset ConfigMap to process all images
echo "Resetting ConfigMap to process all images..."
kubectl create configmap image-processor-config --dry-run=client -o yaml \
    --from-literal=INPUT_PATH=/data/input \
    --from-literal=OUTPUT_PATH=/data/e/output \
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

# Check coordinator logs to see what happened
echo "Checking coordinator status..."
kubectl logs job/image-coordinator 2>/dev/null || echo "Coordinator not ready yet"

# Also check if workers and assembler are ready
echo "Checking worker status..."
kubectl get pods -l app=image-worker --no-headers | wc -l
echo "Checking assembler status..."  
kubectl get pods -l app=image-assembler --no-headers | wc -l

# Monitor processing progress
echo "Monitoring processing progress..."
TIMEOUT=120
for i in $(seq 1 $TIMEOUT); do
    JOB_Q=$(kubectl exec deploy/redis -- redis-cli llen image:job:queue 2>/dev/null | tr -d '\r' || echo "0")
    RESULT_Q=$(kubectl exec deploy/redis -- redis-cli llen image:result:queue 2>/dev/null | tr -d '\r' || echo "0")
    echo "[$i s] Jobs in queue: $JOB_Q, Results waiting: $RESULT_Q"
    
    if [ "$JOB_Q" = "0" ] && [ "$RESULT_Q" = "0" ]; then
        echo "✅ Processing complete!"
        
        # Debug: Check coordinator logs
        echo "Checking coordinator logs for output paths..."
        kubectl logs job/image-coordinator --tail=20 | grep -i "output\|saved" || true
        
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
        
        # Debug: Check if directory exists and list contents
        echo "Checking if /data/e/output/ exists..."
        kubectl exec $TEMP_POD -- ls -la /data/e/ 2>/dev/null || echo "Directory /data/e/ not found"
        
        # List files and copy each one
        echo "Checking files in /data/e/output/..."
        kubectl exec $TEMP_POD -- ls /data/e/output/ 2>/dev/null | grep "\.png$" | while read filename; do
            echo "Copying $filename..."
            kubectl cp "$TEMP_POD:/data/e/output/$filename" "./output/$filename" 2>/dev/null
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