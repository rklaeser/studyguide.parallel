package main

import (
	"fmt"
	"studyguide.parallel/pkg/stats"
)

func main() {
	kernelSize := 21
	
	// Define input and output paths for 5 images
	inputPaths := []string{
		"input/img1.png",
		"input/img2.png", 
		"input/img3.png",
		"input/img4.png",
		"input/img5.png",
	}
	
	outputPathsA := []string{
		"a/output/img1_blurred.png",
		"a/output/img2_blurred.png",
		"a/output/img3_blurred.png", 
		"a/output/img4_blurred.png",
		"a/output/img5_blurred.png",
	}
	
	outputPathsB := []string{
		"b/output/img1_blurred.png",
		"b/output/img2_blurred.png",
		"b/output/img3_blurred.png",
		"b/output/img4_blurred.png", 
		"b/output/img5_blurred.png",
	}

	fmt.Println("Running all three blur implementations...")
	fmt.Println()

	var results []stats.PerformanceData

	// Run sequential version
	fmt.Println("1. Running Sequential Implementation (a/a_sequential.go):")
	resultA := Run_a(inputPaths, outputPathsA, kernelSize)
	results = append(results, resultA)
	fmt.Println()

	// Run tile parallel version
	fmt.Println("2. Running Tile Parallel Implementation (b/b_tile_parallel.go):")
	resultB := Run_b(inputPaths, outputPathsB, kernelSize)
	results = append(results, resultB)
	fmt.Println()

	// Run pipelined version
	fmt.Println("3. Running Pipelined Implementation (c/c_tile+image_parallel.go):")
	resultC := Run_c(inputPaths, kernelSize)
	results = append(results, resultC)

	fmt.Println("\nAll implementations completed!")
	
	// Write combined results file
	fmt.Println("Writing combined results file...")
	stats.WritePerformanceResults(results)
	fmt.Println("Results written to logs/abc_*.txt")
}