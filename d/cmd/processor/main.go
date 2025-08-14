package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"studyguide.parallel/pkg/stats"
)

func main() {
	var (
		inputPath  = flag.String("input", "/input", "Input directory path")
		outputPath = flag.String("output", "/d/output", "Output directory path")
		kernelSize = flag.Int("kernel", 15, "Gaussian kernel size")
		inputFile  = flag.String("file", "", "Specific input file to process (optional)")
	)
	flag.Parse()

	startTime := time.Now()
	log.Printf("=== Starting Distributed Sequential Image Processing ===")
	log.Printf("Start time: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("Kernel size: %d", *kernelSize)
	log.Printf("Input path: %s", *inputPath)
	log.Printf("Output path: %s", *outputPath)

	// Ensure output directory exists
	if err := os.MkdirAll(*outputPath, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	var result stats.PerformanceData
	
	// Process specific file or all files in directory
	if *inputFile != "" {
		inputPaths := []string{filepath.Join(*inputPath, *inputFile)}
		outputPaths := []string{} // Will be filled by processFile
		result = processFileWithTiming(inputPaths[0], *outputPath, *kernelSize, startTime)
		outputPaths = append(outputPaths, result.OutputPaths...)
		result.InputPaths = inputPaths
		result.OutputPaths = outputPaths
	} else {
		result = processDirectoryWithTiming(*inputPath, *outputPath, *kernelSize, startTime)
	}

	// Output performance results
	totalTime := time.Since(startTime).Seconds()
	log.Printf("=== Processing Complete ===")
	log.Printf("Total time: %.2fs", totalTime)
	log.Printf("Images processed: %d", result.ImagesProcessed)
	log.Printf("Average time per image: %.2fs", result.AverageTime)
	
	// Write stats file if processing multiple images
	if result.ImagesProcessed > 1 {
		results := []stats.PerformanceData{result}
		stats.WritePerformanceResultsWithPrefix(results, "d_")
		log.Println("Performance results written to logs/d_*.txt")
	}
}

func processDirectoryWithTiming(inputDir, outputDir string, kernelSize int, overallStartTime time.Time) stats.PerformanceData {
	files, err := os.ReadDir(inputDir)
	if err != nil {
		log.Fatalf("Failed to read input directory: %v", err)
	}

	var inputPaths []string
	var outputPaths []string
	var totalBlurTime float64
	processedCount := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			inputPath := filepath.Join(inputDir, file.Name())
			
			blurTime, outputPath, err := processFileWithDetailedTiming(inputPath, outputDir, kernelSize)
			if err != nil {
				log.Printf("Failed to process %s: %v", file.Name(), err)
			} else {
				inputPaths = append(inputPaths, inputPath)
				outputPaths = append(outputPaths, outputPath)
				totalBlurTime += blurTime
				processedCount++
			}
		}
	}

	totalTime := time.Since(overallStartTime).Seconds()
	log.Printf("Processed %d images", processedCount)

	return stats.PerformanceData{
		AlgorithmName:   "Distributed Sequential",
		ImagesProcessed: processedCount,
		KernelSize:      kernelSize,
		TotalTime:       totalTime,
		AverageTime:     totalTime / float64(processedCount),
		InputPaths:      inputPaths,
		OutputPaths:     outputPaths,
		Timestamp:       overallStartTime,
		TotalBlurTime:   &totalBlurTime,
	}
}

func processDirectory(inputDir, outputDir string, kernelSize int) {
	files, err := os.ReadDir(inputDir)
	if err != nil {
		log.Fatalf("Failed to read input directory: %v", err)
	}

	processedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			inputPath := filepath.Join(inputDir, file.Name())
			if err := processFile(inputPath, outputDir, kernelSize); err != nil {
				log.Printf("Failed to process %s: %v", file.Name(), err)
			} else {
				processedCount++
			}
		}
	}

	log.Printf("Processed %d images", processedCount)
}

func processFile(inputPath, outputDir string, kernelSize int) error {
	log.Printf("Processing: %s", inputPath)

	// Open and decode image
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Apply Gaussian blur
	blurred := applyBlurToImage(img, kernelSize)

	// Generate output filename
	baseName := filepath.Base(inputPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	outputFileName := fmt.Sprintf("%s_blurred.%s", nameWithoutExt, format)
	outputPath := filepath.Join(outputDir, outputFileName)

	// Save blurred image
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	switch format {
	case "jpeg":
		err = jpeg.Encode(outFile, blurred, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(outFile, blurred)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	log.Printf("Saved blurred image to: %s", outputPath)
	return nil
}

// processFileWithDetailedTiming wraps processFile with detailed timing
func processFileWithDetailedTiming(inputPath, outputDir string, kernelSize int) (blurTime float64, outputPath string, err error) {
	log.Printf("Processing: %s", inputPath)

	// Open and decode image
	file, err := os.Open(inputPath)
	if err != nil {
		return 0, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return 0, "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Time the blur operation
	blurStart := time.Now()
	blurred := applyBlurToImage(img, kernelSize)
	blurTime = time.Since(blurStart).Seconds()

	// Generate output filename
	baseName := filepath.Base(inputPath)
	nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	outputFileName := fmt.Sprintf("%s_blurred.%s", nameWithoutExt, format)
	outputPath = filepath.Join(outputDir, outputFileName)

	// Save blurred image
	outFile, err := os.Create(outputPath)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	switch format {
	case "jpeg":
		err = jpeg.Encode(outFile, blurred, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(outFile, blurred)
	default:
		return 0, "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return 0, "", fmt.Errorf("failed to encode image: %w", err)
	}

	log.Printf("Saved blurred image to: %s (blur time: %.3fs)", outputPath, blurTime)
	return blurTime, outputPath, nil
}

// processFileWithTiming wraps single file processing with timing for stats
func processFileWithTiming(inputPath, outputDir string, kernelSize int, startTime time.Time) stats.PerformanceData {
	blurTime, outputPath, err := processFileWithDetailedTiming(inputPath, outputDir, kernelSize)
	if err != nil {
		log.Fatalf("Failed to process file: %v", err)
	}

	totalTime := time.Since(startTime).Seconds()

	return stats.PerformanceData{
		AlgorithmName:   "Distributed Sequential",
		ImagesProcessed: 1,
		KernelSize:      kernelSize,
		TotalTime:       totalTime,
		AverageTime:     totalTime,
		InputPaths:      []string{inputPath},
		OutputPaths:     []string{outputPath},
		Timestamp:       startTime,
		TotalBlurTime:   &blurTime,
	}
}