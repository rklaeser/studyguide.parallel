package main

import (
	"fmt"
	"studyguide.parallel/pkg/stats"
)

func main() {
	kernelSize := 21
	
	// Define input and output paths for 5 images
	inputPaths := []string{
		"../input/img1.png",
		"../input/img2.png", 
		"../input/img3.png",
		"../input/img4.png",
		"../input/img5.png",
	}
	
	outputPaths := []string{
		"output/img1_blurred.png",
		"output/img2_blurred.png",
		"output/img3_blurred.png", 
		"output/img4_blurred.png",
		"output/img5_blurred.png",
	}

	fmt.Println("Running Tile Parallel Implementation:")
	result := Run_b(inputPaths, outputPaths, kernelSize)
	
	// Write results
	results := []stats.PerformanceData{result}
	stats.WritePerformanceResults(results)
	fmt.Println("Results written to logs/")
}