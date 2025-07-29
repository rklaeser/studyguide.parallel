package common

import (
	"image/color"
	"time"
)

const (
	TILE_SIZE = 256
)

// ImageTile represents a tile that belongs to a specific image
type ImageTile struct {
	ImageID int               `json:"image_id"`
	TileID  int               `json:"tile_id"`
	X       int               `json:"x"`
	Y       int               `json:"y"`
	Width   int               `json:"width"`
	Height  int               `json:"height"`
	Data    [][]color.RGBA    `json:"data"`
	Padding int               `json:"padding"`
}

// ProcessedImageTile represents a processed tile with image association
type ProcessedImageTile struct {
	ImageID int               `json:"image_id"`
	TileID  int               `json:"tile_id"`
	X       int               `json:"x"`
	Y       int               `json:"y"`
	Width   int               `json:"width"`
	Height  int               `json:"height"`
	Data    [][]color.RGBA    `json:"data"`
}

// ImageInfo holds metadata about an image being processed
type ImageInfo struct {
	ID            int       `json:"id"`
	InputPath     string    `json:"input_path"`
	OutputPath    string    `json:"output_path"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	ExpectedTiles int       `json:"expected_tiles"`
	LoadTime      time.Time `json:"load_time"`
	StartTime     time.Time `json:"start_time"`
}

// JobMessage represents a job in the Redis queue
type JobMessage struct {
	Type      string     `json:"type"` // "tile", "complete"
	ImageTile *ImageTile `json:"image_tile,omitempty"`
	ImageInfo *ImageInfo `json:"image_info,omitempty"`
}

// ResultMessage represents a processed result in Redis
type ResultMessage struct {
	ProcessedTile *ProcessedImageTile `json:"processed_tile"`
	WorkerID      string              `json:"worker_id"`
	ProcessTime   float64             `json:"process_time"`
}

// TimingData represents overall processing timing stored in Redis
type TimingData struct {
	StartTime       time.Time            `json:"start_time"`
	EndTime         *time.Time           `json:"end_time,omitempty"`
	KernelSize      int                  `json:"kernel_size"`
	TotalImages     int                  `json:"total_images"`
	InputPaths      []string             `json:"input_paths"`
	OutputPaths     []string             `json:"output_paths"`
	ImageStartTimes map[int]time.Time    `json:"image_start_times"`
	ImageEndTimes   map[int]*time.Time   `json:"image_end_times"`
}