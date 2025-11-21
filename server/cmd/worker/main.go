package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/devrayat000/video-process/db"
	"github.com/devrayat000/video-process/models"
	"github.com/devrayat000/video-process/pubsub"
	"github.com/devrayat000/video-process/utils"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Configuration
var (
	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3Access   = os.Getenv("S3_ACCESS_KEY")
	s3Secret   = os.Getenv("S3_SECRET_KEY")
	s3Bucket   = os.Getenv("S3_BUCKET")
)

func main() {
	// 0. Initialize Database and Redis
	if err := db.InitDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	if err := pubsub.InitRedis(); err != nil {
		log.Fatal("Failed to initialize Redis:", err)
	}

	// 1. Connect to MinIO
	minioClient, err := minio.New(s3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s3Access, s3Secret, ""),
		Secure: false,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 128,
			IdleConnTimeout:     90 * time.Second,
		},
	})
	if err != nil {
		log.Fatal("Failed to connect to MinIO:", err)
	}

	// 2. Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping worker...")
		cancel()
	}()

	log.Println(" [*] Worker started. Ready to process videos from Redis Streams.")

	// 3. Start consuming jobs from Redis
	err = pubsub.ConsumeJobs(ctx, func(job utils.VideoJob) error {
		log.Printf(" [x] Received Job: id=%s source=%s", job.VideoID, job.S3Path)

		// Process the video
		err := processVideoStreaming(minioClient, job)
		if err != nil {
			log.Printf(" [!] Error processing %s: %v", job.VideoID, err)
			return err
		}

		log.Printf(" [√] Done: %s", job.VideoID)
		return nil
	})

	if err != nil && err != context.Canceled {
		log.Fatal("Worker error:", err)
	}

	log.Println("Worker stopped gracefully")
}

func processVideoStreaming(mc *minio.Client, job utils.VideoJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	log.Printf(" [>] Processing video_id=%s source=%s", job.VideoID, job.S3Path)

	// Update status to processing
	if err := db.UpdateVideoStatus(job.VideoID, models.StatusProcessing, ""); err != nil {
		log.Printf(" [!] Failed to update status: %v", err)
	}

	// Publish initial progress
	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:   job.VideoID,
		Status:    models.StatusProcessing,
		Progress:  0,
		Message:   "Starting video processing...",
		Timestamp: time.Now(),
	})

	// Get video metadata using ffprobe
	metadata, err := getVideoMetadata(ctx, job.S3Path)
	if err != nil {
		db.UpdateVideoStatus(job.VideoID, models.StatusFailed, err.Error())
		pubsub.PublishProgress(models.ProcessingProgress{
			VideoID:   job.VideoID,
			Status:    models.StatusFailed,
			Progress:  0,
			Message:   fmt.Sprintf("Failed to read video metadata: %v", err),
			Timestamp: time.Now(),
		})
		return fmt.Errorf("failed to get video metadata: %w", err)
	}
	log.Printf(" [i] Source video: %dx%d, duration: %.2fs", metadata.Width, metadata.Height, metadata.Duration)

	// Update video metadata in database
	if err := db.UpdateVideoMetadata(job.VideoID, metadata.Width, metadata.Height, metadata.Duration, 0); err != nil {
		log.Printf(" [!] Failed to update metadata: %v", err)
	}

	// Determine which resolutions to generate
	resolutions := getTargetResolutions(metadata.Height)
	log.Printf(" [i] Generating %d resolutions: %v", len(resolutions), resolutions)

	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:          job.VideoID,
		Status:           models.StatusProcessing,
		TotalResolutions: len(resolutions),
		Progress:         5,
		Message:          fmt.Sprintf("Processing %d resolutions...", len(resolutions)),
		Timestamp:        time.Now(),
	})

	// Process each resolution
	for i, res := range resolutions {
		progressPercent := 5 + ((i * 90) / len(resolutions))

		pubsub.PublishProgress(models.ProcessingProgress{
			VideoID:           job.VideoID,
			Status:            models.StatusProcessing,
			CurrentResolution: res,
			TotalResolutions:  len(resolutions),
			Progress:          progressPercent,
			Message:           fmt.Sprintf("Processing %dp (%d/%d)...", res, i+1, len(resolutions)),
			Timestamp:         time.Now(),
		})

		if err := transcodeResolution(ctx, mc, job, res, metadata.Width); err != nil {
			errMsg := fmt.Sprintf("failed to transcode %dp: %v", res, err)
			db.UpdateVideoStatus(job.VideoID, models.StatusFailed, errMsg)
			pubsub.PublishProgress(models.ProcessingProgress{
				VideoID:   job.VideoID,
				Status:    models.StatusFailed,
				Progress:  progressPercent,
				Message:   errMsg,
				Timestamp: time.Now(),
			})
			return fmt.Errorf("%s", errMsg)
		}
		log.Printf(" [√] Completed %dp for video_id=%s", res, job.VideoID)
	}

	// Mark as completed
	if err := db.CompleteVideo(job.VideoID); err != nil {
		log.Printf(" [!] Failed to mark video as completed: %v", err)
	}

	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:   job.VideoID,
		Status:    models.StatusCompleted,
		Progress:  100,
		Message:   "Processing completed successfully!",
		Timestamp: time.Now(),
	})

	log.Printf(" [√] All resolutions completed for video_id=%s", job.VideoID)
	return nil
}

type VideoMetadata struct {
	Width    int
	Height   int
	Duration float64
	Bitrate  int
}

// getVideoMetadata uses ffprobe to extract video metadata
func getVideoMetadata(ctx context.Context, sourceURL string) (*VideoMetadata, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,bit_rate:format=duration",
		"-of", "default=noprint_wrappers=1",
		sourceURL,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe error: %w", err)
	}

	metadata := &VideoMetadata{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "width":
			fmt.Sscanf(value, "%d", &metadata.Width)
		case "height":
			fmt.Sscanf(value, "%d", &metadata.Height)
		case "duration":
			fmt.Sscanf(value, "%f", &metadata.Duration)
		case "bit_rate":
			fmt.Sscanf(value, "%d", &metadata.Bitrate)
		}
	}

	if metadata.Height == 0 || metadata.Width == 0 {
		return nil, fmt.Errorf("failed to parse video dimensions")
	}

	return metadata, nil
}

// getTargetResolutions returns the list of resolutions to generate
// Standard resolutions: 2160p, 1440p, 1080p, 720p, 480p, 360p, 240p, 144p
func getTargetResolutions(sourceHeight int) []int {
	standardResolutions := []int{2160, 1440, 1080, 720, 480, 360, 240, 144}

	var targets []int

	// Find the highest standard resolution that doesn't exceed source height
	startIdx := -1
	for i, res := range standardResolutions {
		if res <= sourceHeight {
			startIdx = i
			break
		}
	}

	// If source is smaller than 144p, just return the source height
	if startIdx == -1 {
		return []int{sourceHeight}
	}

	// Generate all resolutions from the starting point down to 144p
	targets = standardResolutions[startIdx:]

	return targets
}

// transcodeResolution transcodes a single resolution
func transcodeResolution(ctx context.Context, mc *minio.Client, job utils.VideoJob, height, sourceWidth int) error {
	// Calculate width maintaining aspect ratio (will be handled by FFmpeg's scale=-2)
	args := []string{
		"-y",
		"-fflags", "+discardcorrupt",
		"-i", job.S3Path,
		"-vf", fmt.Sprintf("scale=-2:%d", height),
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "frag_keyframe+empty_moov",
		"-f", "mp4",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	log.Printf(" [>] Running: ffmpeg (transcoding to %dp)", height)

	ffmpegStdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe error: %w", err)
	}

	ffmpegStderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe error: %w", err)
	}

	// Monitor FFmpeg progress in background
	go monitorFFmpegProgress(ffmpegStderr, job.VideoID, height)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start error: %w", err)
	}

	// Upload to MinIO
	processedKey := fmt.Sprintf("processed/%s_%dp.mp4", job.VideoID, height)
	log.Printf(" [>] Uploading to bucket=%s key=%s", s3Bucket, processedKey)

	uploadInfo, err := mc.PutObject(ctx, s3Bucket, processedKey, ffmpegStdout, -1, minio.PutObjectOptions{
		ContentType: "video/mp4",
	})
	if err != nil {
		return fmt.Errorf("S3 upload error: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg execution error: %w", err)
	}

	// Generate presigned URL (valid for 7 days)
	presignedURL, err := mc.PresignedGetObject(ctx, s3Bucket, processedKey, 7*24*time.Hour, nil)
	if err != nil {
		log.Printf(" [!] Failed to generate presigned URL: %v", err)
	}

	// Calculate actual width from aspect ratio
	calculatedWidth := (sourceWidth * height) / sourceWidth
	if calculatedWidth%2 != 0 {
		calculatedWidth--
	}

	// Save resolution to database
	resolution := &models.Resolution{
		ID:          uuid.New().String(),
		VideoID:     job.VideoID,
		Height:      height,
		Width:       calculatedWidth,
		S3Key:       processedKey,
		S3URL:       presignedURL.String(),
		FileSize:    uploadInfo.Size,
		ProcessedAt: time.Now(),
	}

	if err := db.CreateResolution(resolution); err != nil {
		log.Printf(" [!] Failed to save resolution to database: %v", err)
	}

	return nil
}

func monitorFFmpegProgress(stderr io.ReadCloser, videoID string, height int) {
	defer stderr.Close()

	scanner := bufio.NewScanner(stderr)
	progressRegex := regexp.MustCompile(`time=(\d+:\d+:\d+\.\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := progressRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			log.Printf(" [>] FFmpeg progress for %dp: %s", height, matches[1])
		}
	}
}
