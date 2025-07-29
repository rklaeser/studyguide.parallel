package main

import (
	"image"
	"go-blur/pkg/blur"
)

// Wrapper functions to maintain compatibility
func generateGaussianKernel(size int) [][]float64 {
	return blur.GenerateGaussianKernel(size)
}

func applyBlurToImage(img image.Image, kernelSize int) *image.RGBA {
	return blur.ApplyBlurToImage(img, kernelSize)
}