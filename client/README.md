# Video Processing Console (Client)

This React + Vite frontend provides an operator-friendly UI for the transcoding pipeline that runs on the Go API (port `:8080`). It replaces the Vite starter counter with a complete workflow console for launching and monitoring video jobs end-to-end.

## Features

- **Job submission form** – queue a new job by pasting an S3 URI and optional human-friendly name. Video IDs are pre-filled with a random UUID but can be regenerated in one click.
- **Live job queue** – fetches the latest 50 jobs from `GET /videos` and surfaces size, duration, and creation timestamp. Selecting a row loads deep metadata from `GET /videos/{id}`.
- **Real-time progress** – subscribes to `GET /progress/{id}` via Server-Sent Events and updates progress bars, rendition cards, and status pills without refreshing.
- **Detailed telemetry** – shows raw video metadata, timestamps, error messages, and every output rendition (resolution, bitrate, processed time, and S3 URL).
- **Responsive dashboard** – modern glassmorphism-inspired layout with cards, chips, and helpful hints that work on both desktop and tablet breakpoints.

## Expected backend

The UI assumes the Go API in `../server` is running locally on `http://localhost:8080` and exposes the following routes:

- `POST /jobs` – accepts `{ video_id, original_name, s3_path }` payloads.
- `GET /videos` – returns an array of `Video` records.
- `GET /videos/{id}` – returns a single `Video`, including any `resolutions` array.
- `GET /progress/{id}` (SSE) – streams `ProcessingProgress` events.

No client commands were executed as part of these updates per the project guidelines.

## Local setup

1. Start the backend stack from the repo root:

   ```bash
   docker compose -f docker-compose.basic.yaml -f docker-compose.yaml up -d --build
   ```

   (or run `docker compose -f docker-compose.basic.yaml up -d` plus local `go run` processes if you prefer to iterate outside containers.)

2. Navigate to the client folder and install dependencies:

   ```bash
   cd client
   npm install
   ```

3. (Optional) Create `.env.local` to override API/S3 targets. At minimum set the API host if it differs from the default `http://localhost:8000`:

   ```bash
   VITE_API_BASE_URL=http://localhost:8080
   VITE_AWS_REGION=us-east-1
   VITE_AWS_ACCESS_KEY_ID=minioadmin
   VITE_AWS_SECRET_ACCESS_KEY=minioadmin
   VITE_AWS_S3_ENDPOINT=http://localhost:9000
   VITE_AWS_S3_BUCKET=videos
   ```

4. Run the dev server:

   ```bash
   npm run dev
   ```

5. Open the printed Vite URL (typically `http://localhost:5173`). Submitting a job requires the API to be reachable and (if you enable uploads) the MinIO credentials above to resolve.

## Useful npm scripts

- `npm run dev` – Vite dev server with hot reloading.
- `npm run build` – Production build output to `dist/`.
- `npm run preview` – Serve the built bundle locally for smoke tests.
- `npm run lint` – Run the configured ESLint rules.

## Troubleshooting

- **API requests fail with `ECONNREFUSED`**: confirm the Go API is running on `http://localhost:8080` or set `VITE_API_BASE_URL` accordingly.
- **Uploads disabled warning**: populate all `VITE_AWS_*` variables so the S3 helper can generate pre-signed URLs.
- **Stale data**: the dashboard polls `/videos` every load. Use the refresh button or hard reload if you start the API after the client.

Queue jobs via the form and keep this page open to watch status changes in real time.

Feel free to extend the layout with charts, historical stats, or authentication hooks as your backend evolves.
