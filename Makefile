.PHONY: deploy run clean help deploy-all run-all clean-all setup-minikube destroy-minikube

# Individual deployment targets
deploy-a:
	cd a && make deploy

deploy-b:
	cd b && make deploy

deploy-c:
	cd c && make deploy

deploy-d:
	cd d && make deploy

deploy-e:
	cd e && make deploy

deploy-f:
	cd f && make deploy

deploy-g:
	cd g && make deploy

# Individual run targets
run-a:
	cd a && make run

run-b:
	cd b && make run

run-c:
	cd c && make run

run-d:
	cd d && make run

run-e:
	cd e && make run

run-f:
	cd f && make run

run-g:
	cd g && make run

# Individual clean targets
clean-a:
	cd a && make clean

clean-b:
	cd b && make clean

clean-c:
	cd c && make clean

clean-d:
	cd d && make clean

clean-e:
	cd e && make clean

clean-f:
	cd f && make clean

clean-g:
	cd g && make clean

# Setup minikube cluster
setup-minikube:
	@echo "=== Setting up Minikube Cluster ==="
	./scripts/setup-minikube.sh
	@echo "=== Minikube Setup Complete ==="

# Destroy minikube cluster completely
destroy-minikube:
	@echo "=== Destroying Minikube Cluster ==="
	@echo "This will completely remove the minikube cluster and all data."
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	minikube delete
	@echo "=== Minikube Cluster Destroyed ==="

# Deploy all implementations
deploy-all: clean-all
	@echo "=== Deploying All Implementations ==="
	@echo "Deploying shared storage..."
	kubectl apply -f k8s/shared-pv-pvc.yaml
	@echo "Deploying a/ (Sequential)..."
	$(MAKE) deploy-a
	@echo "Deploying b/ (Tile Parallel)..."
	$(MAKE) deploy-b
	@echo "Deploying c/ (Pipelined)..."
	$(MAKE) deploy-c
	@echo "Deploying d/ (Distributed Sequential)..."
	$(MAKE) deploy-d
	@echo "Deploying e/ (Distributed Queue)..."
	$(MAKE) deploy-e
	@echo "Deploying f/ (Fault Tolerant)..."
	$(MAKE) deploy-f
	@echo "Deploying g/ (Multithreaded Microservice)..."
	$(MAKE) deploy-g
	@echo "=== All Implementations Deployed ==="

# Run all implementations sequentially for performance comparison
run-all:
	@echo "=== Running All Implementations for Performance Comparison ==="
	@echo ""
	@echo "1/7: Running Sequential Implementation (a/)..."
	$(MAKE) run-a
	@echo ""
	@echo "2/7: Running Tile Parallel Implementation (b/)..."
	$(MAKE) run-b
	@echo ""
	@echo "3/7: Running Pipelined Implementation (c/)..."
	$(MAKE) run-c
	@echo ""
	@echo "4/7: Running Distributed Sequential Implementation (d/)..."
	$(MAKE) run-d
	@echo ""
	@echo "5/7: Running Distributed Queue Implementation (e/)..."
	$(MAKE) run-e
	@echo ""
	@echo "6/7: Running Fault Tolerant Implementation (f/)..."
	$(MAKE) run-f
	@echo ""
	@echo "7/7: Running Multithreaded Microservice Implementation (g/)..."
	$(MAKE) run-g
	@echo ""
	@echo "=== All Implementations Complete! ==="
	@echo "Check logs/ directory for performance comparison results."

# Clean all implementations
clean-all:
	@echo "=== Cleaning All Implementations ==="
	$(MAKE) clean-a
	$(MAKE) clean-b
	$(MAKE) clean-c
	$(MAKE) clean-d
	$(MAKE) clean-e
	$(MAKE) clean-f
	$(MAKE) clean-g
	kubectl delete pvc shared-storage-pvc --ignore-not-found=true
	kubectl delete pv shared-storage-pv --ignore-not-found=true
	@echo "=== All Implementations Cleaned ==="

# Default targets (aliases for convenience)
deploy: deploy-all
run: run-all
clean: clean-all

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Setup:"
	@echo "  setup-minikube   - Setup minikube cluster with proper resources"
	@echo "  destroy-minikube - Completely destroy minikube cluster"
	@echo ""
	@echo "Individual operations:"
	@echo "  deploy-[a-g]    - Deploy specific implementation"
	@echo "  run-[a-g]       - Run specific implementation"
	@echo "  clean-[a-g]     - Clean specific implementation"
	@echo ""
	@echo "Batch operations:"
	@echo "  deploy-all      - Deploy all implementations"
	@echo "  run-all         - Run all implementations sequentially"
	@echo "  clean-all       - Clean all implementations"
	@echo ""
	@echo "Aliases:"
	@echo "  deploy          - Same as deploy-all"
	@echo "  run             - Same as run-all"
	@echo "  clean           - Same as clean-all"
	@echo ""
	@echo "Examples:"
	@echo "  make setup-minikube   - Setup cluster (run this first!)"
	@echo "  make deploy           - Deploy all implementations"
	@echo "  make run              - Run complete performance comparison"
	@echo "  make run-a            - Run only sequential implementation"
	@echo "  make clean            - Clean up everything"
	@echo "  make destroy-minikube - Completely destroy cluster"