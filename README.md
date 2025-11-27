# Video Processing Service

A YouTube-like transcoding pipeline with multi-resolution FFmpeg processing, Redis-backed job orchestration, and a Vite dashboard for operators.

## Features

- 📹 **Adaptive renditions** – Generates 2160p through 144p MP4 outputs per upload.
- 🚀 **Live telemetry** – Server-Sent Events stream job progress with per-resolution detail.
- 🧵 **Redis Streams queue** – Durable `video:jobs` stream + consumer groups replace RabbitMQ.
- 💾 **Persistent metadata** – PostgreSQL stores source files, renditions, and status history.
- ☁️ **Google Cloud Storage** – A single GCS bucket stores originals (`source/`) and processed outputs (`processed/`).
- 🖥️ **Operator console** – React/Vite UI to submit jobs, inspect metadata, and watch timelines.

## Architecture

```text
┌────────────┐    HTTP/SSE    ┌────────────┐
│   Client   │◀──────────────▶│   API      │
└────────────┘                │ (Go)       │
        ▲                     └────┬───────┘
        │                          │
        │                  XADD/XREADGROUP
        │                          │
        │                    ┌─────▼─────┐
        │                    │ Redis     │
        │                    │ Streams   │
        │                    └─────┬─────┘
        │                          │
        │                    ┌─────▼─────┐
        │                    │ Worker    │─┐
        │                    │ (Go)      │ │ uploads
        │                    └─────┬─────┘ │
        │                          │       ▼
        │                        Updates  GCS
        │                          │       ▲
        │                          ▼       │
        │                      PostgreSQL  │
        ▼                                  │
    Dashboard refreshes ◀──────────────────┘
```

See `ARCHITECTURE.md` for the in-depth design, Redis stream names, and scaling notes.

## Stack Components

- **API (`server/cmd/api`)** – REST + SSE endpoints, video metadata CRUD, progress fan-out.
- **Worker (`server/cmd/worker`)** – Consumes Redis Streams jobs, orchestrates FFmpeg + uploads, persists renditions.
- **Redis 7** – Provides `video:jobs` stream, consumer group `video-workers`, progress pub/sub, and caching.
- **PostgreSQL 15** – Stores `videos` and `resolutions` tables. Exposed on host `localhost:5555` via `docker-compose.basic.yaml`.
- **Google Cloud Storage** – Dedicated bucket for originals and renditions. Configure service-account credentials via `GOOGLE_APPLICATION_CREDENTIALS`.
- **React client (`client`)** – Vite app for operators (dev server on `5173`).

## Local Development

### Prerequisites

- Docker Desktop with Compose v2
- Go 1.22+ (for running API/worker outside Docker)
- Node.js 18+ (for the React client)
- FFmpeg + ffprobe installed locally and on `PATH`
- Google Cloud project + service account with `storage.objects.{create,get}` and `iam.serviceAccounts.signBlob` permissions (store its JSON key locally or rely on ADC)
- Optional: `docker network create devcontainer_default` (only if the external network referenced in compose files does not exist yet)

### Option A – Compose Everything (recommended)

1. From the repo root, start every service defined across both compose files:

   ```bash
   docker compose -f docker-compose.basic.yaml -f docker-compose.yaml up -d --build
   ```

1. Confirm the stack:

- API health: `curl http://localhost:8080/healthz`
- Redis CLI: `redis-cli -h localhost -p 6379 PING`
- PostgreSQL: `psql postgresql://user:password@localhost:5555/videodb`
- GCS: `gsutil ls gs://$GCS_BUCKET_NAME` (requires `gcloud` auth)

1. When finished, stop everything:

   ```bash
   docker compose -f docker-compose.basic.yaml -f docker-compose.yaml down
   ```

### Option B – Manual API/Worker, Dockerized Infra

1. Launch infra only (Postgres, Redis):

   ```bash
   docker compose -f docker-compose.basic.yaml up -d
   ```

2. Copy an env template or export variables for the API/worker (see "Environment Variables").
3. Run the API from `server/`:

   ```bash
   cd server
   go run cmd/api/main.go
   ```

4. In another terminal, run the worker:

   ```bash
   cd server
   go run cmd/worker/main.go
   ```

5. Start the Vite client:

   ```bash
   cd client
   npm install
   npm run dev
   ```

6. Open the client URL (default `http://localhost:5173`) to submit jobs and monitor status.

## API Quick Reference

### POST /jobs

Submit a video for processing.

```json
{
  "video_id": "unique-video-id",
  "s3_path": "https://storage.googleapis.com/your-bucket/source/video.mp4",
  "original_filename": "my-video.mp4",
  "bucket": "videos"
}
```

Returns `{ "status": "queued", "id": "unique-video-id" }`.

### GET /videos

Returns paginated video records (default `limit=20`, `offset=0`) with embedded resolutions.

### GET /videos/{id}

Returns detailed metadata for one video, including renditions, file sizes, errors, and timestamps.

### GET /progress/{id}

SSE stream for a single video. Events resemble:

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

SSE fan-out of every job in flight (good for dashboards or alerting).

### Status Values

`pending`, `processing`, `completed`, `failed`.

## Useful Commands

```bash
# Submit a quick test job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "video_id": "test-video-123",
    "s3_path": "https://storage.googleapis.com/your-bucket/source/test.mp4",
    "original_filename": "test.mp4",
    "bucket": "videos"
  }'

# Watch progress via SSE
curl -N http://localhost:8080/progress/test-video-123
```

## Environment Variables

### Shared (API + Worker)

- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` – PostgreSQL connection (defaults: `postgres`, `5432`, etc.). When using the compose files, connect to `postgres:5432` inside Docker or `localhost:5555` from the host.
- `REDIS_ADDR` – Redis host:port (e.g., `redis:6379` in Docker, `localhost:6379` locally).
- `REDIS_PROGRESS_CHANNEL` / `REDIS_JOBS_STREAM` (optional) – Override the Redis pub/sub + stream names (defaults: `video:progress:*`, `video:jobs`).
- `GCS_BUCKET_NAME` – Target Google Cloud Storage bucket (required).
- `GCS_PUBLIC_ENDPOINT` – Base URL for serving playlists (defaults to `https://storage.googleapis.com`).
- `GOOGLE_APPLICATION_CREDENTIALS` – Path to the service-account JSON with `storage.objects.{get,create}` and `iam.serviceAccounts.signBlob` rights. When running locally, point to `~/.config/gcloud/application_default_credentials.json` or mount a key file into the container.

### Client (Vite)

Defined in `client/.env` or `.env.local`:

```bash
VITE_API_BASE_URL=http://localhost:8080
VITE_GCS_ENDPOINT=https://storage.googleapis.com
VITE_GCS_BUCKET=videos
```

## Frontend Integration

The `client` package already wires up job submission and progress listening. Example usage:

```javascript
const submitVideo = async (videoFile) => {
  const jobRes = await fetch("http://localhost:8080/jobs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      video_id: crypto.randomUUID(),
      s3_path: `https://storage.googleapis.com/${import.meta.env.VITE_GCS_BUCKET}/source/${videoFile.name}`,
      original_filename: videoFile.name,
      bucket: "videos",
    }),
  });
  return jobRes.json();
};

const watchProgress = (videoId, onProgress) => {
  const es = new EventSource(`http://localhost:8080/progress/${videoId}`);
  es.onmessage = (event) => {
    const payload = JSON.parse(event.data);
    onProgress(payload);
    if (["completed", "failed"].includes(payload.status)) {
      es.close();
    }
  };
  es.onerror = () => es.close();
  return es;
};
```

## License

MIT
  // 2. Submit processing job
