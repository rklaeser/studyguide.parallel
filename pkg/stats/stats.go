package stats

import (
	"fmt"
	"log"
	"os"
	"time"
)

// PerformanceData holds timing and metadata for algorithm results
type PerformanceData struct {
	AlgorithmName   string
	ImagesProcessed int
	KernelSize      int
	TotalTime       float64
	AverageTime     float64
	InputPaths      []string
	OutputPaths     []string
	Timestamp       time.Time

	// Algorithm-specific data
	TotalBlurTime *float64 // For sequential and parallel
	Workers       *int     // For parallel algorithms
	TileSize      *int     // For parallel algorithms
	QueueSize     *int     // For pipelined
}

// WritePerformanceResults writes a single combined results file
func WritePerformanceResults(results []PerformanceData) {
	WritePerformanceResultsWithPrefix(results, "abc_")
}

// WritePerformanceResultsWithPrefix writes results file with custom prefix
func WritePerformanceResultsWithPrefix(results []PerformanceData, prefix string) {
	if len(results) == 0 {
		return
	}

	// Ensure logs directory exists
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
		return
	}

	// Use timestamp from first result
	timestamp := results[0].Timestamp.Format("2006-01-02_15-04-05")
	resultsFile := fmt.Sprintf("logs/%s%s.txt", prefix, timestamp)

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