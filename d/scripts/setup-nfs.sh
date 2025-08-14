#!/bin/bash

# NFS Setup Script for Proxmox Host
# Run this on your Proxmox host to set up NFS for the image processor

NFS_PATH="/mnt/image-data"
NFS_NETWORK="10.0.0.0/24"  # Adjust to your network

echo "Setting up NFS server for image processing..."

# Install NFS server if not already installed
if ! command -v exportfs &> /dev/null; then
    echo "Installing NFS server..."
    apt-get update
    apt-get install -y nfs-kernel-server
fi

# Create directories
echo "Creating directories..."
mkdir -p ${NFS_PATH}/input
mkdir -p ${NFS_PATH}/output
mkdir -p ${NFS_PATH}/processed

# Set permissions
chmod -R 777 ${NFS_PATH}

# Add to exports
echo "Configuring NFS exports..."
if ! grep -q "${NFS_PATH}" /etc/exports; then
    echo "${NFS_PATH} ${NFS_NETWORK}(rw,sync,no_subtree_check,no_root_squash)" >> /etc/exports
fi

# Export the filesystem
exportfs -ra

# Enable and start NFS
systemctl enable nfs-kernel-server
systemctl restart nfs-kernel-server

# Show status
echo ""
echo "NFS Setup Complete!"
echo "==================="
echo "NFS Path: ${NFS_PATH}"
echo "Allowed Network: ${NFS_NETWORK}"
echo ""
echo "Test from a client with:"
echo "  showmount -e $(hostname -I | awk '{print $1}')"
echo "  mount -t nfs $(hostname -I | awk '{print $1}'):${NFS_PATH} /mnt/test"