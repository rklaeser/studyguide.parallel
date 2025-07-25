package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"time"
)

// PerformanceData holds timing and metadata for algorithm results
type PerformanceData struct {
	AlgorithmName    string
	ImagesProcessed  int
	KernelSize      int
	TotalTime       float64
	AverageTime     float64
	InputPaths      []string
	OutputPaths     []string
	Timestamp       time.Time
	
	// Algorithm-specific data
	TotalBlurTime   *float64  // For sequential and parallel
	Workers         *int      // For parallel algorithms
	TileSize        *int      // For parallel algorithms
	QueueSize       *int      // For pipelined
}

// generateGaussianKernel creates a Gaussian kernel of given size
func generateGaussianKernel(size int) [][]float64 {
	kernel := make([][]float64, size)
	// Sigma should be proportional to size, but not too large
	// Common formula: sigma = radius / 3, where radius = size / 2
	sigma := float64(size) / 3.0
	sum := 0.0
	center := size / 2

	// Generate kernel values
	for i := 0; i < size; i++ {
		kernel[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			x := float64(i - center)
			y := float64(j - center)
			kernel[i][j] = math.Exp(-(x*x+y*y)/(2*sigma*sigma)) / (2 * math.Pi * sigma * sigma)
			sum += kernel[i][j]
		}
	}

	// Normalize kernel
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			kernel[i][j] /= sum
		}
	}

	return kernel
}

// applyBlurToImage applies Gaussian blur directly to an image (optimized for sequential processing)
func applyBlurToImage(img image.Image, kernelSize int) *image.RGBA {
	bounds := img.Bounds()
	blurred := image.NewRGBA(bounds)
	kernel := generateGaussianKernel(kernelSize)
	offset := kernelSize / 2

	// Convert input image to RGBA for direct pixel access
	var srcRGBA *image.RGBA
	if rgba, ok := img.(*image.RGBA); ok {
		srcRGBA = rgba
	} else {
		srcRGBA = image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcRGBA.Set(x, y, img.At(x, y))
			}
		}
	}

	width := bounds.Dx()
	height := bounds.Dy()

	// Process each pixel with direct pixel access
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rSum, gSum, bSum, aSum float64

			// Apply kernel
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					// Calculate source pixel position
					sx := x + kx - offset
					sy := y + ky - offset

					// Handle boundaries with clamping
					if sx < 0 {
						sx = 0
					} else if sx >= width {
						sx = width - 1
					}
					if sy < 0 {
						sy = 0
					} else if sy >= height {
						sy = height - 1
					}

					// Direct pixel access using RGBAAt - much faster than img.At()
					pixel := srcRGBA.RGBAAt(sx+bounds.Min.X, sy+bounds.Min.Y)
					weight := kernel[ky][kx]

					// Accumulate weighted values
					rSum += float64(pixel.R) * weight
					gSum += float64(pixel.G) * weight
					bSum += float64(pixel.B) * weight
					aSum += float64(pixel.A) * weight
				}
			}

			// Set blurred pixel directly
			blurred.Set(x+bounds.Min.X, y+bounds.Min.Y, color.RGBA{
				R: uint8(rSum),
				G: uint8(gSum),
				B: uint8(bSum),
				A: uint8(aSum),
			})
		}
	}

	return blurred
}

// applyBlurToTile applies Gaussian blur to tile data (optimized for parallel processing)
func applyBlurToTile(data [][]color.RGBA, kernel [][]float64) [][]color.RGBA {
	height := len(data)
	width := len(data[0])
	kernelSize := len(kernel)
	offset := kernelSize / 2
	
	result := make([][]color.RGBA, height)
	for i := range result {
		result[i] = make([]color.RGBA, width)
	}
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rSum, gSum, bSum, aSum float64
			
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					sx := x + kx - offset
					sy := y + ky - offset
					
					// Handle boundaries by clamping to valid range
					if sx < 0 {
						sx = 0
					}
					if sx >= width {
						sx = width - 1
					}
					if sy < 0 {
						sy = 0
					}
					if sy >= height {
						sy = height - 1
					}
					
					pixel := data[sy][sx]
					weight := kernel[ky][kx]
					
					rSum += float64(pixel.R) * weight
					gSum += float64(pixel.G) * weight
					bSum += float64(pixel.B) * weight
					aSum += float64(pixel.A) * weight
				}
			}
			
			result[y][x] = color.RGBA{
				R: uint8(rSum),
				G: uint8(gSum),
				B: uint8(bSum),
				A: uint8(aSum),
			}
		}
	}
	
	return result
}

// writePerformanceResults writes a single combined results file
func writePerformanceResults(results []PerformanceData) {
	if len(results) == 0 {
		return
	}
	
	// Use timestamp from first result
	timestamp := results[0].Timestamp.Format("2006-01-02_15-04-05")
	resultsFile := fmt.Sprintf("results/combined_%s.txt", timestamp)
	
	file, err := os.Create(resultsFile)
	if err != nil {
		log.Printf("Failed to create results file: %v", err)
		return
	}
	defer file.Close()
	
	fmt.Fprintf(file, "=== Combined Multi-Algorithm Gaussian Blur Results ===\n")
	fmt.Fprintf(file, "Timestamp: %s\n\n", results[0].Timestamp.Format("2006-01-02 15:04:05"))
	
	for _, result := range results {
		prefix := ""
		switch result.AlgorithmName {
		case "Sequential":
			prefix = "a_"
		case "Parallel":
			prefix = "b_"
		case "Pipelined":
			prefix = "c_"
		}
		
		fmt.Fprintf(file, "=== %s%s Results ===\n", prefix, result.AlgorithmName)
		fmt.Fprintf(file, "Images processed: %d\n", result.ImagesProcessed)
		fmt.Fprintf(file, "Kernel size: %d\n", result.KernelSize)
		
		if result.TotalBlurTime != nil {
			fmt.Fprintf(file, "Total blur time: %.2fs\n", *result.TotalBlurTime)
		}
		
		fmt.Fprintf(file, "Total execution time: %.2fs\n", result.TotalTime)
		fmt.Fprintf(file, "Average time per image: %.2fs\n", result.AverageTime)
		
		if result.Workers != nil {
			fmt.Fprintf(file, "Workers: %d\n", *result.Workers)
		}
		if result.TileSize != nil {
			fmt.Fprintf(file, "Tile size: %d\n", *result.TileSize)
		}
		if result.QueueSize != nil {
			fmt.Fprintf(file, "Queue size: %d\n", *result.QueueSize)
		}
		
		fmt.Fprintf(file, "\nInput files:\n")
		for i, path := range result.InputPaths {
			fmt.Fprintf(file, "  %d. %s\n", i+1, path)
		}
		
		fmt.Fprintf(file, "\nOutput files:\n")
		for i, path := range result.OutputPaths {
			fmt.Fprintf(file, "  %d. %s\n", i+1, path)
		}
		
		fmt.Fprintf(file, "\n")
	}
}