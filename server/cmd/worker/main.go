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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/devrayat000/video-process/db"
	"github.com/devrayat000/video-process/models"
	"github.com/devrayat000/video-process/pubsub"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/gorm"
)

// Configuration
var (
	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3Access   = os.Getenv("S3_ACCESS_KEY")
	s3Secret   = os.Getenv("S3_SECRET_KEY")
	s3Bucket   = os.Getenv("S3_BUCKET")
)

// ResolutionInfo holds metadata about a processed resolution for master playlist generation
type ResolutionInfo struct {
	Resolution   string
	Bandwidth    int
	PlaylistKey  string
	SegmentCount int
	TotalSize    int64
}

func main() {
	// 0. Initialize Database and Redis
	gormDB, err := db.InitDB()
	if err != nil {
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
	err = pubsub.ConsumeJobs(ctx, func(job models.VideoJob) error {
		log.Printf(" [x] Received Job: id=%s source=%s", job.VideoID, job.S3Path)

		// Process the video
		err := processVideoStreaming(minioClient, gormDB, job)
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

func processVideoStreaming(mc *minio.Client, gormDB *gorm.DB, job models.VideoJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	log.Printf(" [>] Processing video_id=%s source=%s", job.VideoID, job.S3Path)

	// Update status to processing
	_, err := gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
		Status:       models.StatusProcessing,
		ErrorMessage: nil,
	})
	if err != nil {
		log.Printf(" [!] Failed to update status: %v", err)
	}

	// Publish initial progress
	message := "Starting video processing..."
	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:   job.VideoID,
		Status:    models.StatusProcessing,
		Progress:  0,
		Message:   &message,
		Timestamp: time.Now(),
	})

	// Get video metadata using ffprobe
	metadata, err := getVideoMetadata(ctx, job.S3Path)
	if err != nil {
		errorMessage := err.Error()
		gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
			Status:       models.StatusFailed,
			ErrorMessage: &errorMessage,
		})

		message := fmt.Sprintf("Failed to read video metadata: %v", err)
		pubsub.PublishProgress(models.ProcessingProgress{
			VideoID:   job.VideoID,
			Status:    models.StatusFailed,
			Progress:  0,
			Message:   &message,
			Timestamp: time.Now(),
		})
		return fmt.Errorf("failed to get video metadata: %w", err)
	}
	log.Printf(" [i] Source video: %dx%d, duration: %.2fs", metadata.Width, metadata.Height, metadata.Duration)

	// Update video metadata in database
	_, err = gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
		SourceWidth:  metadata.Width,
		SourceHeight: metadata.Height,
		Duration:     metadata.Duration,
	})
	if err != nil {
		log.Printf(" [!] Failed to update metadata: %v", err)
	}

	// Determine which resolutions to generate
	resolutions := getTargetResolutions(metadata.Height)
	log.Printf(" [i] Generating %d resolutions: %v", len(resolutions), resolutions)

	message = fmt.Sprintf("Processing %d resolutions...", len(resolutions))
	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:          job.VideoID,
		Status:           models.StatusProcessing,
		TotalResolutions: len(resolutions),
		Progress:         5,
		Message:          &message,
		Timestamp:        time.Now(),
	})

	// Track resolution info for master playlist
	var resolutionInfos []ResolutionInfo

	// Process each resolution
	for i, res := range resolutions {
		progressPercent := 5 + ((i * 90) / len(resolutions))

		message = fmt.Sprintf("Processing %dp (%d/%d)...", res, i+1, len(resolutions))
		pubsub.PublishProgress(models.ProcessingProgress{
			VideoID:           job.VideoID,
			Status:            models.StatusProcessing,
			CurrentResolution: res,
			TotalResolutions:  len(resolutions),
			Progress:          progressPercent,
			Message:           &message,
			Timestamp:         time.Now(),
		})

		resInfo, err := transcodeToHLS(ctx, mc, gormDB, job, res, metadata.Width, metadata.Height)
		if err != nil {
			errMsg := fmt.Sprintf("failed to transcode %dp: %v", res, err)
			gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
				Status:       models.StatusFailed,
				ErrorMessage: &errMsg,
			})
			pubsub.PublishProgress(models.ProcessingProgress{
				VideoID:   job.VideoID,
				Status:    models.StatusFailed,
				Progress:  progressPercent,
				Message:   &errMsg,
				Timestamp: time.Now(),
			})
			return fmt.Errorf("%s", errMsg)
		}

		resolutionInfos = append(resolutionInfos, resInfo)
		log.Printf(" [√] Completed %dp HLS for video_id=%s", res, job.VideoID)
	}

	// Generate and upload master playlist
	if err := generateAndUploadMasterPlaylist(ctx, mc, gormDB, job.VideoID, resolutionInfos); err != nil {
		log.Printf(" [!] Failed to generate master playlist: %v", err)

		errorMessage := err.Error()
		gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
			Status:       models.StatusFailed,
			ErrorMessage: &errorMessage,
		})
		return err
	}

	// Mark as completed
	_, err = gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
		Status:      models.StatusCompleted,
		CompletedAt: ptr(time.Now()),
	})
	if err != nil {
		log.Printf(" [!] Failed to mark video as completed: %v", err)
	}

	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:   job.VideoID,
		Status:    models.StatusCompleted,
		Progress:  100,
		Message:   ptr("Processing completed successfully!"),
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

// transcodeToHLS transcodes a single resolution to HLS format
func transcodeToHLS(ctx context.Context, mc *minio.Client, gormDB *gorm.DB, job models.VideoJob, height, sourceWidth, sourceHeight int) (ResolutionInfo, error) {
	// Create temporary directory for HLS output
	tempDir := fmt.Sprintf("/tmp/hls_%s_%dp_%d", job.VideoID, height, time.Now().UnixNano())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return ResolutionInfo{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after upload

	playlistPath := fmt.Sprintf("%s/playlist.m3u8", tempDir)
	segmentPattern := fmt.Sprintf("%s/segment_%%03d.ts", tempDir)

	// Determine audio bitrate based on resolution
	audioBitrate := getAudioBitrate(height)

	args := []string{
		"-y",
		"-fflags", "+discardcorrupt",
		"-i", job.S3Path,
		"-vf", fmt.Sprintf("scale=-2:%d", height),
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", fmt.Sprintf("%dk", audioBitrate),
		"-f", "hls",
		"-hls_time", "10",
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", segmentPattern,
		"-hls_flags", "independent_segments",
		"-start_number", "0",
		playlistPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	log.Printf(" [>] Running: ffmpeg (transcoding to %dp HLS)", height)

	ffmpegStderr, err := cmd.StderrPipe()
	if err != nil {
		return ResolutionInfo{}, fmt.Errorf("stderr pipe error: %w", err)
	}

	// Monitor FFmpeg progress in background
	go monitorFFmpegProgress(ffmpegStderr, job.VideoID, height)

	if err := cmd.Run(); err != nil {
		return ResolutionInfo{}, fmt.Errorf("ffmpeg execution error: %w", err)
	}

	// Upload all segment files to S3
	segmentFiles, err := os.ReadDir(tempDir)
	if err != nil {
		return ResolutionInfo{}, fmt.Errorf("failed to read temp dir: %w", err)
	}

	var totalSize int64
	segmentCount := 0

	for _, file := range segmentFiles {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".ts") {
			continue
		}

		segmentPath := fmt.Sprintf("%s/%s", tempDir, file.Name())
		s3Key := fmt.Sprintf("%s/processed/%dp/%s", job.VideoID, height, file.Name())

		// Open and upload segment file
		segmentFile, err := os.Open(segmentPath)
		if err != nil {
			return ResolutionInfo{}, fmt.Errorf("failed to open segment %s: %w", file.Name(), err)
		}

		fileInfo, err := segmentFile.Stat()
		if err != nil {
			segmentFile.Close()
			return ResolutionInfo{}, fmt.Errorf("failed to stat segment: %w", err)
		}

		uploadInfo, err := mc.PutObject(ctx, s3Bucket, s3Key, segmentFile, fileInfo.Size(), minio.PutObjectOptions{
			ContentType: "video/mp2t",
		})
		segmentFile.Close()

		if err != nil {
			return ResolutionInfo{}, fmt.Errorf("S3 upload error for %s: %w", file.Name(), err)
		}

		totalSize += uploadInfo.Size
		segmentCount++
		log.Printf(" [>] Uploaded segment %s (%d bytes)", file.Name(), uploadInfo.Size)
	}

	// Upload playlist file
	playlistS3Key := fmt.Sprintf("%s/processed/%dp/playlist.m3u8", job.VideoID, height)
	playlistFile, err := os.Open(playlistPath)
	if err != nil {
		return ResolutionInfo{}, fmt.Errorf("failed to open playlist: %w", err)
	}
	defer playlistFile.Close()

	playlistInfo, err := playlistFile.Stat()
	if err != nil {
		return ResolutionInfo{}, fmt.Errorf("failed to stat playlist: %w", err)
	}

	_, err = mc.PutObject(ctx, s3Bucket, playlistS3Key, playlistFile, playlistInfo.Size(), minio.PutObjectOptions{
		ContentType: "application/vnd.apple.mpegurl",
	})
	if err != nil {
		return ResolutionInfo{}, fmt.Errorf("playlist upload error: %w", err)
	}

	totalSize += playlistInfo.Size()
	log.Printf(" [>] Uploaded playlist for %dp", height)

	// Generate presigned URL for playlist (valid for 7 days)
	presignedURL, err := mc.PresignedGetObject(ctx, s3Bucket, playlistS3Key, 7*24*time.Hour, nil)
	if err != nil {
		log.Printf(" [!] Failed to generate presigned URL: %v", err)
	}

	// Estimate bandwidth
	bandwidth := estimateBandwidth(height)

	// Save resolution to database
	resolution := &models.VideoResolution{
		ID:            uuid.New(),
		VideoID:       job.VideoID,
		Resolution:    fmt.Sprintf("%dp", height),
		PlaylistS3Key: playlistS3Key,
		PlaylistURL:   presignedURL.String(),
		SegmentCount:  segmentCount,
		TotalSize:     totalSize,
		Bandwidth:     bandwidth,
		ProcessedAt:   time.Now(),
	}

	err = gorm.G[models.VideoResolution](gormDB).Create(ctx, resolution)
	if err != nil {
		log.Printf(" [!] Failed to save resolution to database: %v", err)
	}

	return ResolutionInfo{
		Resolution:   fmt.Sprintf("%dp", height),
		Bandwidth:    bandwidth,
		PlaylistKey:  playlistS3Key,
		SegmentCount: segmentCount,
		TotalSize:    totalSize,
	}, nil
}

func estimateBandwidth(height int) int {
	bandwidthMap := map[int]int{
		2160: 8000000, // 8 Mbps for 4K
		1440: 6000000, // 6 Mbps for 1440p
		1080: 5000000, // 5 Mbps for 1080p
		720:  2800000, // 2.8 Mbps for 720p
		480:  1400000, // 1.4 Mbps for 480p
		360:  800000,  // 800 Kbps for 360p
		240:  500000,  // 500 Kbps for 240p
		144:  300000,  // 300 Kbps for 144p
	}

	if bw, exists := bandwidthMap[height]; exists {
		return bw
	}
	return height * 2500 // Estimate based on height
}

// getAudioBitrate returns the appropriate audio bitrate for a given resolution
// Following YouTube's standards: higher resolutions get better audio quality
func getAudioBitrate(height int) int {
	switch {
	case height >= 1080:
		return 192 // 192 kbps for 1080p and above
	case height >= 720:
		return 160 // 160 kbps for 720p
	case height >= 480:
		return 128 // 128 kbps for 480p
	default:
		return 96 // 96 kbps for 360p and below
	}
}

func generateAndUploadMasterPlaylist(ctx context.Context, mc *minio.Client, gormDB *gorm.DB, videoID uuid.UUID, resolutions []ResolutionInfo) error {
	// Sort resolutions by bandwidth descending
	sort.Slice(resolutions, func(i, j int) bool {
		return resolutions[i].Bandwidth > resolutions[j].Bandwidth
	})

	// Generate master playlist content
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-VERSION:3\n\n")

	for _, res := range resolutions {
		builder.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,NAME=\"%s\"\n",
			res.Bandwidth, res.Resolution,
		))
		builder.WriteString(fmt.Sprintf("%s/playlist.m3u8\n\n", res.Resolution))
	}

	// Upload master playlist
	masterKey := fmt.Sprintf("%s/processed/master.m3u8", videoID)
	content := builder.String()

	_, err := mc.PutObject(ctx, s3Bucket, masterKey,
		strings.NewReader(content), int64(len(content)),
		minio.PutObjectOptions{
			ContentType: "application/vnd.apple.mpegurl",
		})

	if err != nil {
		return fmt.Errorf("failed to upload master playlist: %w", err)
	}

	// Generate presigned URL
	masterURL, err := mc.PresignedGetObject(ctx, s3Bucket, masterKey, 7*24*time.Hour, nil)
	if err != nil {
		log.Printf(" [!] Failed to generate master playlist URL: %v", err)
	}

	// Update video record
	_, err = gorm.G[models.Video](gormDB).Where("id = ?", videoID).Updates(ctx, models.Video{
		MasterPlaylistKey: ptr(masterKey),
		MasterPlaylistURL: ptr(masterURL.String()),
	})
	if err != nil {
		log.Printf(" [!] Failed to update master playlist in DB: %v", err)
	}

	log.Printf(" [√] Master playlist uploaded: %s", masterKey)
	return nil
}

func monitorFFmpegProgress(stderr io.ReadCloser, videoID uuid.UUID, height int) {
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

func ptr[T any](v T) *T {
	return &v
}
