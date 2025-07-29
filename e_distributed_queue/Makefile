.PHONY: build push deploy clean logs minikube-start minikube-build minikube-deploy

# Image names
COORDINATOR_IMAGE = image-coordinator
WORKER_IMAGE = image-worker
ASSEMBLER_IMAGE = image-assembler
IMAGE_TAG = latest

# Build all images
build:
	cd .. && docker build -f e_distributed_queue/Dockerfile.coordinator -t $(COORDINATOR_IMAGE):$(IMAGE_TAG) .
	cd .. && docker build -f e_distributed_queue/Dockerfile.worker -t $(WORKER_IMAGE):$(IMAGE_TAG) .
	cd .. && docker build -f e_distributed_queue/Dockerfile.assembler -t $(ASSEMBLER_IMAGE):$(IMAGE_TAG) .

# Minikube specific targets
minikube-start:
	minikube start
	@echo "Minikube started. Use 'eval \$$(minikube docker-env)' to configure Docker."

minikube-build: minikube-start
	@echo "Building images in minikube Docker environment..."
	cd .. && eval $$(minikube docker-env) && \
		docker build -f e_distributed_queue/Dockerfile.coordinator -t $(COORDINATOR_IMAGE):$(IMAGE_TAG) . && \
		docker build -f e_distributed_queue/Dockerfile.worker -t $(WORKER_IMAGE):$(IMAGE_TAG) . && \
		docker build -f e_distributed_queue/Dockerfile.assembler -t $(ASSEMBLER_IMAGE):$(IMAGE_TAG) .

deploy-storage:
	kubectl apply -f k8s/pv-pvc.yaml

deploy-config:
	kubectl apply -f k8s/configmap.yaml

deploy-redis:
	kubectl apply -f k8s/redis-deployment.yaml
	@echo "Waiting for Redis to be ready..."
	kubectl wait --for=condition=available --timeout=60s deployment/redis

deploy-services: deploy-redis
	kubectl apply -f k8s/worker-deployment.yaml
	kubectl apply -f k8s/assembler-deployment.yaml
	@echo "Waiting for services to be ready..."
	kubectl wait --for=condition=available --timeout=60s deployment/image-worker
	kubectl wait --for=condition=available --timeout=60s deployment/image-assembler

run-coordinator:
	kubectl apply -f k8s/coordinator-job.yaml

deploy: deploy-storage deploy-config deploy-services
	@echo "Infrastructure deployed. Run 'make run-coordinator' to process images."

minikube-deploy: minikube-build deploy

# Clean up resources
clean:
	kubectl delete job image-coordinator --ignore-not-found=true
	kubectl delete deployment image-worker --ignore-not-found=true
	kubectl delete deployment image-assembler --ignore-not-found=true
	kubectl delete deployment redis --ignore-not-found=true
	kubectl delete service redis --ignore-not-found=true
	kubectl delete configmap image-processor-config --ignore-not-found=true

clean-all: clean
	kubectl delete pvc image-processor-pvc --ignore-not-found=true
	kubectl delete pv image-processor-pv --ignore-not-found=true

# Monitoring
logs-coordinator:
	kubectl logs -l app=image-coordinator -f

logs-workers:
	kubectl logs -l app=image-worker -f

logs-assembler:
	kubectl logs -l app=image-assembler -f

logs-redis:
	kubectl logs -l app=redis -f

status:
	@echo "=== Deployments ==="
	kubectl get deployments
	@echo "\n=== Jobs ==="
	kubectl get jobs
	@echo "\n=== Pods ==="
	kubectl get pods
	@echo "\n=== Services ==="
	kubectl get services

# Scale workers
scale-workers:
	kubectl scale deployment image-worker --replicas=$(REPLICAS)

help:
	@echo "Available targets:"
	@echo "  build              - Build Docker images locally"
	@echo "  minikube-start     - Start minikube cluster"
	@echo "  minikube-build     - Build images in minikube Docker env"
	@echo "  minikube-deploy    - Full deployment to minikube"
	@echo "  deploy             - Deploy all services (except coordinator)"
	@echo "  run-coordinator    - Run the coordinator job to process images"
	@echo "  clean              - Remove deployments and jobs"
	@echo "  clean-all          - Remove all resources including PV/PVC"
	@echo "  logs-coordinator   - View coordinator logs"
	@echo "  logs-workers       - View worker logs"
	@echo "  logs-assembler     - View assembler logs"
	@echo "  logs-redis         - View Redis logs"
	@echo "  status             - Check deployment status"
	@echo "  scale-workers      - Scale workers (use REPLICAS=n)"