#!/bin/bash

set -euo pipefail

echo "=== Starting Tile Parallel Image Processing Job ==="

# Clean up previous outputs from shared storage
echo "Cleaning previous outputs from shared storage..."
kubectl run temp-cleanup-$$-$RANDOM --image=busybox --restart=Never --rm -i \
    --overrides='{
        "spec": {
            "containers": [{
                "name": "cleanup",
                "image": "busybox",
                "command": ["sh", "-c", "mkdir -p /data/b/output && rm -f /data/b/output/*"],
                "volumeMounts": [{"name": "shared-data", "mountPath": "/data"}]
            }],
            "volumes": [{"name": "shared-data", "persistentVolumeClaim": {"claimName": "shared-storage-pvc"}}]
        }
    }' 2>/dev/null || true

# Clean up previous job
echo "Cleaning up previous job..."
kubectl delete job tile-parallel-processor-job --ignore-not-found=true

# Wait for pods to be cleaned up
sleep 2

# Start new job
echo "Starting tile parallel processor job..."
kubectl apply -f k8s/job.yaml

# Wait for job to start
echo "Waiting for job to start..."
sleep 3

# Get the pod name
POD=$(kubectl get pods -l app=tile-parallel-processor --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}')

if [ -z "$POD" ]; then
    echo "Error: No pod found for tile-parallel-processor job"
    exit 1
fi

echo "Following logs from pod: $POD"
echo "---"

# Follow logs from the single pod
kubectl logs -f "$POD"

# Wait for job completion
echo "---"
echo "Waiting for job completion..."
kubectl wait --for=condition=complete --timeout=300s job/tile-parallel-processor-job || {
    echo "Job failed or timed out"
    kubectl describe job tile-parallel-processor-job
    exit 1
}

echo "✅ Job completed successfully!"

# Copy output files from shared storage to local directory
echo "Copying output files to local directory..."
mkdir -p output

# Create a temporary pod to list and copy files
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
echo "Checking files in /data/b/output/..."
kubectl exec $TEMP_POD -- ls /data/b/output/ 2>/dev/null | grep "\.png$" | while read filename; do
    echo "Copying $filename..."
    kubectl cp "$TEMP_POD:/data/b/output/$filename" "./output/$filename" 2>/dev/null
done

# Clean up temp pod
kubectl delete pod $TEMP_POD --force --grace-period=0 2>/dev/null

# List the local output files
echo "Output files in ./output/:"
ls -la output/ 2>/dev/null || echo "No output files found"