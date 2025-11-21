package models

import "time"

type VideoStatus string

const (
	StatusPending    VideoStatus = "pending"
	StatusProcessing VideoStatus = "processing"
	StatusCompleted  VideoStatus = "completed"
	StatusFailed     VideoStatus = "failed"
)

type Video struct {
	ID           string       `json:"id" db:"id"`
	OriginalName string       `json:"original_name" db:"original_name"`
	S3Path       string       `json:"s3_path" db:"s3_path"`
	Status       VideoStatus  `json:"status" db:"status"`
	SourceHeight int          `json:"source_height" db:"source_height"`
	SourceWidth  int          `json:"source_width" db:"source_width"`
	Duration     float64      `json:"duration" db:"duration"`
	FileSize     int64        `json:"file_size" db:"file_size"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty" db:"completed_at"`
	ErrorMessage string       `json:"error_message,omitempty" db:"error_message"`
	Resolutions  []Resolution `json:"resolutions,omitempty" db:"-"`
}

type Resolution struct {
	ID          string    `json:"id" db:"id"`
	VideoID     string    `json:"video_id" db:"video_id"`
	Height      int       `json:"height" db:"height"`
	Width       int       `json:"width" db:"width"`
	S3Key       string    `json:"s3_key" db:"s3_key"`
	S3URL       string    `json:"s3_url" db:"s3_url"`
	FileSize    int64     `json:"file_size" db:"file_size"`
	Bitrate     int       `json:"bitrate" db:"bitrate"`
	ProcessedAt time.Time `json:"processed_at" db:"processed_at"`
}

type ProcessingProgress struct {
	VideoID           string      `json:"video_id"`
	Status            VideoStatus `json:"status"`
	CurrentResolution int         `json:"current_resolution,omitempty"`
	TotalResolutions  int         `json:"total_resolutions,omitempty"`
	Progress          int         `json:"progress"` // 0-100
	Message           string      `json:"message"`
	Timestamp         time.Time   `json:"timestamp"`
}
