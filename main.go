package main

import (
	"fmt"
	"go-blur/pkg/stats"
)

func main() {
	kernelSize := 21
	
	// Define input and output paths for 5 images
	inputPaths := []string{
		"data/input/img1.png",
		"data/input/img2.png", 
		"data/input/img3.png",
		"data/input/img4.png",
		"data/input/img5.png",
	}
	
	outputPathsA := []string{
		"data/a_output/img1_blurred.png",
		"data/a_output/img2_blurred.png",
		"data/a_output/img3_blurred.png", 
		"data/a_output/img4_blurred.png",
		"data/a_output/img5_blurred.png",
	}
	
	outputPathsB := []string{
		"data/b_output/img1_blurred.png",
		"data/b_output/img2_blurred.png",
		"data/b_output/img3_blurred.png",
		"data/b_output/img4_blurred.png", 
		"data/b_output/img5_blurred.png",
	}

	fmt.Println("Running all three blur implementations...")
	fmt.Println()

	var results []stats.PerformanceData

	// Run sequential version
	fmt.Println("1. Running Sequential Implementation (a_sequential.go):")
	resultA := Run_a(inputPaths, outputPathsA, kernelSize)
	results = append(results, resultA)
	fmt.Println()

	// Run tile parallel version
	fmt.Println("2. Running Tile Parallel Implementation (b_tile_parallel.go):")
	resultB := Run_b(inputPaths, outputPathsB, kernelSize)
	results = append(results, resultB)
	fmt.Println()

	// Run pipelined version
	fmt.Println("3. Running Pipelined Implementation (c_tile+image_parallel.go):")
	resultC := Run_c(inputPaths, kernelSize)
	results = append(results, resultC)

	fmt.Println("\nAll implementations completed!")
	
	// Write combined results file
	fmt.Println("Writing combined results file...")
	stats.WritePerformanceResults(results)
	fmt.Println("Results written to logs/abc_*.txt")
}