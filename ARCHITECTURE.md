# Video Processing Architecture

## Overview

YouTube-like video processing system using Redis Streams for job orchestration, PostgreSQL for persistence, MinIO for storage, and FFmpeg for transcoding.

## Why Redis Instead of RabbitMQ?

### Single Service, Multiple Use Cases

- ✅ **Message Queue** – Redis Streams with consumer groups
- ✅ **Pub/Sub** – Real-time progress updates
- ✅ **Caching** – Video metadata and progress state
- ✅ **Atomic Operations** – Built-in support

### Benefits

- Reduced infrastructure complexity (3 services instead of 4)
- Lower memory footprint
- Unified monitoring and management
- Durable, ordered message delivery
- Automatic redelivery of unacknowledged messages

## System Components

### 1. Redis (Port 6379)

**Role:** Message queue, pub/sub, caching

#### Streams

- `video:jobs` – Durable job queue
  - Consumer group: `video-workers`
  - Guarantees ordered delivery
  - Includes automatic retry via pending entries

#### Pub/Sub Channels

- `video:progress:{video_id}` – Per-video progress updates
- `video:progress:all` – Global progress feed used by dashboards

#### Keys

- `progress:{video_id}` – Current progress snapshot (24h TTL)

### 2. PostgreSQL (Port 5432 / Host 5555)

**Role:** Persistent video metadata storage

**Compose defaults:** database `videodb`, user `user`, password `password` (see `docker-compose.basic.yaml`).

#### Tables

- `videos` – Source video information, status, metadata
- `resolutions` – Generated resolution details, URLs

### 3. MinIO (Ports 9000, 9001)

**Role:** S3-compatible object storage

**Compose defaults:** `minioadmin / minioadmin` credentials; console exposed on `http://localhost:9001`.

#### Buckets

- `videos/source/` – Original uploads
- `videos/processed/` – Transcoded resolutions

### 4. API Server (Port 8080)

**Role:** REST API and SSE endpoints

#### Endpoints

- `POST /jobs` – Submit video processing job
- `GET /videos` – List all videos
- `GET /videos/{id}` – Get video details
- `GET /progress/{id}` – SSE stream for video progress
- `GET /progress` – SSE stream for all progress
- `GET /healthz` – Health check

### 5. Worker

**Role:** Consumes jobs from Redis, transcodes videos

#### Process

1. Read job from Redis Stream (`XREADGROUP`)
2. Update video status to `processing`
3. Probe video metadata with `ffprobe`
4. Determine target resolutions (2160p → 144p)
5. Transcode each resolution with `ffmpeg`
6. Upload to MinIO via streaming
7. Save resolution metadata to PostgreSQL
8. Publish progress via Redis pub/sub
9. Acknowledge message (`XACK`)

### Docker Compose Layout

- `docker-compose.basic.yaml` – Infrastructure only (PostgreSQL, Redis, MinIO, bucket bootstrapper). Creates/uses the `devcontainer_default` bridge network and exposes Postgres on host `5555`, Redis on `6379`, and MinIO on `9000/9001`.
- `docker-compose.yaml` – API + worker images. Combine with the base file via `docker compose -f docker-compose.basic.yaml -f docker-compose.yaml up -d --build`.
- `client/.env` – Points to `host.docker.internal` so the Vite dev server can call the containers while running in the host browser.

## Data Flow

```text
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
| --- | --- | --- |
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

#### Workers

- Multiple worker containers automatically join `video-workers`
- Redis balances work across the consumer group
- Each job is processed exactly once before `XACK`

#### API Servers

- Stateless handlers allow horizontal scaling behind Nginx/HAProxy
- SSE connections can be fanned out via sticky sessions or shared Redis subscriptions

### Performance Tuning

#### Redis

```bash
# Increase maxmemory
maxmemory 4gb
maxmemory-policy allkeys-lru

# Enable AOF for durability
appendonly yes
appendfsync everysec
```

#### Worker

```bash
# Scale workers
docker compose up --scale worker=5
```

#### FFmpeg

```bash
# Adjust preset for speed vs quality
-preset ultrafast   # Fastest, lower quality
-preset veryfast    # Current setting
-preset medium      # Better quality, slower
```

## Monitoring

### Redis CLI

```bash
# Stream length
redis-cli XLEN video:jobs

# Pending messages
redis-cli XPENDING video:jobs video-workers

# Pub/sub fan-out
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
# Bucket size
mc du local/videos

# Recent uploads
mc ls --recursive local/videos/processed/
```

## Error Handling

### Worker Failures

- Jobs remain in the Redis pending list
- `XCLAIM` enables dead-letter style recovery for stuck messages
- Automatic retries occur when workers restart

### Database Failures

- Worker marks video as `failed` with an error message
- API surfaces the error to the frontend
- Operators can re-enqueue by calling `POST /jobs` again

### MinIO Failures

- Worker retries uploads with exponential backoff
- Partial objects are cleaned via lifecycle policies
- For hard failures, the job is marked as `failed`

## Security

### Redis

- Enable AUTH with strong passwords
- Use TLS for production clusters
- Keep instances on private networks

### PostgreSQL

- Enforce strong credentials and least-privilege roles
- Consider pgBouncer for pooled connections
- Enable automated backups

### MinIO

- Use pre-signed URLs with short expirations
- Restrict bucket policies to required prefixes
- Enable TLS or place behind a TLS-terminating proxy

## Development vs Production

### Development (docker-compose)

- All services run on a single host
- Default credentials checked into `.env`
- No TLS, volumes persist between restarts

### Production

- Managed Redis (ElastiCache, Redis Cloud)
- Managed PostgreSQL (RDS, Cloud SQL)
- AWS S3 / GCS instead of MinIO
- Kubernetes for orchestration + HPA for autoscaling
- Centralized metrics (Prometheus/Grafana) and logging (ELK, Loki)

## Next Steps

1. **Frontend Enhancements** – richer analytics, adaptive streaming UI, operator alerts.
2. **Advanced Media Features** – HLS/DASH packaging, thumbnail sprites, subtitle ingestion.
3. **Production Hardening** – authentication/authorization, rate limiting, webhook notifications, SLA dashboards.
