# Video Processing Architecture

## Overview

YouTube-like video processing system using Redis Streams for job queue, PostgreSQL for persistence, MinIO for storage, and FFmpeg for transcoding.

## Why Redis Instead of RabbitMQ?

**Single Service, Multiple Use Cases:**
- ✅ **Message Queue** - Redis Streams with consumer groups
- ✅ **Pub/Sub** - Real-time progress updates
- ✅ **Caching** - Video metadata and progress state
- ✅ **Atomic Operations** - Built-in support

**Benefits:**
- Reduced infrastructure complexity (3 services instead of 4)
- Lower memory footprint
- Unified monitoring and management
- Redis Streams provide durable, ordered message delivery
- Consumer groups ensure work distribution
- Automatic redelivery of unacknowledged messages

## System Components

### 1. Redis (Port 6379)
**Role:** Message queue, pub/sub, caching

**Streams:**
- `video:jobs` - Job queue for video processing
  - Consumer group: `video-workers`
  - Persistent, ordered delivery
  - Automatic retries for failed jobs

**Pub/Sub Channels:**
- `video:progress:{video_id}` - Per-video progress updates
- `video:progress:all` - Global progress feed

**Keys:**
- `progress:{video_id}` - Current progress state (24h TTL)

### 2. PostgreSQL (Port 5432)
**Role:** Persistent video metadata storage

**Tables:**
- `videos` - Source video information, status, metadata
- `resolutions` - Generated resolution details, URLs

### 3. MinIO (Ports 9000, 9001)
**Role:** S3-compatible object storage

**Buckets:**
- `videos/source/` - Original uploads
- `videos/processed/` - Transcoded resolutions

### 4. API Server (Port 8080)
**Role:** REST API and SSE endpoints

**Endpoints:**
- `POST /jobs` - Submit video processing job
- `GET /videos` - List all videos
- `GET /videos/{id}` - Get video details
- `GET /progress/{id}` - SSE stream for video progress
- `GET /progress` - SSE stream for all progress
- `GET /healthz` - Health check

### 5. Worker
**Role:** Consumes jobs from Redis, transcodes videos

**Process:**
1. Read job from Redis Stream (`XREADGROUP`)
2. Update video status to `processing`
3. Probe video metadata with `ffprobe`
4. Determine target resolutions (2160p → 144p)
5. Transcode each resolution with `ffmpeg`
6. Upload to MinIO via streaming
7. Save resolution metadata to PostgreSQL
8. Publish progress via Redis pub/sub
9. Acknowledge message (`XACK`)

## Data Flow

```
┌─────────────┐
│   Client    │
│  (Upload)   │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────┐
│  POST /jobs                              │
│  • Create video record (DB)              │
│  • Enqueue to Redis Stream               │
│  └─▶ XADD video:jobs                     │
└─────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────┐
│  Worker (ConsumeJobs)                    │
│  XREADGROUP video:jobs video-workers     │
└─────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────┐
│  Process Video                           │
│  1. ffprobe → metadata                   │
│  2. Determine resolutions                │
│  3. For each resolution:                 │
│     • ffmpeg transcode                   │
│     • Upload to MinIO                    │
│     • Save to PostgreSQL                 │
│     • PUBLISH progress update            │
│  4. XACK message                         │
└─────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────┐
│  Client (SSE /progress/{id})             │
│  ← SUBSCRIBE video:progress:{id}         │
│  ← Receive real-time updates             │
└─────────────────────────────────────────┘
```

## Redis Streams vs RabbitMQ

| Feature | Redis Streams | RabbitMQ |
|---------|--------------|----------|
| Persistence | ✅ AOF/RDB | ✅ Durable queues |
| Ordering | ✅ Guaranteed | ✅ Per queue |
| Consumer Groups | ✅ Built-in | ✅ Competing consumers |
| Retries | ✅ Automatic | ⚠️ Manual DLQ |
| Pub/Sub | ✅ Same service | ❌ Separate feature |
| Memory | ~50MB | ~200MB+ |
| Complexity | Low | Medium |
| Message Size | Limited | Unlimited |

## Scaling Considerations

### Horizontal Scaling
**Workers:**
- Multiple worker containers automatically join `video-workers` consumer group
- Redis distributes jobs among consumers
- Each job processed by exactly one worker

**API Servers:**
- Stateless design allows multiple instances
- Load balance with Nginx/HAProxy
- SSE connections distributed across instances

### Performance Tuning

**Redis:**
```bash
# Increase max memory
maxmemory 4gb
maxmemory-policy allkeys-lru

# Enable AOF for durability
appendonly yes
appendfsync everysec
```

**Worker:**
```yaml
# Scale workers
docker-compose up --scale worker=5
```

**FFmpeg:**
```bash
# Adjust preset for speed vs quality
-preset ultrafast   # Fastest, lower quality
-preset veryfast    # Current setting
-preset medium      # Better quality, slower
```

## Monitoring

### Redis CLI
```bash
# Monitor stream length
redis-cli XLEN video:jobs

# Check pending messages
redis-cli XPENDING video:jobs video-workers

# Monitor pub/sub
redis-cli PSUBSCRIBE 'video:progress:*'
```

### Database Queries
```sql
-- Active jobs
SELECT COUNT(*) FROM videos WHERE status = 'processing';

-- Failed jobs
SELECT id, error_message FROM videos WHERE status = 'failed';

-- Average processing time
SELECT AVG(EXTRACT(EPOCH FROM (completed_at - created_at)))
FROM videos WHERE status = 'completed';
```

### MinIO
```bash
# Check bucket size
mc du local/videos

# List recent uploads
mc ls --recursive local/videos/processed/
```

## Error Handling

### Worker Failures
- Jobs remain in Redis Stream pending list
- Unacknowledged messages auto-retry
- Dead letter handling via XCLAIM for stuck jobs

### Database Failures
- Worker marks video as `failed` with error message
- Frontend displays error to user
- Manual retry via re-enqueuing job

### MinIO Failures
- Worker retries upload with exponential backoff
- Partial uploads cleaned via MinIO lifecycle policies

## Security

**Redis:**
- Enable AUTH with password
- Use TLS for production
- Network isolation via Docker networks

**PostgreSQL:**
- Strong password
- Limited user permissions
- Connection pooling with pgBouncer

**MinIO:**
- Pre-signed URLs with expiration
- Bucket policies for access control
- TLS encryption

## Development vs Production

**Development (docker-compose.yaml):**
- All services on single host
- No TLS
- Default credentials
- Persistent volumes

**Production:**
- Managed Redis (AWS ElastiCache, Redis Cloud)
- Managed PostgreSQL (RDS, Cloud SQL)
- S3 instead of MinIO
- Kubernetes for orchestration
- Horizontal Pod Autoscaling
- Monitoring (Prometheus, Grafana)
- Logging (ELK stack)

## Next Steps

1. **Frontend Development**
   - React/Vue.js upload interface
   - Video player with resolution selector
   - Real-time progress bars

2. **Advanced Features**
   - HLS/DASH adaptive streaming
   - Thumbnail generation
   - Subtitle support
   - CDN integration

3. **Production Hardening**
   - Rate limiting
   - Authentication/Authorization
   - Webhook notifications
   - Detailed analytics
