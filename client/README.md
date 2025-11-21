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

## Development notes

1. Install dependencies in `/client` (Vite + React) if you have not already.
2. Run the API server first so the UI can successfully fetch `/videos` and subscribe to `/progress`.
3. Start the Vite dev server (`npm run dev` or equivalent) and open the provided URL.
4. Queue jobs via the form and keep this page open to watch status changes in real time.

Feel free to extend the layout with charts, historical stats, or authentication hooks as your backend evolves.
