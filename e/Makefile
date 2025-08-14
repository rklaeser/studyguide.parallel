.PHONY: deploy run run-fault clean

# Deploy all services (builds images, deploys infrastructure)
deploy: clean
	@echo "Building images in minikube..."
	cd .. && eval $$(minikube docker-env) && \
		docker build -f e/Dockerfile.coordinator -t image-coordinator:latest . && \
		docker build -f e/Dockerfile.worker -t image-worker:latest . && \
		docker build -f e/Dockerfile.assembler -t image-assembler:latest .
	@echo "Deploying infrastructure..."
	kubectl apply -f k8s/pv-pvc-v2.yaml
	kubectl apply -f k8s/redis-deployment.yaml
	@echo "Creating default ConfigMap..."
	kubectl create configmap image-processor-config \
		--from-literal=INPUT_PATH=/input \
		--from-literal=OUTPUT_PATH=/e/output \
		--from-literal=KERNEL_SIZE=15 \
		--from-literal=REDIS_ADDR=redis:6379 \
		--from-literal=NUM_WORKERS=4 \
		--dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f k8s/worker-deployment.yaml
	kubectl apply -f k8s/assembler-deployment.yaml
	@echo "Waiting for services to be ready..."
	kubectl wait --for=condition=available --timeout=60s deployment/redis || true
	kubectl wait --for=condition=available --timeout=60s deployment/image-worker || true
	kubectl wait --for=condition=available --timeout=60s deployment/image-assembler || true

# Run the coordinator job with progress monitoring
run:
	./run.sh

# Run fault injection demo
run-fault:
	bash chaos/demo-inflight-tile-loss.sh

# Clean up all resources
clean:
	kubectl delete job image-coordinator --ignore-not-found=true
	kubectl delete deployment image-worker --ignore-not-found=true
	kubectl delete deployment image-assembler --ignore-not-found=true
	kubectl delete deployment redis --ignore-not-found=true
	kubectl delete service redis --ignore-not-found=true
	kubectl delete configmap image-processor-config --ignore-not-found=true