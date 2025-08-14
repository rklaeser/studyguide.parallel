#!/bin/bash

set -euo pipefail

echo "=== FTQ Fault Tolerance Demo ==="

# Clean up previous outputs
echo "Clearing previous outputs..."
kubectl exec deploy/ftq-assembler -- sh -c 'rm -f /data/f_output/*' 2>/dev/null || true
rm -rf ./output-fault 2>/dev/null || true
mkdir -p ./output-fault

# Start coordinator and kill workers mid-run
echo "Starting coordinator and killing workers mid-run..."
kubectl delete job ftq-coordinator --ignore-not-found=true
kubectl apply -f k8s/coordinator-job.yaml
sleep 5

# Get run information
RUN_ID=$(kubectl exec deploy/redis -- redis-cli GET runs:current 2>/dev/null | tr -d '\r')
TOTAL=$(kubectl exec deploy/redis -- redis-cli XLEN "run:$RUN_ID:jobs" 2>/dev/null | tr -d '\r' || echo "0")
echo "Run ID: $RUN_ID, Total tiles: $TOTAL"

# Wait for processing to start and kill workers
echo "Waiting for processing to start..."
for i in 1 2 3 4 5; do
    PENDING=$(kubectl exec deploy/redis -- redis-cli XPENDING "run:$RUN_ID:jobs" workers 2>/dev/null | head -1 | awk '{print $1}' || echo "0")
    RESULTS=$(kubectl exec deploy/redis -- redis-cli XLEN "run:$RUN_ID:results" 2>/dev/null | tr -d '\r' || echo "0")
    echo "[$i s] In-flight: $PENDING, Completed: $RESULTS/$TOTAL"
    
    if [ "$PENDING" -gt "0" ]; then
        echo ">>> KILLING ALL WORKERS NOW ($PENDING tiles in-flight) <<<"
        kubectl delete pods -l app=ftq-worker --grace-period=0 --force || true
        break
    fi
    sleep 1
done

# Wait for workers to restart
echo "Waiting for workers to restart..."
kubectl wait --for=condition=available --timeout=90s deployment/ftq-worker

# Monitor recovery
echo "Monitoring recovery..."
for i in $(seq 1 60); do
    RESULTS=$(kubectl exec deploy/redis -- redis-cli XLEN "run:$RUN_ID:results" 2>/dev/null | tr -d '\r' || echo "0")
    PENDING=$(kubectl exec deploy/redis -- redis-cli XPENDING "run:$RUN_ID:jobs" workers 2>/dev/null | head -1 | awk '{print $1}' || echo "0")
    echo "[Recovery $i s] Completed: $RESULTS/$TOTAL, In-flight: $PENDING"
    
    if [ "$RESULTS" = "$TOTAL" ]; then
        echo "✅ SUCCESS: All $TOTAL tiles recovered despite worker failures!"
        echo "Copying images to ./output-fault/"
        
        # Copy recovered images
        POD=$(kubectl get pods -l app=ftq-assembler -o jsonpath='{.items[0].metadata.name}')
        kubectl exec "$POD" -- sh -c 'ls /data/f_output/*.png' 2>/dev/null | while read img; do
            filename=$(basename "$img")
            echo "Copying $filename..."
            kubectl cp "$POD:$img" "./output-fault/$filename" 2>/dev/null || true
        done
        
        echo "Images available in ./output-fault/ directory:"
        ls -la ./output-fault/ 2>/dev/null || echo "No images copied"
        echo "🎉 FTQ demonstrated full fault tolerance - no tiles were lost!"
        exit 0
    fi
    
    sleep 2
done

echo "⚠️ Recovery timed out after 120s"
echo "Copying any partial images..."
POD=$(kubectl get pods -l app=ftq-assembler -o jsonpath='{.items[0].metadata.name}')
kubectl exec "$POD" -- sh -c 'ls /data/f_output/*.png' 2>/dev/null | while read img; do
    filename=$(basename "$img")
    echo "Copying $filename..."
    kubectl cp "$POD:$img" "./output-fault/$filename" 2>/dev/null || true
done

echo "Images available in ./output-fault/ directory:"
ls -la ./output-fault/ 2>/dev/null || echo "No images copied"
exit 1