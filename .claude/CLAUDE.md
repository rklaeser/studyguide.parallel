# Parallel Image Processing Study Guide

## Project Overview

Each directory (a through g) implements the same core image processing logic using different parallelization architectures.

## Implementation Groups

### Group 1: Basic Parallel Approaches (a, b, c)
- **a/**: Sequential implementation (baseline)
- **b/**: Tile-based parallel processing
- **c/**: Tile + image parallel processing

### Group 2: Microservice Architectures (d, e, f, g)
- **d/**: Docker-based microservice
- **e/**: Kubernetes with Redis queue
- **f/**: Enhanced Kubernetes architecture
- **g/**: Multi-threaded microservice

## Development Guidelines

### Consistency Requirements
All implementations should maintain consistency in:
- **Input handling**: How images are read and processed
- **Output generation**: How processed images are saved and returned
- **Error handling**: Common error patterns and responses
- **Configuration**: Similar parameter handling across implementations
--**Performance**: How time is measured

### Debugging Strategy
When encountering bugs or issues:

1. **For Group 1 (a, b, c)**: Compare the problematic implementation with the other two in the group
   - Check how each handles the specific functionality
   - Identify why one implementation works while another doesn't
   - Apply working patterns from functional implementations

2. **For Group 2 (d, e, f, g)**: Compare with other microservice implementations
   - Examine container configurations and deployment patterns
   - Compare queue handling and service communication
   - Review Kubernetes manifests and Docker configurations

### Code Changes
When making changes:
- Ensure consistency across implementations in the same group
- Test similar functionality across related implementations
- Apply fixes or improvements to parallel implementations when applicable
- Maintain the architectural integrity of each approach while keeping core logic aligned

## Common Commands
- Build: `make build` or `go build`
- Test: `make test` or `go test`
- Run: `make run` or specific run scripts in each directory
- Deploy (for d, e, f, g): `kubectl apply -f k8s/`