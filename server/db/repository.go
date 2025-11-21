package db

import (
	"database/sql"
	"time"

	"github.com/devrayat000/video-process/models"
	"github.com/google/uuid"
)

// CreateVideo inserts a new video record
func CreateVideo(video *models.Video) error {
	query := `
		INSERT INTO videos (id, original_name, s3_path, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := DB.Exec(query,
		video.ID,
		video.OriginalName,
		video.S3Path,
		video.Status,
		video.CreatedAt,
		video.UpdatedAt,
	)
	return err
}

// UpdateVideoStatus updates the status of a video
func UpdateVideoStatus(videoID string, status models.VideoStatus, errorMsg string) error {
	query := `
		UPDATE videos 
		SET status = $1, updated_at = $2, error_message = $3
		WHERE id = $4
	`
	_, err := DB.Exec(query, status, time.Now(), errorMsg, videoID)
	return err
}

// UpdateVideoMetadata updates video metadata after processing
func UpdateVideoMetadata(videoID string, width, height int, duration float64, fileSize int64) error {
	query := `
		UPDATE videos 
		SET source_width = $1, source_height = $2, duration = $3, file_size = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := DB.Exec(query, width, height, duration, fileSize, time.Now(), videoID)
	return err
}

// CompleteVideo marks a video as completed
func CompleteVideo(videoID string) error {
	now := time.Now()
	query := `
		UPDATE videos 
		SET status = $1, completed_at = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := DB.Exec(query, models.StatusCompleted, now, now, videoID)
	return err
}

// CreateResolution inserts a new resolution record
func CreateResolution(res *models.VideoResolution) error {
	if res.ID == "" {
		res.ID = uuid.New().String()
	}

	query := `
		INSERT INTO resolutions (id, video_id, resolution, playlist_s3_key, playlist_url, segment_count, total_size, bandwidth, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := DB.Exec(query,
		res.ID,
		res.VideoID,
		res.Resolution,
		res.PlaylistS3Key,
		res.PlaylistURL,
		res.SegmentCount,
		res.TotalSize,
		res.Bandwidth,
		res.ProcessedAt,
	)
	return err
}

// UpdateMasterPlaylist updates the master playlist URLs for a video
func UpdateMasterPlaylist(videoID, playlistKey, playlistURL string) error {
	query := `
		UPDATE videos 
		SET master_playlist_key = $1, master_playlist_url = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := DB.Exec(query, playlistKey, playlistURL, time.Now(), videoID)
	return err
}

// GetVideo retrieves a video by ID with its resolutions
func GetVideo(videoID string) (*models.Video, error) {
	query := `
		SELECT id, original_name, s3_path, status, source_height, source_width, 
		       duration, file_size, created_at, updated_at, completed_at, error_message
		FROM videos WHERE id = $1
	`

	video := &models.Video{}
	var completedAt sql.NullTime

	err := DB.QueryRow(query, videoID).Scan(
		&video.ID,
		&video.OriginalName,
		&video.S3Path,
		&video.Status,
		&video.SourceHeight,
		&video.SourceWidth,
		&video.Duration,
		&video.FileSize,
		&video.CreatedAt,
		&video.UpdatedAt,
		&completedAt,
		&video.ErrorMessage,
	)

	if err != nil {
		return nil, err
	}

	if completedAt.Valid {
		video.CompletedAt = &completedAt.Time
	}

	// Get resolutions
	resQuery := `
		SELECT id, video_id, resolution, playlist_s3_key, playlist_url, segment_count, total_size, bandwidth, processed_at
		FROM resolutions WHERE video_id = $1 ORDER BY bandwidth DESC
	`

	rows, err := DB.Query(resQuery, videoID)
	if err != nil {
		return video, nil // Return video even if resolutions query fails
	}
	defer rows.Close()

	for rows.Next() {
		var res models.VideoResolution
		if err := rows.Scan(&res.ID, &res.VideoID, &res.Resolution, &res.PlaylistS3Key, &res.PlaylistURL, &res.SegmentCount, &res.TotalSize, &res.Bandwidth, &res.ProcessedAt); err != nil {
			continue
		}
		video.Resolutions = append(video.Resolutions, res)
	}

	return video, nil
}

// ListVideos retrieves all videos with pagination
func ListVideos(limit, offset int) ([]*models.Video, error) {
	query := `
		SELECT id, original_name, s3_path, status, source_height, source_width, 
		       duration, file_size, created_at, updated_at, completed_at, error_message
		FROM videos ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`

	rows, err := DB.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		video := &models.Video{}
		var completedAt sql.NullTime

		if err := rows.Scan(
			&video.ID,
			&video.OriginalName,
			&video.S3Path,
			&video.Status,
			&video.SourceHeight,
			&video.SourceWidth,
			&video.Duration,
			&video.FileSize,
			&video.CreatedAt,
			&video.UpdatedAt,
			&completedAt,
			&video.ErrorMessage,
		); err != nil {
			continue
		}

		if completedAt.Valid {
			video.CompletedAt = &completedAt.Time
		}

		videos = append(videos, video)
	}

	return videos, nil
}
