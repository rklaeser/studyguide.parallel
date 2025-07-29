package blur

import (
	"image"
	"image/color"
	"math"
)

// GenerateGaussianKernel creates a Gaussian kernel of given size
func GenerateGaussianKernel(size int) [][]float64 {
	kernel := make([][]float64, size)
	// Sigma should be proportional to size, but not too large
	// Common formula: sigma = radius / 3, where radius = size / 2
	sigma := float64(size) / 3.0
	sum := 0.0
	center := size / 2

	// Generate kernel values
	for i := 0; i < size; i++ {
		kernel[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			x := float64(i - center)
			y := float64(j - center)
			kernel[i][j] = math.Exp(-(x*x+y*y)/(2*sigma*sigma)) / (2 * math.Pi * sigma * sigma)
			sum += kernel[i][j]
		}
	}

	// Normalize kernel
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			kernel[i][j] /= sum
		}
	}

	return kernel
}

// ApplyBlurToImage applies Gaussian blur directly to an image (optimized for sequential processing)
func ApplyBlurToImage(img image.Image, kernelSize int) *image.RGBA {
	bounds := img.Bounds()
	blurred := image.NewRGBA(bounds)
	kernel := GenerateGaussianKernel(kernelSize)
	offset := kernelSize / 2

	// Convert input image to RGBA for direct pixel access
	var srcRGBA *image.RGBA
	if rgba, ok := img.(*image.RGBA); ok {
		srcRGBA = rgba
	} else {
		srcRGBA = image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcRGBA.Set(x, y, img.At(x, y))
			}
		}
	}

	width := bounds.Dx()
	height := bounds.Dy()

	// Process each pixel with direct pixel access
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rSum, gSum, bSum, aSum float64

			// Apply kernel
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					// Calculate source pixel position
					sx := x + kx - offset
					sy := y + ky - offset

					// Handle boundaries with clamping
					if sx < 0 {
						sx = 0
					} else if sx >= width {
						sx = width - 1
					}
					if sy < 0 {
						sy = 0
					} else if sy >= height {
						sy = height - 1
					}

					// Direct pixel access using RGBAAt - much faster than img.At()
					pixel := srcRGBA.RGBAAt(sx+bounds.Min.X, sy+bounds.Min.Y)
					weight := kernel[ky][kx]

					// Accumulate weighted values
					rSum += float64(pixel.R) * weight
					gSum += float64(pixel.G) * weight
					bSum += float64(pixel.B) * weight
					aSum += float64(pixel.A) * weight
				}
			}

			// Set blurred pixel directly
			blurred.Set(x+bounds.Min.X, y+bounds.Min.Y, color.RGBA{
				R: uint8(rSum),
				G: uint8(gSum),
				B: uint8(bSum),
				A: uint8(aSum),
			})
		}
	}

	return blurred
}

// ApplyBlurToTile applies Gaussian blur to tile data (optimized for parallel processing)
func ApplyBlurToTile(data [][]color.RGBA, kernel [][]float64) [][]color.RGBA {
	height := len(data)
	width := len(data[0])
	kernelSize := len(kernel)
	offset := kernelSize / 2
	
	result := make([][]color.RGBA, height)
	for i := range result {
		result[i] = make([]color.RGBA, width)
	}
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rSum, gSum, bSum, aSum float64
			
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					sx := x + kx - offset
					sy := y + ky - offset
					
					// Handle boundaries by clamping to valid range
					if sx < 0 {
						sx = 0
					}
					if sx >= width {
						sx = width - 1
					}
					if sy < 0 {
						sy = 0
					}
					if sy >= height {
						sy = height - 1
					}
					
					pixel := data[sy][sx]
					weight := kernel[ky][kx]
					
					rSum += float64(pixel.R) * weight
					gSum += float64(pixel.G) * weight
					bSum += float64(pixel.B) * weight
					aSum += float64(pixel.A) * weight
				}
			}
			
			result[y][x] = color.RGBA{
				R: uint8(rSum),
				G: uint8(gSum),
				B: uint8(bSum),
				A: uint8(aSum),
			}
		}
	}
	
	return result
}

// ExtractCenter removes padding from processed tile data (FIXED VERSION)
func ExtractCenter(data [][]color.RGBA, padding, width, height int) [][]color.RGBA {
	result := make([][]color.RGBA, height)
	
	for y := 0; y < height; y++ {
		result[y] = make([]color.RGBA, width)
		for x := 0; x < width; x++ {
			// Clamp indices to available data bounds to prevent black artifacts
			srcY := y + padding
			srcX := x + padding
			if srcY >= len(data) {
				srcY = len(data) - 1
			}
			if srcX >= len(data[0]) {
				srcX = len(data[0]) - 1
			}
			result[y][x] = data[srcY][srcX]
		}
	}
	
	return result
}