package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/devrayat000/video-process/db"
	"github.com/devrayat000/video-process/models"
	"github.com/devrayat000/video-process/pubsub"
	server_utils "github.com/devrayat000/video-process/utils"
	"gorm.io/gorm"
)

// GCS configuration is resolved at runtime to keep API and worker consistent.
var (
	gcsPublicEndpoint = server_utils.GetEnv("GCS_PUBLIC_ENDPOINT", "https://storage.googleapis.com")
	gcsBucket         = server_utils.GetEnv("GCS_BUCKET_NAME", "")
)

func main() {
	// Initialize Database and Redis
	gormDB, err := db.InitDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	redis, err := pubsub.InitRedis()
	if err != nil {
		log.Fatal("Failed to initialize Redis:", err)
	}
	defer redis.Close()

	// Initialize GCS client (uses ADC / service account credentials)
	ctx := context.Background()
	gcsClient, err := server_utils.InitStorage(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer gcsClient.Close()

	// HTTP Handlers
	http.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var job models.VideoJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Create video record in database
		video := &models.Video{
			ID:           job.VideoID,
			OriginalName: job.OriginalName,
			S3Path:       job.S3Path,
			Status:       models.StatusWaiting,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := gorm.G[models.Video](gormDB).Create(r.Context(), video); err != nil {
			log.Printf("Failed to create video record: %s", err)
			http.Error(w, "Failed to create video record", http.StatusInternalServerError)
			return
		}

		// Enqueue job to Redis Stream
		if err := pubsub.EnqueueJob(job); err != nil {
			log.Printf("Failed to enqueue job: %s", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		log.Printf(" [x] Sent Job: %s", job.VideoID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "queued", "id": job.VideoID.String()})
	})

	// Get video details
	http.HandleFunc("/videos/", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		videoID := r.URL.Path[len("/videos/"):]
		if videoID == "" {
			http.Error(w, "Video ID required", http.StatusBadRequest)
			return
		}

		video, err := gorm.G[models.Video](gormDB).Where("id = ?", videoID).First(r.Context())
		if err != nil {
			http.Error(w, "Video not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&video)
	})

	// List all videos
	http.HandleFunc("/videos", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 20
		offset := 0

		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil {
				limit = l
			}
		}

		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil {
				offset = o
			}
		}

		videos, err := gorm.G[models.Video](gormDB).Limit(limit).Offset(offset).Find(r.Context())
		if err != nil {
			http.Error(w, "Failed to fetch videos", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(videos)
	})

	// SSE endpoint for real-time progress updates
	http.HandleFunc("/progress/", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		videoID := r.URL.Path[len("/progress/"):]
		if videoID == "" {
			http.Error(w, "Video ID required", http.StatusBadRequest)
			return
		}

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send current progress if available
		if progress, err := pubsub.GetProgress(videoID); err == nil {
			data, _ := json.Marshal(progress)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		// Subscribe to progress updates
		ctx := r.Context()
		progressChan, err := pubsub.SubscribeToProgress(ctx, videoID)
		if err != nil {
			http.Error(w, "Failed to subscribe", http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case progress := <-progressChan:
				if progress == nil {
					return
				}
				data, _ := json.Marshal(progress)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()

				// Close connection when completed or failed
				if progress.Status == models.StatusCompleted || progress.Status == models.StatusFailed {
					return
				}
			}
		}
	})

	// SSE endpoint for all progress updates
	http.HandleFunc("/progress", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Subscribe to all progress updates
		ctx := r.Context()
		progressChan, err := pubsub.SubscribeToAllProgress(ctx)
		if err != nil {
			http.Error(w, "Failed to subscribe", http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case progress := <-progressChan:
				if progress == nil {
					return
				}
				data, _ := json.Marshal(progress)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	// Initialize resumable upload to GCS
	http.HandleFunc("/upload/init", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Bucket      string `json:"bucket"`
			Key         string `json:"key"`
			ContentType string `json:"content_type"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.Key == "" {
			http.Error(w, "Key is required", http.StatusBadRequest)
			return
		}

		// Use default bucket if not provided
		bucket := req.Bucket
		if bucket == "" {
			bucket = gcsBucket
		}

		contentType := req.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Generate signed URL for resumable upload
		opts := &storage.SignedURLOptions{
			Scheme:      storage.SigningSchemeV4,
			Method:      "PUT",
			Expires:     time.Now().Add(24 * time.Hour),
			ContentType: contentType,
			Headers: []string{
				"x-goog-resumable:start",
			},
		}

		signedURL, err := gcsClient.Bucket(bucket).SignedURL(req.Key, opts)
		if err != nil {
			log.Printf("Failed to create upload signed URL: %v", err)
			http.Error(w, "Failed to create upload URL", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"upload_url": signedURL,
			"bucket":     bucket,
			"key":        req.Key,
			"method":     "signed",
		})
	})

	// Direct upload endpoint (proxy to GCS for CORS support)
	http.HandleFunc("/upload/direct", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" && r.Method != "PUT" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "Key query parameter is required", http.StatusBadRequest)
			return
		}

		bucket := r.URL.Query().Get("bucket")
		if bucket == "" {
			bucket = gcsBucket
		}

		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Upload to GCS
		ctx := r.Context()
		obj := gcsClient.Bucket(bucket).Object(key)
		writer := obj.NewWriter(ctx)
		writer.ContentType = contentType

		_, err := io.Copy(writer, r.Body)
		if err != nil {
			writer.Close()
			log.Printf("Failed to upload to GCS: %v", err)
			http.Error(w, "Upload failed", http.StatusInternalServerError)
			return
		}

		if err := writer.Close(); err != nil {
			log.Printf("Failed to close GCS writer: %v", err)
			http.Error(w, "Upload failed", http.StatusInternalServerError)
			return
		}

		// Return the public URL
		publicURL := fmt.Sprintf("%s/%s/%s", gcsPublicEndpoint, bucket, encodeKeyForURL(key))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url":    publicURL,
			"bucket": bucket,
			"key":    key,
		})
	})

	// Generate signed URL for downloading
	http.HandleFunc("/upload/signed-url", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "Key query parameter is required", http.StatusBadRequest)
			return
		}

		bucket := r.URL.Query().Get("bucket")
		if bucket == "" {
			bucket = gcsBucket
		}

		expiresIn := 3600 // 1 hour default
		if expiresStr := r.URL.Query().Get("expires"); expiresStr != "" {
			if e, err := strconv.Atoi(expiresStr); err == nil {
				expiresIn = e
			}
		}

		opts := &storage.SignedURLOptions{
			Scheme:  storage.SigningSchemeV4,
			Method:  "GET",
			Expires: time.Now().Add(time.Duration(expiresIn) * time.Second),
		}

		signedURL, err := gcsClient.Bucket(bucket).SignedURL(key, opts)
		if err != nil {
			log.Printf("Failed to create download signed URL: %v", err)
			http.Error(w, "Failed to create download URL", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url":    signedURL,
			"method": "signed",
		})
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "api_server"})
	})

	log.Println("API Server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func encodeKeyForURL(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
