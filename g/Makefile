.PHONY: deploy run run-local run-distributed clean logs

# Deploy all services (builds images, deploys infrastructure)
deploy: clean
	@echo "Cleaning output directory..."
	rm -rf output/*
	@echo "Building multithreaded microservice image in minikube..."
	eval $$(minikube docker-env) && \
		docker build -f Dockerfile -t mt-service:latest ..
	@echo "Deploying infrastructure..."
	kubectl apply -f ../k8s/shared-pv-pvc.yaml
	kubectl apply -f k8s/redis-deployment.yaml
	@echo "Waiting for Redis to be ready..."
	kubectl wait --for=condition=available --timeout=60s deployment/redis || true
	@echo "Deploying service components..."
	kubectl apply -f k8s/worker-deployment.yaml
	kubectl apply -f k8s/assembler-deployment.yaml
	@echo "Waiting for services to be ready..."
	kubectl wait --for=condition=available --timeout=90s deployment/mt-workers || true
	kubectl wait --for=condition=available --timeout=60s deployment/mt-assembler || true
	@echo "Setting up shared volume with input images..."
	@# Use a temporary pod to copy images to shared volume
	kubectl run temp-setup --image=busybox --restart=Never --rm -i --overrides='{ \
		"spec": { \
			"containers": [{ \
				"name": "setup", \
				"image": "busybox", \
				"command": ["sh", "-c", "mkdir -p /data/input /data/output && echo Setup complete"], \
				"volumeMounts": [{"name": "shared-data", "mountPath": "/data"}] \
			}], \
			"volumes": [{"name": "shared-data", "persistentVolumeClaim": {"claimName": "shared-storage-pvc"}}] \
		} \
	}' --timeout=60s || true
	@# Copy images to shared volume using first available pod
	@POD=$$(kubectl get pods -l app=mt-worker -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || \
		kubectl get pods -l app=mt-assembler -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	if [ -n "$$POD" ]; then \
		echo "Copying images to shared volume via pod $$POD..."; \
		kubectl cp ../input $$POD:/data/; \
	else \
		echo "No pods available for copying files"; \
		exit 1; \
	fi
	@echo "Deploy complete! Shared volume ready with input images."

# Run the coordinator job to process images (assumes deploy has been run)
run:
	@echo "Starting coordinator job..."
	kubectl delete job mt-coordinator --ignore-not-found=true
	kubectl apply -f k8s/coordinator-job.yaml
	@echo "Monitoring job progress..."
	@kubectl wait --for=condition=complete --timeout=300s job/mt-coordinator || \
		(echo "Job did not complete in time. Check logs with: kubectl logs job/mt-coordinator"; exit 1)
	@echo "Processing complete! Fetching results..."
	@echo "Copying processed images to ./output/"
	@rm -rf ./output 2>/dev/null || true
	@mkdir -p ./output
	@TEMP_POD="output-copy-$$$$-$$RANDOM" && \
	kubectl run $$TEMP_POD --image=busybox --restart=Never \
		--overrides='{ \
			"spec": { \
				"containers": [{ \
					"name": "copy", \
					"image": "busybox", \
					"command": ["sleep", "300"], \
					"volumeMounts": [{"name": "shared-data", "mountPath": "/data"}] \
				}], \
				"volumes": [{"name": "shared-data", "persistentVolumeClaim": {"claimName": "shared-storage-pvc"}}] \
			} \
		}' 2>/dev/null && \
	sleep 3 && \
	echo "Checking files in /data/g/output/..." && \
	kubectl exec $$TEMP_POD -- ls /data/g/output/ 2>/dev/null | grep "\.png$$" | while read filename; do \
		echo "Copying $$filename..."; \
		kubectl cp "$$TEMP_POD:/data/g/output/$$filename" "./output/$$filename" 2>/dev/null; \
	done && \
	kubectl delete pod $$TEMP_POD --force --grace-period=0 2>/dev/null
	@echo "Images available in ./output/ directory:"
	@ls -la ./output/ 2>/dev/null || echo "No images copied"

# Run locally with Docker Redis
run-local:
	@echo "Checking for Redis..."
	@if ! nc -z localhost 6379 2>/dev/null; then \
		echo "Starting Redis in Docker..."; \
		docker run -d --name mt-redis -p 6379:6379 redis:7-alpine || docker start mt-redis; \
		sleep 2; \
	fi
	@echo "Building service..."
	go build -o mt-service cmd/service/main.go
	@echo "Creating output directory..."
	mkdir -p ../data/g_output
	@echo "Starting multithreaded microservice..."
	./mt-service \
		-mode=all \
		-redis=localhost:6379 \
		-input=../data/input \
		-output=../data/g_output \
		-workers=10 \
		-kernel=15

# Run in distributed mode (separate components)
run-distributed:
	@echo "Deploying all-in-one service deployment..."
	kubectl delete deployment mt-service --ignore-not-found=true
	kubectl apply -f k8s/service-deployment.yaml
	@echo "Waiting for service pods..."
	kubectl wait --for=condition=available --timeout=120s deployment/mt-service
	@echo "Service deployed with 3 replicas running all components"
	@echo "Check logs with: make logs"

# Clean up all resources
clean:
	@echo "Cleaning up Kubernetes resources..."
	kubectl delete job mt-coordinator --ignore-not-found=true
	kubectl delete deployment mt-workers --ignore-not-found=true
	kubectl delete deployment mt-assembler --ignore-not-found=true
	kubectl delete deployment mt-service --ignore-not-found=true
	kubectl delete deployment redis --ignore-not-found=true
	kubectl delete service redis --ignore-not-found=true
	@# Note: shared-storage-pvc and shared-storage-pv are managed by main Makefile
	@echo "Cleaning up local Docker containers..."
	docker stop mt-redis 2>/dev/null || true
	docker rm mt-redis 2>/dev/null || true
	@echo "Cleaning up build artifacts..."
	rm -f mt-service
	rm -rf ../data/g_output
	@echo "Cleanup complete"

# View logs from all components
logs:
	@echo "=== Worker Logs ==="
	kubectl logs -l app=mt-worker --tail=20 || true
	@echo "\n=== Assembler Logs ==="
	kubectl logs -l app=mt-assembler --tail=20 || true
	@echo "\n=== Coordinator Logs ==="
	kubectl logs job/mt-coordinator --tail=50 || true
	@echo "\n=== Service Logs (if using all-in-one) ==="
	kubectl logs -l app=mt-service --tail=20 || true