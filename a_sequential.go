package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"strings"
	"time"
)

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

// applyGaussianBlur applies Gaussian blur to an image
func applyGaussianBlur(img image.Image, kernelSize int) *image.RGBA {
	bounds := img.Bounds()
	blurred := image.NewRGBA(bounds)
	kernel := generateGaussianKernel(kernelSize)
	offset := kernelSize / 2

	// Process each pixel
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var rSum, gSum, bSum, aSum float64

			// Apply kernel
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					// Calculate source pixel position
					sx := x + kx - offset
					sy := y + ky - offset

					// Handle image boundaries
					if sx < bounds.Min.X {
						sx = bounds.Min.X
					}
					if sx >= bounds.Max.X {
						sx = bounds.Max.X - 1
					}
					if sy < bounds.Min.Y {
						sy = bounds.Min.Y
					}
					if sy >= bounds.Max.Y {
						sy = bounds.Max.Y - 1
					}

					// Get pixel color
					r, g, b, a := img.At(sx, sy).RGBA()
					weight := kernel[ky][kx]

					// Accumulate weighted values
					rSum += float64(r) * weight
					gSum += float64(g) * weight
					bSum += float64(b) * weight
					aSum += float64(a) * weight
				}
			}

			// Set blurred pixel
			blurred.Set(x, y, color.RGBA{
				R: uint8(rSum / 256),
				G: uint8(gSum / 256),
				B: uint8(bSum / 256),
				A: uint8(aSum / 256),
			})
		}
	}

	return blurred
}

// RunSequentialSingle executes the sequential blur for a single image
func RunSequentialSingle(inputPath, outputPath string, kernelSize int) (float64, error) {
	startTime := time.Now()
	
	// Open input image
	file, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open image: %v", err)
	}
	defer file.Close()

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		return 0, fmt.Errorf("failed to decode image: %v", err)
	}

	fmt.Printf("  Processing %s (%dx%d)...", inputPath, img.Bounds().Dx(), img.Bounds().Dy())

	// Apply Gaussian blur
	blurred := applyGaussianBlur(img, kernelSize)

	// Save output image
	outFile, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	// Encode and save
	err = png.Encode(outFile, blurred)
	if err != nil {
		return 0, fmt.Errorf("failed to encode image: %v", err)
	}

	duration := time.Since(startTime).Seconds()
	fmt.Printf(" %.2fs\n", duration)
	return duration, nil
}

// RunSequentialMultiple executes sequential blur for multiple images
func RunSequentialMultiple(inputPaths []string, outputPaths []string, kernelSize int) {
	fmt.Println("=== Starting Sequential Multi-Image Gaussian Blur ===")
	startTime := time.Now()
	
	if len(inputPaths) != len(outputPaths) {
		log.Fatalf("Input and output path arrays must have same length")
	}

	totalBlurTime := 0.0
	
	for i, inputPath := range inputPaths {
		imageTime, err := RunSequentialSingle(inputPath, outputPaths[i], kernelSize)
		if err != nil {
			log.Fatalf("Error processing image %d: %v", i+1, err)
		}
		totalBlurTime += imageTime
	}

	totalTime := time.Since(startTime).Seconds()
	fmt.Printf("\n=== Sequential Multi-Image Blur Complete ===\n")
	fmt.Printf("Images processed: %d\n", len(inputPaths))
	fmt.Printf("Total blur time: %.2fs\n", totalBlurTime)
	fmt.Printf("Total execution time: %.2fs\n", totalTime)
	fmt.Printf("Average time per image: %.2fs\n", totalTime/float64(len(inputPaths)))
}

func main() {
	kernelSize := 21
	
	// Define input and output paths for 5 images
	inputPaths := []string{
		"img/img1.png",
		"img/img2.png", 
		"img/img3.png",
		"img/img4.png",
		"img/img5.png",
	}
	
	sequentialOutputs := []string{
		"img/blurred_sequential_1.png",
		"img/blurred_sequential_2.png",
		"img/blurred_sequential_3.png", 
		"img/blurred_sequential_4.png",
		"img/blurred_sequential_5.png",
	}
	
	parallelOutputs := []string{
		"img/blurred_parallel_1.png",
		"img/blurred_parallel_2.png",
		"img/blurred_parallel_3.png",
		"img/blurred_parallel_4.png", 
		"img/blurred_parallel_5.png",
	}
	
	// Run sequential version for multiple images
	RunSequentialMultiple(inputPaths, sequentialOutputs, kernelSize)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	
	// Run parallel version for multiple images
	RunParallelMultiple(inputPaths, parallelOutputs, kernelSize)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	
	// Run pipelined version for multiple images
	RunPipelined(inputPaths, kernelSize)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("=== Performance Comparison ===")
	fmt.Println("Sequential vs Parallel vs Pipelined - Check timing results above!")
}