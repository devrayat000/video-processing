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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/devrayat000/video-process/db"
	"github.com/devrayat000/video-process/models"
	"github.com/devrayat000/video-process/pubsub"
	server_utils "github.com/devrayat000/video-process/utils"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/gorm"
)

// Configuration
var (
	s3Endpoint = server_utils.GetEnv("S3_ENDPOINT", "localhost:9000")
	s3Access   = server_utils.GetEnv("S3_ACCESS_KEY", "minioadmin")
	s3Secret   = server_utils.GetEnv("S3_SECRET_KEY", "minioadmin")
	s3Bucket   = server_utils.GetEnv("S3_BUCKET", "videos")
)

// Rendition defines a single video quality preset
type Rendition struct {
	Height    int
	Bitrate   int // in kbps
	MaxRate   int // in kbps
	BufSize   int // in kbps
	AudioRate int // in kbps
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

	video := &models.Video{
		ID:     job.VideoID,
		S3Path: job.S3Path,
	}

	// Update status to processing
	_, err := gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
		Status:       models.StatusProcessing,
		ErrorMessage: nil,
	})
	if err != nil {
		log.Printf(" [!] Failed to update status: %v", err)
	}

	// Publish initial progress snapshot
	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:   job.VideoID,
		Status:    models.StatusStarted,
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

		errorMsg := fmt.Sprintf("Failed to read video metadata: %v", err)
		pubsub.PublishProgress(models.ProcessingProgress{
			VideoID:   job.VideoID,
			Status:    models.StatusFailed,
			Error:     errorMsg,
			Timestamp: time.Now(),
		})
		return fmt.Errorf("failed to get video metadata: %w", err)
	}
	log.Printf(" [i] Source video: %dx%d, duration: %.2fs", metadata.Width, metadata.Height, metadata.Duration)

	video = &models.Video{
		ID:           video.ID,
		S3Path:       video.S3Path,
		Frames:       metadata.Frames,
		SourceWidth:  metadata.Width,
		SourceHeight: metadata.Height,
		Duration:     metadata.Duration,
	}
	// Update video metadata in database
	_, err = gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, *video)
	if err != nil {
		log.Printf(" [!] Failed to update metadata: %v", err)
	}

	// Determine which renditions to generate
	renditions := filterRenditions(metadata.Height)
	log.Printf(" [i] Generating %d renditions: %v", len(renditions), getRenditionHeights(renditions))

	pubsub.PublishProgress(models.ProcessingProgress{
		VideoID:   job.VideoID,
		Status:    models.StatusProcessing,
		Timestamp: time.Now(),
	})

	// Transcode all renditions in a single FFmpeg command
	err = transcodeToHLSBatch(ctx, mc, gormDB, *video, renditions)
	if err != nil {
		errMsg := fmt.Sprintf("failed to transcode video: %v", err)
		gorm.G[models.Video](gormDB).Where("id = ?", job.VideoID).Updates(ctx, models.Video{
			Status:       models.StatusFailed,
			ErrorMessage: &errMsg,
		})
		pubsub.PublishProgress(models.ProcessingProgress{
			VideoID:   job.VideoID,
			Status:    models.StatusFailed,
			Error:     errMsg,
			Timestamp: time.Now(),
		})
		return fmt.Errorf("%s", errMsg)
	}

	log.Printf(" [√] Completed HLS transcoding for video_id=%s", job.VideoID)

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
		Timestamp: time.Now(),
	})

	log.Printf(" [√] All renditions completed for video_id=%s", job.VideoID)
	return nil
}

type VideoMetadata struct {
	Width    int
	Height   int
	Duration float64
	Bitrate  int
	Frames   int64
}

// getVideoMetadata uses ffprobe to extract video metadata
func getVideoMetadata(ctx context.Context, sourceURL string) (*VideoMetadata, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,bit_rate,nb_frames:format=duration",
		"-of", "default=noprint_wrappers=1",
		sourceURL,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe error: %w", err)
	}

	metadata := &VideoMetadata{}
	lines := strings.SplitSeq(string(output), "\n")

	for line := range lines {
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
		case "nb_frames":
			fmt.Sscanf(value, "%d", &metadata.Frames)
		}
	}

	if metadata.Height == 0 || metadata.Width == 0 {
		return nil, fmt.Errorf("failed to parse video dimensions")
	}

	return metadata, nil
}

var renditions = []Rendition{
	{Height: 2160, Bitrate: 16000, MaxRate: 17600, BufSize: 24000, AudioRate: 256}, // 4K UHD
	{Height: 1440, Bitrate: 9000, MaxRate: 9900, BufSize: 13500, AudioRate: 256},   // 2K QHD
	{Height: 1080, Bitrate: 5000, MaxRate: 5350, BufSize: 7500, AudioRate: 192},    // Full HD
	{Height: 720, Bitrate: 2800, MaxRate: 2996, BufSize: 4200, AudioRate: 160},     // HD
	{Height: 480, Bitrate: 1400, MaxRate: 1498, BufSize: 2100, AudioRate: 128},     // SD
	{Height: 360, Bitrate: 800, MaxRate: 856, BufSize: 1200, AudioRate: 128},       // Low
	{Height: 240, Bitrate: 500, MaxRate: 535, BufSize: 750, AudioRate: 96},         // Mobile
	{Height: 144, Bitrate: 300, MaxRate: 321, BufSize: 450, AudioRate: 96},         // Ultra Low
}

// filterRenditions selects renditions that don't exceed the source height
func filterRenditions(sourceHeight int) []Rendition {
	var selected []Rendition

	for _, r := range renditions {
		if r.Height <= sourceHeight {
			selected = append(selected, r)
		}
	}

	// If source is smaller than smallest preset, create a custom rendition
	if len(selected) == 0 {
		selected = []Rendition{
			{
				Height:    sourceHeight,
				Bitrate:   sourceHeight * 2,
				MaxRate:   sourceHeight*2 + 200,
				BufSize:   sourceHeight * 3,
				AudioRate: 96,
			},
		}
	}

	return selected
}

// getRenditionHeights returns a slice of heights for logging purposes
func getRenditionHeights(renditions []Rendition) []int {
	heights := make([]int, len(renditions))
	for i, r := range renditions {
		heights[i] = r.Height
	}
	return heights
}

// transcodeToHLSBatch transcodes all renditions in a single FFmpeg command
func transcodeToHLSBatch(ctx context.Context, mc *minio.Client, gormDB *gorm.DB, video models.Video, renditions []Rendition) error {
	// Create temporary directory for HLS output
	tempDir := fmt.Sprintf("/tmp/%s", video.ID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after upload

	splitCount := len(renditions)

	// -------- BUILD FILTER COMPLEX --------
	var filterParts []string

	// Split input into multiple streams
	splitOutputs := make([]string, splitCount)
	for i := 0; i < splitCount; i++ {
		splitOutputs[i] = fmt.Sprintf("[v%d]", i+1)
	}
	filterParts = append(filterParts, fmt.Sprintf("[0:v]split=%d%s", splitCount, strings.Join(splitOutputs, "")))

	// Scale each stream to target resolution
	for i, r := range renditions {
		filterParts = append(filterParts, fmt.Sprintf("[v%d]scale=-2:%d[v%dout]", i+1, r.Height, i+1))
	}

	filterComplex := strings.Join(filterParts, ";")

	// -------- BUILD FFMPEG ARGS --------
	args := []string{
		"-y",
		"-v", "error",
		"-fflags", "+discardcorrupt",
		"-i", video.S3Path,
		"-progress", "pipe:1",
		"-filter_complex", filterComplex,
	}

	// Add video maps for each rendition
	for i, r := range renditions {
		args = append(args,
			"-map", fmt.Sprintf("[v%dout]", i+1),
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-b:v:%d", i), fmt.Sprintf("%dk", r.Bitrate),
			fmt.Sprintf("-maxrate:v:%d", i), fmt.Sprintf("%dk", r.MaxRate),
			fmt.Sprintf("-bufsize:v:%d", i), fmt.Sprintf("%dk", r.BufSize),
			"-preset", "ultrafast",
			"-g", "48",
			"-keyint_min", "48",
			"-sc_threshold", "0",
		)
	}

	// Add audio maps for each rendition
	for i, r := range renditions {
		args = append(args,
			"-map", "a:0",
			fmt.Sprintf("-c:a:%d", i), "aac",
			fmt.Sprintf("-b:a:%d", i), fmt.Sprintf("%dk", r.AudioRate),
			"-ac", "2",
		)
	}

	// Build var_stream_map
	varStreamParts := make([]string, splitCount)
	for i := range renditions {
		varStreamParts[i] = fmt.Sprintf("v:%d,a:%d", i, i)
	}
	varStreamMap := strings.Join(varStreamParts, " ")

	// Add HLS output options
	args = append(args,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_type", "mpegts",
		"-hls_segment_filename", fmt.Sprintf("%s/stream_%%v/segment_%%03d.ts", tempDir),
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", varStreamMap,
		fmt.Sprintf("%s/stream_%%v/playlist.m3u8", tempDir),
	)

	// Execute FFmpeg
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	log.Printf(" [>] Running FFmpeg batch transcoding for %d renditions", splitCount)

	// Capture stderr for progress monitoring
	ffmpegStderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe error: %w", err)
	}

	// Monitor FFmpeg progress in background
	go monitorFFmpegProgressBatch(ffmpegStderr)

	ffmpegStdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe error: %w", err)
	}

	// Publish progress from stdout
	go func() {
		publishProgress(video, ffmpegStdout)
	}()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg execution error: %w", err)
	}

	log.Printf(" [√] FFmpeg transcoding completed for video_id=%s", video.ID)

	// -------- UPLOAD MASTER PLAYLIST FIRST --------
	masterPlaylistPath := fmt.Sprintf("%s/master.m3u8", tempDir)
	masterPlaylistKey := fmt.Sprintf("%s/processed/master.m3u8", video.ID)
	masterFile, err := os.Open(masterPlaylistPath)
	if err != nil {
		return fmt.Errorf("failed to open master playlist: %w", err)
	}

	masterFileInfo, err := masterFile.Stat()
	if err != nil {
		masterFile.Close()
		return fmt.Errorf("failed to stat master playlist: %w", err)
	}

	_, err = mc.PutObject(ctx, s3Bucket, masterPlaylistKey, masterFile, masterFileInfo.Size(), minio.PutObjectOptions{
		ContentType: "application/vnd.apple.mpegurl",
	})
	masterFile.Close()

	if err != nil {
		return fmt.Errorf("failed to upload master playlist: %w", err)
	}

	// Construct permanent S3 URL for master playlist
	masterURL := fmt.Sprintf("http://%s/%s/%s", s3Endpoint, s3Bucket, masterPlaylistKey)

	// Update video record with master playlist info
	gorm.G[models.Video](gormDB).Where("id = ?", video.ID).Updates(ctx, models.Video{
		MasterPlaylistKey: ptr(masterPlaylistKey),
		MasterPlaylistURL: ptr(masterURL),
	})

	log.Printf(" [√] Master playlist uploaded: %s", masterPlaylistKey)

	// -------- UPLOAD TO S3 --------
	for i, r := range renditions {
		streamDir := fmt.Sprintf("%s/stream_%d", tempDir, i)
		streamName := fmt.Sprintf("stream_%d", i) // Keep same structure as temp dir
		resolutionName := fmt.Sprintf("%dp", r.Height)

		// Count segments and calculate total size
		segmentFiles, err := os.ReadDir(streamDir)
		if err != nil {
			return fmt.Errorf("failed to read stream dir %s: %w", streamDir, err)
		}

		var totalSize int64
		segmentCount := 0

		// Upload segment files
		for _, file := range segmentFiles {
			if file.IsDir() {
				continue
			}

			filePath := fmt.Sprintf("%s/%s", streamDir, file.Name())
			s3Key := fmt.Sprintf("%s/processed/%s/%s", video.ID, streamName, file.Name())

			fileHandle, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", file.Name(), err)
			}

			fileInfo, err := fileHandle.Stat()
			if err != nil {
				fileHandle.Close()
				return fmt.Errorf("failed to stat file: %w", err)
			}

			contentType := "application/vnd.apple.mpegurl"
			if strings.HasSuffix(file.Name(), ".ts") {
				contentType = "video/mp2t"
				segmentCount++
			}

			uploadInfo, err := mc.PutObject(ctx, s3Bucket, s3Key, fileHandle, fileInfo.Size(), minio.PutObjectOptions{
				ContentType: contentType,
			})
			fileHandle.Close()

			if err != nil {
				return fmt.Errorf("S3 upload error for %s: %w", file.Name(), err)
			}

			totalSize += uploadInfo.Size
		}

		log.Printf(" [>] Uploaded %d segments for %s", segmentCount, resolutionName)

		// Construct permanent S3 URL for playlist
		playlistS3Key := fmt.Sprintf("%s/processed/%s/playlist.m3u8", video.ID, streamName)
		playlistURL := fmt.Sprintf("http://%s/%s/%s", s3Endpoint, s3Bucket, playlistS3Key)

		// Calculate bandwidth (convert kbps to bps)
		bandwidth := r.Bitrate * 1000

		// Save resolution to database
		resolution := &models.VideoResolution{
			ID:            uuid.New(),
			VideoID:       video.ID,
			Resolution:    resolutionName,
			PlaylistS3Key: playlistS3Key,
			PlaylistURL:   playlistURL,
			SegmentCount:  segmentCount,
			TotalSize:     totalSize,
			Bandwidth:     bandwidth,
			ProcessedAt:   time.Now(),
		}

		err = gorm.G[models.VideoResolution](gormDB).Create(ctx, resolution)
		if err != nil {
			log.Printf(" [!] Failed to save resolution to database: %v", err)
		}
	}

	return nil
}

func publishProgress(video models.Video, stdout io.ReadCloser) {
	defer stdout.Close()

	scanner := bufio.NewScanner(stdout)
	progressRegex := regexp.MustCompile(`frames=(\d+)`)

	for scanner.Scan() {
		// frames=1234
		line := scanner.Text()

		matches := progressRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			framesStr := matches[1]
			frames, err := strconv.ParseInt(framesStr, 10, 64)

			if err != nil {
				log.Printf("Error parsing frames: %v", err)
				continue
			}

			pubsub.PublishProgress(models.ProcessingProgress{
				VideoID:         video.ID,
				Status:          models.StatusProcessing,
				ProcessedFrames: frames,
				TotalFrames:     video.Frames,
				Timestamp:       time.Now(),
			})
		}
	}
}

func monitorFFmpegProgressBatch(stderr io.ReadCloser) {
	defer stderr.Close()

	scanner := bufio.NewScanner(stderr)
	progressRegex := regexp.MustCompile(`time=(\d+:\d+:\d+\.\d+)`)
	fpsRegex := regexp.MustCompile(`fps=\s*(\d+\.?\d*)`)

	for scanner.Scan() {
		line := scanner.Text()

		timeMatches := progressRegex.FindStringSubmatch(line)
		fpsMatches := fpsRegex.FindStringSubmatch(line)

		if len(timeMatches) > 1 {
			logMsg := fmt.Sprintf("FFmpeg progress: time=%s", timeMatches[1])
			if len(fpsMatches) > 1 {
				logMsg += fmt.Sprintf(", fps=%s", fpsMatches[1])
			}
			log.Printf(" [>] %s", logMsg)
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}
