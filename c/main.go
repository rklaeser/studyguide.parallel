package main

import (
	"fmt"
	"studyguide.parallel/pkg/stats"
)

func main() {
	kernelSize := 21
	
	// Define input paths for 5 images
	inputPaths := []string{
		"../input/img1.png",
		"../input/img2.png", 
		"../input/img3.png",
		"../input/img4.png",
		"../input/img5.png",
	}

	fmt.Println("Running Pipelined Implementation:")
	result := Run_c(inputPaths, kernelSize)
	
	// Write results
	results := []stats.PerformanceData{result}
	stats.WritePerformanceResults(results)
	fmt.Println("Results written to logs/")
}