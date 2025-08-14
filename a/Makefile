.PHONY: deploy run clean

IMAGE_NAME = sequential-processor
IMAGE_TAG = latest

# Deploy all resources (builds image, deploys infrastructure and job)
deploy: clean
	@echo "Cleaning output directory..."
	rm -rf output/*
	@echo "Building image in minikube..."
	cd .. && eval $$(minikube docker-env) && \
		docker build -f a/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "Deploying infrastructure..."
	kubectl apply -f ../k8s/shared-pv-pvc.yaml
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/job.yaml
	@echo "Deployment complete. Use 'make run' to monitor progress."

# Monitor running job
run:
	./run.sh

# Clean up all resources (keep PV/PVC for data persistence)
clean:
	kubectl delete job sequential-processor-job --ignore-not-found=true
	kubectl delete configmap sequential-processor-config --ignore-not-found=true