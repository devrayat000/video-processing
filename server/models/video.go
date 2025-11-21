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
	ID                string            `json:"id" db:"id"`
	OriginalName      string            `json:"original_name" db:"original_name"`
	S3Path            string            `json:"s3_path" db:"s3_path"`
	Status            VideoStatus       `json:"status" db:"status"`
	SourceHeight      int               `json:"source_height" db:"source_height"`
	SourceWidth       int               `json:"source_width" db:"source_width"`
	Duration          float64           `json:"duration" db:"duration"`
	FileSize          int64             `json:"file_size" db:"file_size"`
	MasterPlaylistKey string            `json:"master_playlist_key" db:"master_playlist_key"`
	MasterPlaylistURL string            `json:"master_playlist_url" db:"master_playlist_url"`
	CreatedAt         time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at" db:"updated_at"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty" db:"completed_at"`
	ErrorMessage      string            `json:"error_message,omitempty" db:"error_message"`
	Resolutions       []VideoResolution `json:"resolutions,omitempty" db:"-"`
}

type VideoResolution struct {
	ID            string    `json:"id" db:"id"`
	VideoID       string    `json:"video_id" db:"video_id"`
	Resolution    string    `json:"resolution" db:"resolution"`
	PlaylistS3Key string    `json:"playlist_s3_key" db:"playlist_s3_key"`
	PlaylistURL   string    `json:"playlist_url" db:"playlist_url"`
	SegmentCount  int       `json:"segment_count" db:"segment_count"`
	TotalSize     int64     `json:"total_size" db:"total_size"`
	Bandwidth     int       `json:"bandwidth" db:"bandwidth"`
	ProcessedAt   time.Time `json:"processed_at" db:"processed_at"`
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

type VideoJob struct {
	VideoID      string `json:"video_id"`
	S3Path       string `json:"s3_path"`
	OriginalName string `json:"original_name"`
}
