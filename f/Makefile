.PHONY: deploy run run-fault clean

# Deploy all services (builds images, deploys infrastructure)
deploy: clean
	@echo "Cleaning output directory..."
	rm -rf output/*
	@echo "Building FTQ images in minikube..."
	cd .. && eval $$(minikube docker-env) && \
		docker build -f f/Dockerfile.coordinator -t ftq-coordinator:latest . && \
		docker build -f f/Dockerfile.worker -t ftq-worker:latest . && \
		docker build -f f/Dockerfile.assembler -t ftq-assembler:latest .
	@echo "Deploying infrastructure..."
	kubectl apply -f ../k8s/shared-pv-pvc.yaml
	kubectl apply -f ../e/k8s/redis-deployment.yaml
	kubectl apply -f k8s/worker-deployment.yaml
	kubectl apply -f k8s/assembler-deployment.yaml
	@echo "Waiting for services to be ready..."
	kubectl wait --for=condition=available --timeout=60s deployment/redis || true
	kubectl wait --for=condition=available --timeout=90s deployment/ftq-worker || true
	kubectl wait --for=condition=available --timeout=60s deployment/ftq-assembler || true

# Run the coordinator job with progress monitoring
run:
	./run.sh

# Run fault injection demo (demonstrates FTQ's fault tolerance)
run-fault:
	./run-fault.sh

# Clean up all resources
clean:
	kubectl delete job ftq-coordinator --ignore-not-found=true
	kubectl delete deployment ftq-worker --ignore-not-found=true
	kubectl delete deployment ftq-assembler --ignore-not-found=true