# Video Processing Server

Backend source for both the HTTP API (`cmd/api`) and the worker (`cmd/worker`). The API handles job intake, metadata reads, and SSE fan-out. The worker consumes Redis Streams jobs, runs FFmpeg pipelines, uploads to MinIO, and records renditions in PostgreSQL.

## Requirements

- Go 1.22+
- FFmpeg + ffprobe on your `PATH`
- Access to PostgreSQL, Redis, and MinIO (start them via `docker-compose.basic.yaml`)

Bring up the infra services:

```bash
docker compose -f docker-compose.basic.yaml up -d
```

## Environment Variables

Place these in `server/.env` or export them manually.

| Variable | Purpose | Example |
| --- | --- | --- |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | PostgreSQL connection | `localhost` / `5555` / `user` / `password` / `videodb` |
| `REDIS_ADDR` | Redis host:port | `localhost:6379` |
| `REDIS_JOBS_STREAM` (optional) | Redis Stream name for jobs | `video:jobs` |
| `REDIS_PROGRESS_CHANNEL` (optional) | Pub/Sub channel for SSE updates | `video:progress:all` |
| `S3_ENDPOINT` | MinIO/S3 endpoint | `http://localhost:9000` |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | MinIO credentials | `minioadmin` / `minioadmin` |
| `S3_BUCKET` | Bucket for processed outputs | `videos` |
| `S3_USE_SSL` | `false` for local MinIO, `true` for AWS S3 | `false` |

Inside Docker, use service DNS names (`postgres`, `redis`, `minio`) instead of `localhost`.

## Install Dependencies

```bash
cd server
go mod download
```

## Running the Services

### API Server

```bash
cd server
go run cmd/api/main.go
```

### Worker

```bash
cd server
go run cmd/worker/main.go
```

Both commands read from the same `.env` file. Ensure the worker-specific S3 variables are present before starting.

## Verifying the Stack

1. `curl http://localhost:8080/healthz`
2. Submit a test job:

   ```bash
   curl -X POST http://localhost:8080/jobs \
     -H "Content-Type: application/json" \
     -d '{
       "video_id": "readme-test",
       "s3_path": "http://minio:9000/videos/source/test.mp4",
       "original_filename": "test.mp4",
       "bucket": "videos"
     }'
   ```

3. Watch worker logs for FFmpeg progress.
4. Stream progress: `curl -N http://localhost:8080/progress/readme-test`

## Troubleshooting

- **`dial tcp ... connection refused`** – verify `docker compose -f docker-compose.basic.yaml ps` shows Postgres, Redis, and MinIO as `Up`.
- **`XREADGROUP` errors** – confirm `REDIS_ADDR` points to the same instance for API and worker, and the stream exists (`redis-cli XLEN video:jobs`).
- **`no such host minio`** – set `S3_ENDPOINT=http://localhost:9000` when running outside Docker or update `/etc/hosts`.
- **FFmpeg missing** – install FFmpeg (`ffmpeg -version`) and restart the worker.
- **Schema mismatches** – reset the `videodb` database or run migrations before restarting services.
