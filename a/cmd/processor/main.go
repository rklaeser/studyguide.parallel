package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"studyguide.parallel/pkg/blur"
	"studyguide.parallel/pkg/stats"
)

func main() {
	var (
		inputPath  = flag.String("input", "/input", "Input directory path")
		outputPath = flag.String("output", "/data/a/output", "Output directory path")
		kernelSize = flag.Int("kernel", 15, "Gaussian kernel size")
	)
	flag.Parse()

	startTime := time.Now()
	log.Printf("=== Starting Sequential Image Processing ===")
	log.Printf("Start time: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("Kernel size: %d", *kernelSize)
	log.Printf("Input path: %s", *inputPath)
	log.Printf("Output path: %s", *outputPath)

	// Create output directory
	if err := os.MkdirAll(*outputPath, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Find all PNG files in input directory
	files, err := filepath.Glob(filepath.Join(*inputPath, "*.png"))
	if err != nil {
		log.Fatalf("Failed to find input files: %v", err)
	}

	if len(files) == 0 {
		log.Fatalf("No PNG files found in %s", *inputPath)
	}

	// Create input and output paths for Run_a
	var inputPaths []string
	var outputPaths []string

	for _, file := range files {
		inputPaths = append(inputPaths, file)
		
		// Generate output filename
		filename := filepath.Base(file)
		name := strings.TrimSuffix(filename, filepath.Ext(filename))
		outputFile := filepath.Join(*outputPath, name+"_blurred.png")
		outputPaths = append(outputPaths, outputFile)
	}

	log.Printf("Found %d images to process", len(inputPaths))

	// Process images sequentially
	result := processSequential(inputPaths, outputPaths, *kernelSize)

	// Write performance results
	results := []stats.PerformanceData{result}
	stats.WritePerformanceResults(results)

	log.Printf("=== Processing Complete ===")
	log.Printf("Total execution time: %.2fs", time.Since(startTime).Seconds())
}

func processSequential(inputPaths []string, outputPaths []string, kernelSize int) stats.PerformanceData {
	fmt.Println("=== Starting Sequential Multi-Image Gaussian Blur ===")
	startTime := time.Now()
	
	if len(inputPaths) != len(outputPaths) {
		log.Fatalf("Input and output path arrays must have same length")
	}

	totalBlurTime := 0.0
	
	for i, inputPath := range inputPaths {
		imageTime, err := runSequentialSingle(inputPath, outputPaths[i], kernelSize)
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
	}
}

func runSequentialSingle(inputPath, outputPath string, kernelSize int) (float64, error) {
	startTime := time.Now()
	
	// Open input image
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return 0, err
	}
	defer inputFile.Close()

	img, _, err := image.Decode(inputFile)
	if err != nil {
		return 0, err
	}

	fmt.Printf("  Processing %s (%dx%d)...", filepath.Base(inputPath), img.Bounds().Dx(), img.Bounds().Dy())

	// Apply blur
	blurredImg := blur.ApplyBlurToImage(img, kernelSize)

	// Save output
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer outputFile.Close()

	err = png.Encode(outputFile, blurredImg)
	if err != nil {
		return 0, err
	}

	elapsed := time.Since(startTime).Seconds()
	fmt.Printf(" %.2fs\n", elapsed)
	
	return elapsed, nil
}