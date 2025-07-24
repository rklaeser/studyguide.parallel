# studyguide.parallel
Implementing patterns from my C++ multi-threading course in Go

## Image Processing Approaches

This project demonstrates three different approaches to processing images with Gaussian blur:

### 1. Sequential Processing (~10 seconds)
- **File**: `a_sequential.go`
- **Approach**: Processes images one at a time in sequence
- **Implementation**: Single-threaded, applies Gaussian blur to each image sequentially
- **Performance**: Slowest approach, baseline for comparison

![Sequential Processing](visualize/a_sequential.png)

### 2. Parallel Tile Processing (~3 seconds)
- **File**: `b_tile_parallel.go`
- **Approach**: Tile level parallelism
- **Implementation**: 
  - Divides each image into 256x256 tiles with padding for seamless blur
  - Uses 10 worker goroutines to process tiles concurrently
  - Processes images sequentially but uses parallelism within each image
- **Performance**: ~3x faster than sequential due to multi-threading

![Parallel Tile Processing](visualize/b_tile_parallel.png)

### 3. Parallel Tile and Image Processing (~2 seconds)
- **File**: `c_tile+image_parallel.go`
- **Approach**: Tile level + image level parallelism
- **Implementation**:
  - Image loading, tile processing, and image assembly run concurrently
  - Multiple images can be in different pipeline stages simultaneously
  - Uses the same tile-based approach with 10 workers as parallel version
  - Optimizes overall throughput by overlapping I/O and computation
- **Performance**: Fastest approach, ~5x faster than sequential

![Parallel Tile and Image Processing](visualize/c_tile+image_parallel.png)

The multi-threading implementation uses Go's goroutines and channels to coordinate work between different pipeline stages, demonstrating concepts like producer-consumer patterns, work queues, and synchronization barriers.

Concurrent or Parallel?
  - The images are misleading because they show complete parallel processing when in actuality the threads will be parallel until the computer's cores are all become busy, at which point the processing will happen concurrently. My computer has 10 cores which is why I wrote the tile parallelism to use up to 10 threads.
