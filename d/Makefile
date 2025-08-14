.PHONY: build push deploy clean logs trigger-job minikube-start minikube-build minikube-deploy redis-deploy redis-clean

IMAGE_NAME = image-processor
IMAGE_TAG = latest
REGISTRY = localhost:5000

build:
	cd .. && docker build -f d/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

push: build
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

deploy-storage:
	kubectl apply -f k8s/pv-pvc.yaml

deploy-config:
	kubectl apply -f k8s/configmap.yaml

deploy: clean deploy-storage deploy-config
	kubectl apply -f k8s/job.yaml

clean:
	kubectl delete job -l app=image-processor --ignore-not-found=true
	kubectl delete configmap image-processor-config --ignore-not-found=true

clean-all: clean
	kubectl delete pvc image-processor-pvc --ignore-not-found=true
	kubectl delete pv image-processor-pv --ignore-not-found=true

logs:
	kubectl logs -l app=image-processor --tail=100 -f

status:
	kubectl get jobs -l app=image-processor
	kubectl get pods -l app=image-processor

trigger-job:
	./scripts/trigger-job.sh

# Minikube-specific targets
minikube-start:
	minikube start
	@echo "Minikube started. Use 'eval \$$(minikube docker-env)' to configure Docker."

redis-deploy:
	kubectl create deployment redis --image=redis:alpine --dry-run=client -o yaml | kubectl apply -f -
	kubectl expose deployment redis --port=6379 --dry-run=client -o yaml | kubectl apply -f -
	@echo "Redis deployed and exposed on port 6379"

minikube-build: minikube-start
	@echo "Building image in minikube Docker environment..."
	cd .. && eval $$(minikube docker-env) && docker build -f d/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

minikube-deploy: minikube-build redis-deploy clean deploy-storage deploy-config
	kubectl apply -f k8s/job.yaml
	@echo "Full minikube deployment complete. Use 'make logs' to monitor."

redis-clean:
	kubectl delete deployment redis --ignore-not-found=true
	kubectl delete service redis --ignore-not-found=true
	@echo "Redis resources cleaned up"

minikube-clean: clean redis-clean
	@echo "Minikube resources cleaned up"

help:
	@echo "Available targets:"
	@echo "  build         - Build Docker image"
	@echo "  push          - Push image to registry"
	@echo "  deploy        - Deploy all Kubernetes resources"
	@echo "  clean         - Remove jobs and configmap"
	@echo "  clean-all     - Remove all resources including PV/PVC"
	@echo "  logs          - View processor logs"
	@echo "  status        - Check job and pod status"
	@echo "  trigger-job   - Run the trigger script to create a new job"
	@echo ""
	@echo "Minikube targets:"
	@echo "  minikube-start    - Start minikube cluster"
	@echo "  minikube-build    - Build image in minikube Docker env"
	@echo "  minikube-deploy   - Full deployment to minikube (Redis + app)"
	@echo "  redis-deploy      - Deploy Redis to cluster"
	@echo "  redis-clean       - Remove Redis resources"
	@echo "  minikube-clean    - Clean all minikube resources"