# f

Redis Streams + consumer groups based implementation that prevents tile loss, supports retries, and enables crash-safe assembly.

Key features:
- Per-run namespacing (`run:<id>`)
- Reliable delivery with XREADGROUP/XACK and visibility timeout reclaims
- Idempotent assembly via Redis Set of received tileIds
- Ack-after-durable-write policy in assembler

Quick start (minikube assumed):
```bash
# Build images inside minikube
cd f
eval $(minikube docker-env)
docker build -f Dockerfile.coordinator -t ftq-coordinator:latest ..
docker build -f Dockerfile.worker -t ftq-worker:latest ..
docker build -f Dockerfile.assembler -t ftq-assembler:latest ..

# Apply shared storage and redis (use the same as e or an HA setup)
kubectl apply -f ../e/k8s/pv-pvc-v2.yaml
kubectl apply -f ../e/k8s/redis-deployment.yaml

# Deploy FTQ worker/assembler manifests (not included here yet)
```

Coordinator runs as a Job; workers and assembler as Deployments. See code in `cmd/*`.

