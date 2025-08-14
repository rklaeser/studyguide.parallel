.PHONY: minikube-setup help

# Setup minikube cluster with increased resources for all projects
minikube-setup:
	./scripts/setup-minikube.sh

# Show available targets
help:
	@echo "Available targets:"
	@echo "  minikube-setup  - Setup minikube cluster with 4 CPUs and 8GB RAM"
	@echo ""
	@echo "Project-specific targets (run from project directories):"
	@echo "  cd d_distributed_sequential && make deploy"
	@echo "  cd e_distributed_queue && make deploy"  
	@echo "  cd f_fault_tolerant_queue && make deploy"
	@echo "  cd g_multithreaded_microservice && make deploy"