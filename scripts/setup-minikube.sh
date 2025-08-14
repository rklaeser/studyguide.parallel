#!/bin/bash

echo "Setting up minikube cluster with increased resources..."

# Delete existing cluster to allow resource changes
echo "Deleting existing minikube cluster to change resources..."
minikube delete 2>/dev/null || true

# Start minikube with increased resources
echo "Starting minikube with 4 CPUs and 8GB RAM..."
minikube start --cpus=4 --memory=8192

# Enable useful addons
echo "Enabling addons..."
minikube addons enable metrics-server || echo "Warning: metrics-server addon failed"

echo "Minikube setup complete!"
echo "Cluster info:"
kubectl get nodes
echo ""
echo "Available resources:"
kubectl describe node minikube | grep -A 5 "Allocatable:"
echo ""
echo "Storage classes:"
kubectl get storageclass
echo ""
echo "Note: Projects use dynamic storage provisioning for reliability across cluster recreations"