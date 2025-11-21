# Video Processing Service

A YouTube-like video processing service with multi-resolution transcoding, real-time progress updates, and persistent storage.

## Features

- 📹 **Multi-Resolution Processing**: Automatically generates multiple resolutions (2160p, 1440p, 1080p, 720p, 480p, 360p, 240p, 144p)
- 🚀 **Real-Time Progress**: Server-Sent Events (SSE) for live progress updates
- 💾 **Database Persistence**: PostgreSQL for video metadata and resolutions
- 📡 **Message Queue**: RabbitMQ for job distribution
- 🔄 **Pub/Sub**: Redis for real-time notifications
- 📦 **Object Storage**: MinIO (S3-compatible) for video storage
- 🎬 **FFmpeg**: Industry-standard video transcoding

## Architecture

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Frontend  │─────▶│  API Server │─────▶│  RabbitMQ   │
└─────────────┘      └─────────────┘      └─────────────┘
       │                    │                      │
       │                    ▼                      ▼
       │             ┌─────────────┐      ┌─────────────┐
       │             │  PostgreSQL │      │   Worker    │
       │             └─────────────┘      └─────────────┘
       │                    │                      │
       │                    │                      ▼
       │                    │             ┌─────────────┐
       │                    │             │    MinIO    │
       │                    │             └─────────────┘
       │                    │                      │
       ▼                    ▼                      ▼
┌─────────────────────────────────────────────────────┐
│                      Redis (Pub/Sub)                 │
└─────────────────────────────────────────────────────┘
```

## API Endpoints

### POST /jobs
Submit a new video processing job.

**Request:**
```json
{
  "video_id": "unique-video-id",
  "s3_path": "http://minio:9000/videos/source/video.mp4",
  "original_filename": "my-video.mp4",
  "bucket": "videos"
}
```

**Response:**
```json
{
  "status": "queued",
  "id": "unique-video-id"
}
```

### GET /videos
List all videos with pagination.

**Query Parameters:**
- `limit` (default: 20)
- `offset` (default: 0)

**Response:**
```json
[
  {
    "id": "video-id",
    "original_name": "video.mp4",
    "s3_path": "http://...",
    "status": "completed",
    "source_height": 1080,
    "source_width": 1920,
    "duration": 120.5,
    "file_size": 1024000,
    "created_at": "2025-11-21T10:00:00Z",
    "updated_at": "2025-11-21T10:05:00Z",
    "completed_at": "2025-11-21T10:05:00Z",
    "resolutions": [
      {
        "id": "res-id",
        "video_id": "video-id",
        "height": 1080,
        "width": 1920,
        "s3_key": "processed/video-id_1080p.mp4",
        "s3_url": "http://...",
        "file_size": 512000,
        "bitrate": 4000000,
        "processed_at": "2025-11-21T10:05:00Z"
      }
    ]
  }
]
```

### GET /videos/{video_id}
Get details of a specific video.

### GET /progress/{video_id}
**Server-Sent Events (SSE)** - Real-time progress updates for a specific video.

**Event Data:**
```json
{
  "video_id": "unique-video-id",
  "status": "processing",
  "current_resolution": 720,
  "total_resolutions": 5,
  "progress": 45,
  "message": "Processing 720p (2/5)...",
  "timestamp": "2025-11-21T10:02:30Z"
}
```

### GET /progress
**Server-Sent Events (SSE)** - Real-time progress updates for all videos.

## Status Values

- `pending` - Job queued, not started
- `processing` - Currently transcoding
- `completed` - All resolutions generated successfully
- `failed` - Processing failed

## Running Locally

### Prerequisites
- Docker & Docker Compose
- FFmpeg (for local development)

### Start Services

```bash
docker-compose up -d
```

This starts:
- PostgreSQL (port 5432)
- Redis (port 6379)
- RabbitMQ (port 5672, management UI: 15672)
- MinIO (port 9000, console: 9001)
- API Server (port 8080)
- Worker

### Check Service Health

```bash
# API Health
curl http://localhost:8080/healthz

# RabbitMQ Management
open http://localhost:15672
# Login: guest / guest

# MinIO Console
open http://localhost:9001
# Login: minioadmin / minioadmin
```

### Submit a Test Job

```bash
# First, upload a video to MinIO
# Then submit a job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "video_id": "test-video-123",
    "s3_path": "http://minio:9000/videos/source/test.mp4",
    "original_filename": "test.mp4",
    "bucket": "videos"
  }'
```

### Monitor Progress (SSE)

```bash
# Watch progress for specific video
curl -N http://localhost:8080/progress/test-video-123

# Watch all progress
curl -N http://localhost:8080/progress
```

## Frontend Integration

### React Example

```javascript
// Submit job
const submitVideo = async (videoFile) => {
  // 1. Upload to S3 (MinIO)
  const formData = new FormData();
  formData.append('file', videoFile);
  
  const uploadRes = await fetch('http://localhost:9000/upload', {
    method: 'POST',
    body: formData
  });
  
  const { s3_path } = await uploadRes.json();
  
  // 2. Submit processing job
  const jobRes = await fetch('http://localhost:8080/jobs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      video_id: crypto.randomUUID(),
      s3_path: s3_path,
      original_filename: videoFile.name,
      bucket: 'videos'
    })
  });
  
  return await jobRes.json();
};

// Monitor progress
const watchProgress = (videoId, onProgress) => {
  const eventSource = new EventSource(
    `http://localhost:8080/progress/${videoId}`
  );
  
  eventSource.onmessage = (event) => {
    const progress = JSON.parse(event.data);
    onProgress(progress);
    
    // Close when done
    if (progress.status === 'completed' || progress.status === 'failed') {
      eventSource.close();
    }
  };
  
  eventSource.onerror = () => {
    eventSource.close();
  };
  
  return eventSource;
};

// Usage
const MyComponent = () => {
  const [progress, setProgress] = useState(null);
  
  const handleUpload = async (file) => {
    const job = await submitVideo(file);
    watchProgress(job.id, setProgress);
  };
  
  return (
    <div>
      <input type="file" onChange={(e) => handleUpload(e.target.files[0])} />
      {progress && (
        <div>
          <p>Status: {progress.status}</p>
          <p>Progress: {progress.progress}%</p>
          <p>{progress.message}</p>
        </div>
      )}
    </div>
  );
};
```

## Environment Variables

### API Server
- `RABBITMQ_URL` - RabbitMQ connection string
- `DB_HOST` - PostgreSQL host
- `DB_PORT` - PostgreSQL port
- `DB_USER` - PostgreSQL user
- `DB_PASSWORD` - PostgreSQL password
- `DB_NAME` - PostgreSQL database name
- `REDIS_ADDR` - Redis address

### Worker
- All API Server variables plus:
- `S3_ENDPOINT` - MinIO/S3 endpoint
- `S3_ACCESS_KEY` - MinIO/S3 access key
- `S3_SECRET_KEY` - MinIO/S3 secret key
- `S3_BUCKET` - Default bucket name

## Database Schema

### videos
- `id` - Primary key
- `original_name` - Original filename
- `s3_path` - Source file path
- `status` - Current status
- `source_height` - Original height
- `source_width` - Original width
- `duration` - Duration in seconds
- `file_size` - File size in bytes
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp
- `completed_at` - Completion timestamp (nullable)
- `error_message` - Error message if failed

### resolutions
- `id` - Primary key
- `video_id` - Foreign key to videos
- `height` - Resolution height
- `width` - Resolution width
- `s3_key` - Processed file key
- `s3_url` - Presigned URL (7 days)
- `file_size` - File size in bytes
- `bitrate` - Video bitrate
- `processed_at` - Processing timestamp

## Development

### Run API Server
```bash
cd server
RABBITMQ_URL=amqp://localhost:5672 \
DB_HOST=localhost \
REDIS_ADDR=localhost:6379 \
go run cmd/api/main.go
```

### Run Worker
```bash
cd server
RABBITMQ_URL=amqp://localhost:5672 \
S3_ENDPOINT=localhost:9000 \
S3_ACCESS_KEY=minioadmin \
S3_SECRET_KEY=minioadmin \
S3_BUCKET=videos \
DB_HOST=localhost \
REDIS_ADDR=localhost:6379 \
go run cmd/worker/main.go
```

## License

MIT
