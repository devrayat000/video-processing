package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/devrayat000/video-process/models"
	server_utils "github.com/devrayat000/video-process/utils"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

const (
	VideoJobsStream   = "video:jobs"
	ConsumerGroup     = "video-workers"
	ProgressKeyPrefix = "progress:"
	ProgressChannel   = "video:progress:"
	ProgressAllChan   = "video:progress:all"
)

var (
	redisAddr    = server_utils.GetEnv("REDIS_ADDR", "localhost:6379")
	redisPass    = server_utils.GetEnv("REDIS_PASSWORD", "")
	ConsumerName = server_utils.GetEnv("HOSTNAME", "worker-1")
)

func InitRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPass,
		DB:       0,
	})

	ctx := context.Background()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("Redis connection established")

	// Create consumer group for video jobs stream (ignore error if already exists)
	err := RedisClient.XGroupCreateMkStream(ctx, VideoJobsStream, ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Printf("Warning: Failed to create consumer group: %v", err)
	}

	return nil
}

// EnqueueJob adds a video processing job to the Redis stream
func EnqueueJob(job models.VideoJob) error {
	ctx := context.Background()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	values := map[string]interface{}{
		"video_id":      job.VideoID,
		"s3_path":       job.S3Path,
		"original_name": job.OriginalName,
		"data":          string(data),
		"enqueued_at":   time.Now().Unix(),
	}

	_, err = RedisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: VideoJobsStream,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to add job to stream: %w", err)
	}

	log.Printf("Job enqueued: video_id=%s", job.VideoID)
	return nil
}

// ConsumeJobs reads jobs from the Redis stream and processes them
func ConsumeJobs(ctx context.Context, handler func(models.VideoJob) error) error {
	// First, process any pending messages from previous runs
	if err := processPendingMessages(ctx, handler); err != nil {
		log.Printf("Warning: Error processing pending messages: %v", err)
	}

	// Then start consuming new messages
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read from stream with consumer group
			streams, err := RedisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: ConsumerName,
				Streams:  []string{VideoJobsStream, ">"},
				Count:    1,
				Block:    5 * time.Second,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					// No new messages, continue
					continue
				}
				log.Printf("Error reading from stream: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					processMessage(ctx, message, handler)
				}
			}
		}
	}
}

// processPendingMessages handles messages that were enqueued but not yet processed
func processPendingMessages(ctx context.Context, handler func(models.VideoJob) error) error {
	log.Println("Checking for pending messages...")

	// Get pending messages for this consumer
	pending, err := RedisClient.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: VideoJobsStream,
		Group:  ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(pending) > 0 {
		log.Printf("Found %d pending messages, processing...", len(pending))
	}

	for _, p := range pending {
		// Claim the message for this consumer
		messages, err := RedisClient.XClaim(ctx, &redis.XClaimArgs{
			Stream:   VideoJobsStream,
			Group:    ConsumerGroup,
			Consumer: ConsumerName,
			MinIdle:  0,
			Messages: []string{p.ID},
		}).Result()

		if err != nil {
			log.Printf("Error claiming message %s: %v", p.ID, err)
			continue
		}

		for _, message := range messages {
			processMessage(ctx, message, handler)
		}
	}

	return nil
}

// processMessage processes a single message from the stream
func processMessage(ctx context.Context, message redis.XMessage, handler func(models.VideoJob) error) {
	job, err := parseJob(message.Values)
	if err != nil {
		log.Printf("Error parsing job: %v", err)
		// Acknowledge anyway to prevent reprocessing bad messages
		RedisClient.XAck(ctx, VideoJobsStream, ConsumerGroup, message.ID)
		return
	}

	log.Printf("Processing job: video_id=%s, message_id=%s", job.VideoID, message.ID)

	// Process the job
	if err := handler(job); err != nil {
		log.Printf("Error processing job %s: %v", job.VideoID, err)
		// Job will be retried on next worker restart or manual intervention
	} else {
		// Acknowledge successful processing
		RedisClient.XAck(ctx, VideoJobsStream, ConsumerGroup, message.ID)
		log.Printf("Job completed and acknowledged: video_id=%s", job.VideoID)
	}
}

// parseJob extracts VideoJob from Redis stream message
func parseJob(values map[string]interface{}) (models.VideoJob, error) {
	dataStr, ok := values["data"].(string)
	if !ok {
		return models.VideoJob{}, fmt.Errorf("missing or invalid data field")
	}

	var job models.VideoJob
	if err := json.Unmarshal([]byte(dataStr), &job); err != nil {
		return models.VideoJob{}, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return job, nil
}

func PublishProgress(progress models.ProcessingProgress) error {
	ctx := context.Background()

	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}

	// Publish to channel for real-time updates
	channel := fmt.Sprintf("%s%s", ProgressChannel, progress.VideoID)
	if err := RedisClient.Publish(ctx, channel, data).Err(); err != nil {
		return err
	}

	// Also publish to global channel
	if err := RedisClient.Publish(ctx, ProgressAllChan, data).Err(); err != nil {
		return err
	}

	// Store latest progress in Redis with TTL (24 hours)
	key := fmt.Sprintf("%s%s", ProgressKeyPrefix, progress.VideoID)
	if err := RedisClient.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
		return err
	}

	return nil
}

func GetProgress(videoID string) (*models.ProcessingProgress, error) {
	ctx := context.Background()
	key := fmt.Sprintf("%s%s", ProgressKeyPrefix, videoID)

	data, err := RedisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var progress models.ProcessingProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, err
	}

	return &progress, nil
}

func SubscribeToProgress(ctx context.Context, videoID string) (<-chan *models.ProcessingProgress, error) {
	channel := fmt.Sprintf("%s%s", ProgressChannel, videoID)
	pubsub := RedisClient.Subscribe(ctx, channel)

	progressChan := make(chan *models.ProcessingProgress)

	go func() {
		defer close(progressChan)
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-pubsub.Channel():
				var progress models.ProcessingProgress
				if err := json.Unmarshal([]byte(msg.Payload), &progress); err != nil {
					log.Printf("Error unmarshaling progress: %v", err)
					continue
				}
				progressChan <- &progress
			}
		}
	}()

	return progressChan, nil
}

func SubscribeToAllProgress(ctx context.Context) (<-chan *models.ProcessingProgress, error) {
	pubsub := RedisClient.Subscribe(ctx, ProgressAllChan)

	progressChan := make(chan *models.ProcessingProgress)

	go func() {
		defer close(progressChan)
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-pubsub.Channel():
				var progress models.ProcessingProgress
				if err := json.Unmarshal([]byte(msg.Payload), &progress); err != nil {
					log.Printf("Error unmarshaling progress: %v", err)
					continue
				}
				progressChan <- &progress
			}
		}
	}()

	return progressChan, nil
}
