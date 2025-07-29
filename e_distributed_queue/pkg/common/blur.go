package common

import (
	"image/color"
	"go-blur/pkg/blur"
)

// Wrapper functions to maintain API compatibility with existing code
func GenerateGaussianKernel(size int) [][]float64 {
	return blur.GenerateGaussianKernel(size)
}

func ApplyBlurToTile(tileData [][]color.RGBA, kernel [][]float64) [][]color.RGBA {
	return blur.ApplyBlurToTile(tileData, kernel)
}

func ExtractCenter(paddedData [][]color.RGBA, padding, width, height int) [][]color.RGBA {
	return blur.ExtractCenter(paddedData, padding, width, height)
}