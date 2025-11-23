package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/devrayat000/video-process/db"
	"github.com/devrayat000/video-process/models"
	"github.com/devrayat000/video-process/pubsub"
	"gorm.io/gorm"
)

func main() {
	// Initialize Database and Redis
	gormDB, err := db.InitDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	if err := pubsub.InitRedis(); err != nil {
		log.Fatal("Failed to initialize Redis:", err)
	}

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
