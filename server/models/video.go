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
	ID                string            `json:"id" db:"id" gorm:"column:id;type:uuid;primaryKey"`
	OriginalName      string            `json:"original_name" db:"original_name" gorm:"column:original_name;type:varchar(255);not null"`
	S3Path            string            `json:"s3_path" db:"s3_path" gorm:"column:s3_path;type:text;not null"`
	Status            VideoStatus       `json:"status" db:"status" gorm:"column:status;type:varchar(32);not null"`
	SourceHeight      int               `json:"source_height" db:"source_height" gorm:"column:source_height;not null"`
	SourceWidth       int               `json:"source_width" db:"source_width" gorm:"column:source_width;not null"`
	Duration          float64           `json:"duration" db:"duration" gorm:"column:duration;type:double precision;not null"`
	FileSize          int64             `json:"file_size" db:"file_size" gorm:"column:file_size;type:bigint;not null"`
	MasterPlaylistKey string            `json:"master_playlist_key" db:"master_playlist_key" gorm:"column:master_playlist_key;type:text"`
	MasterPlaylistURL string            `json:"master_playlist_url" db:"master_playlist_url" gorm:"column:master_playlist_url;type:text"`
	CreatedAt         time.Time         `json:"created_at" db:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time         `json:"updated_at" db:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty" db:"completed_at" gorm:"column:completed_at"`
	ErrorMessage      string            `json:"error_message,omitempty" db:"error_message" gorm:"column:error_message;type:text"`
	Resolutions       []VideoResolution `json:"resolutions,omitempty" db:"-" gorm:"foreignKey:VideoID;references:ID;constraint:OnDelete:CASCADE"`
}

type VideoResolution struct {
	ID            string    `json:"id" db:"id" gorm:"column:id;type:uuid;primaryKey"`
	VideoID       string    `json:"video_id" db:"video_id" gorm:"column:video_id;type:uuid;not null;index"`
	Resolution    string    `json:"resolution" db:"resolution" gorm:"column:resolution;type:varchar(32);not null"`
	PlaylistS3Key string    `json:"playlist_s3_key" db:"playlist_s3_key" gorm:"column:playlist_s3_key;type:text;not null"`
	PlaylistURL   string    `json:"playlist_url" db:"playlist_url" gorm:"column:playlist_url;type:text;not null"`
	SegmentCount  int       `json:"segment_count" db:"segment_count" gorm:"column:segment_count;not null"`
	TotalSize     int64     `json:"total_size" db:"total_size" gorm:"column:total_size;type:bigint;not null"`
	Bandwidth     int       `json:"bandwidth" db:"bandwidth" gorm:"column:bandwidth;not null"`
	ProcessedAt   time.Time `json:"processed_at" db:"processed_at" gorm:"column:processed_at;autoCreateTime"`
}

type ProcessingProgress struct {
	VideoID           string      `json:"video_id" gorm:"column:video_id;type:uuid;index"`
	Status            VideoStatus `json:"status" gorm:"column:status;type:varchar(32);not null"`
	CurrentResolution int         `json:"current_resolution,omitempty" gorm:"column:current_resolution"`
	TotalResolutions  int         `json:"total_resolutions,omitempty" gorm:"column:total_resolutions"`
	Progress          int         `json:"progress" gorm:"column:progress;not null"`
	Message           string      `json:"message" gorm:"column:message;type:text"`
	Timestamp         time.Time   `json:"timestamp" gorm:"column:timestamp;autoCreateTime"`
}

type VideoJob struct {
	VideoID      string `json:"video_id"`
	S3Path       string `json:"s3_path"`
	OriginalName string `json:"original_name"`
}
