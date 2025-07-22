# studyguide.parallel
Implementing patterns from my C++ multi-threading course in Go

## Image Processing Approaches

This project demonstrates three different approaches to processing images with Gaussian blur:

### 1. Sequential Processing (~10 seconds)
- **File**: `main.go`
- **Approach**: Processes images one at a time in sequence
- **Implementation**: Single-threaded, applies Gaussian blur to each image sequentially
- **Performance**: Slowest approach, baseline for comparison

### 2. Parallel Processing (~3 seconds)
- **File**: `parallel.go`
- **Approach**: Multi-threaded tile-based processing for each image
- **Implementation**: 
  - Divides each image into 256x256 tiles with padding for seamless blur
  - Uses 10 worker goroutines to process tiles concurrently
  - Processes images sequentially but uses parallelism within each image
- **Performance**: ~3x faster than sequential due to multi-threading

### 3. Pipelined Processing (~2 seconds)
- **File**: `pipelined.go`
- **Approach**: Full pipeline parallelism across multiple images
- **Implementation**:
  - Image loading, tile processing, and image assembly run concurrently
  - Multiple images can be in different pipeline stages simultaneously
  - Uses the same tile-based approach with 10 workers as parallel version
  - Optimizes overall throughput by overlapping I/O and computation
- **Performance**: Fastest approach, ~5x faster than sequential

The multi-threading implementation uses Go's goroutines and channels to coordinate work between different pipeline stages, demonstrating concepts like producer-consumer patterns, work queues, and synchronization barriers.
