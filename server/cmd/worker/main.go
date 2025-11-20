package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/devrayat000/video-process/utils"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Configuration
var (
	rabbitURL  = os.Getenv("RABBITMQ_URL")
	s3Endpoint = os.Getenv("S3_ENDPOINT")
	s3Access   = os.Getenv("S3_ACCESS_KEY")
	s3Secret   = os.Getenv("S3_SECRET_KEY")
	s3Bucket   = os.Getenv("S3_BUCKET")
)

func main() {
	// 1. Connect to MinIO
	minioClient, err := minio.New(s3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s3Access, s3Secret, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal("Failed to connect to MinIO:", err)
	}

	// 2. Connect to RabbitMQ (with retry logic for Docker startup)
	var conn *amqp.Connection
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("Worker waiting for RabbitMQ... (%s)", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ")
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"video_jobs", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Start Consumer
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (FALSE: we ack manually after success)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(" [*] Worker started. Ready to process streams.")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var job utils.VideoJob
			json.Unmarshal(d.Body, &job)

			log.Printf(" [x] Received Job: %s", job.VideoID)

			// Process the video using Streaming (No Disk I/O)
			err := processVideoStreaming(minioClient, job)

			if err != nil {
				log.Printf(" [!] Error processing %s: %v", job.VideoID, err)
				// Nack(multiple=false, requeue=true) - put it back in queue
				// Note: In production, use a Dead Letter Queue to avoid infinite loops
				d.Nack(false, true)
			} else {
				log.Printf(" [√] Done: %s", job.VideoID)
				d.Ack(false)
			}
		}
	}()

	<-forever
}

func processVideoStreaming(mc *minio.Client, job utils.VideoJob) error {
	// Set a timeout for the processing job
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// A. Stream DOWN from MinIO
	object, err := mc.GetObject(ctx, s3Bucket, job.S3Path, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("S3 download error: %w", err)
	}
	defer object.Close()

	// B. FFmpeg Command (Streaming Mode)
	// -i pipe:0                   Read from Stdin (the S3 stream)
	// -vf scale=1280:-2           Resize to 720p
	// -movflags frag_keyframe...  REQUIRED: Makes MP4 streamable/seekable without a second pass
	// -f mp4                      Force MP4 format
	// pipe:1                      Write to Stdout
	args := []string{
		"-i", "pipe:0",
		"-vf", "scale=1280:-2",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-movflags", "frag_keyframe+empty_moov",
		"-f", "mp4",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Wire inputs/outputs
	cmd.Stdin = object                    // Connect S3 Reader to FFmpeg Input
	cmd.Stderr = os.Stderr                // Redirect logs to Docker logs
	ffmpegStdout, err := cmd.StdoutPipe() // Create a pipe to read FFmpeg Output
	if err != nil {
		return err
	}

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start error: %w", err)
	}

	// C. Stream UP to MinIO
	// We read from ffmpegStdout and upload simultaneously.
	// We use -1 for size because the stream size is unknown until it finishes.
	processedKey := fmt.Sprintf("processed/%s_720p.mp4", job.VideoID)

	_, err = mc.PutObject(ctx, s3Bucket, processedKey, ffmpegStdout, -1, minio.PutObjectOptions{
		ContentType: "video/mp4",
	})
	if err != nil {
		return fmt.Errorf("S3 upload error: %w", err)
	}

	// Wait for FFmpeg process to exit cleanly
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg execution error: %w", err)
	}

	return nil
}
