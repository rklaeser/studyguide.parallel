#!/bin/bash

# Host Storage Setup Script
# Run this on your Kubernetes node to set up local storage for the image processor

HOST_PATH="/mnt/image-data"

echo "Setting up host storage for image processing..."

# Create directories
echo "Creating directories at ${HOST_PATH}..."
sudo mkdir -p ${HOST_PATH}/input
sudo mkdir -p ${HOST_PATH}/output
sudo mkdir -p ${HOST_PATH}/processed

# Set permissions (allowing pods to read/write)
echo "Setting permissions..."
sudo chmod -R 777 ${HOST_PATH}

# Create some test structure
sudo mkdir -p ${HOST_PATH}/input/batch1
sudo mkdir -p ${HOST_PATH}/input/batch2
sudo mkdir -p ${HOST_PATH}/output/batch1
sudo mkdir -p ${HOST_PATH}/output/batch2

echo ""
echo "Host Storage Setup Complete!"
echo "============================"
echo "Storage Path: ${HOST_PATH}"
echo "Input Directory: ${HOST_PATH}/input"
echo "Output Directory: ${HOST_PATH}/output"
echo ""
echo "Directory structure:"
tree ${HOST_PATH} 2>/dev/null || ls -la ${HOST_PATH}/
echo ""
echo "To use:"
echo "1. Copy images to: ${HOST_PATH}/input/"
echo "2. Run the image processor job"
echo "3. Find processed images in: ${HOST_PATH}/output/"