package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"time"
	"studyguide.parallel/pkg/blur"
	"studyguide.parallel/pkg/stats"
)

// RunSequentialSingle executes the sequential blur for a single image
func RunSequentialSingle(inputPath, outputPath string, kernelSize int) (float64, error) {
	startTime := time.Now()
	
	// Open input image
	file, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open image: %v", err)
	}
	defer file.Close() // defer is used to close the file after the function is done

	// Decode image
	img, _, err := image.Decode(file) // decode gives us an image object that can be accessed by pixels and manipulated
	if err != nil {
		return 0, fmt.Errorf("failed to decode image: %v", err)
	}

	fmt.Printf("  Processing %s (%dx%d)...", inputPath, img.Bounds().Dx(), img.Bounds().Dy())

	// Apply Gaussian blur directly to the image
	blurred := blur.ApplyBlurToImage(img, kernelSize) // matrix math I don't understand that averages pixel according to its surrounding pixels

	// Save output image
	outFile, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	// Encode and save
	err = png.Encode(outFile, blurred) // saves the manipulated image object to a file
	if err != nil {
		return 0, fmt.Errorf("failed to encode image: %v", err)
	}

	duration := time.Since(startTime).Seconds()
	fmt.Printf(" %.2fs\n", duration)
	return duration, nil
}

// RunSequentialMultiple executes sequential blur for multiple images
func Run_a(inputPaths []string, outputPaths []string, kernelSize int) stats.PerformanceData {
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
	
	return stats.PerformanceData{
		AlgorithmName:   "Sequential",
		ImagesProcessed: len(inputPaths),
		KernelSize:      kernelSize,
		TotalTime:       totalTime,
		AverageTime:     totalTime / float64(len(inputPaths)),
		InputPaths:      inputPaths,
		OutputPaths:     outputPaths,
		Timestamp:       startTime,
		TotalBlurTime:   &totalBlurTime,
	}
}