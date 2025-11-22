"use client";

import {
  cache,
  Suspense,
  use,
  useEffect,
  useOptimistic,
  useState,
} from "react";
import { useParams } from "react-router";
import { CustomVideoPlayer } from "@ntxmjs/react-custom-video-player";
import { playerIcons, videoContainerStyles } from "@/lib/videoPlayerConfig";

const API_BASE_URL = "http://localhost:8080";

type VideoStatus = "pending" | "processing" | "completed" | "failed";

type Resolution = {
  id: string;
  height: number;
  width: number;
  s3_url: string;
  bitrate: number;
  file_size: number;
  processed_at: string;
};

type Video = {
  id: string;
  original_name: string;
  s3_path: string;
  status: VideoStatus;
  master_playlist_url?: string;
  source_height?: number;
  source_width?: number;
  duration?: number;
  file_size?: number;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  error_message?: string;
  resolutions?: Resolution[];
};

type ProcessingProgress = {
  video_id: string;
  status: VideoStatus;
  current_resolution?: number;
  total_resolutions?: number;
  progress: number;
  message?: string;
  timestamp: string;
};

const statusMeta: Record<
  VideoStatus,
  {
    label: string;
    tone: "pending" | "processing" | "completed" | "failed";
    blurb: string;
  }
> = {
  pending: {
    label: "Queued",
    tone: "pending",
    blurb: "Waiting for a worker to pick up the job.",
  },
  processing: {
    label: "Processing",
    tone: "processing",
    blurb: "A worker is actively transcoding the video.",
  },
  completed: {
    label: "Completed",
    tone: "completed",
    blurb: "All renditions are ready to deliver.",
  },
  failed: {
    label: "Failed",
    tone: "failed",
    blurb: "The job encountered an error. Inspect the logs below.",
  },
};

const formatBytes = (value?: number) => {
  if (!value || Number.isNaN(value)) return "—";
  if (value === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const idx = Math.floor(Math.log(value) / Math.log(1024));
  const size = value / Math.pow(1024, idx);
  return `${size.toFixed(1)} ${units[idx]}`;
};

const formatDuration = (value?: number) => {
  if (!value || Number.isNaN(value)) return "—";
  const minutes = Math.floor(value / 60);
  const seconds = Math.floor(value % 60);
  return `${minutes}m ${seconds.toString().padStart(2, "0")}s`;
};

const formatDateTime = (value?: string) => {
  if (!value) return "—";
  const dt = new Date(value);
  if (Number.isNaN(dt.valueOf())) return value;
  return dt.toLocaleString();
};

const loadVideoDetails = cache(async (id: string) => {
  const response = await fetch(`${API_BASE_URL}/videos/${id}`);
  if (!response.ok) {
    throw new Error("Unable to load video details");
  }
  const data: Video = await response.json();
  return data;
});

export default function VideoDetailsPage() {
  const { videoId } = useParams<{ videoId: string }>();

  if (!videoId) {
    return (
      <main className="max-w-7xl mx-auto py-8 px-4">
        <p>No video ID provided</p>
      </main>
    );
  }

  return (
    <main className="max-w-7xl mx-auto py-8 px-4">
      <Suspense>
        <VideoDetails videoPromise={loadVideoDetails(videoId)} />
      </Suspense>
    </main>
  );
}

function VideoDetails({ videoPromise }: { videoPromise: Promise<Video> }) {
  const video = use(videoPromise);

  return (
    <div className="panel mb-8">
      <section>
        <div className="aspect-video bg-black rounded-lg overflow-hidden flex items-center justify-center">
          {video?.status === "completed" && video.master_playlist_url ? (
            <CustomVideoPlayer
              src={video.master_playlist_url}
              poster=""
              theme="dark"
              icons={playerIcons}
              videoContainerStyles={videoContainerStyles}
              type="auto"
              stableVolume={true}
              controlSize={40}
            />
          ) : (
            <div className="text-center text-white px-4">
              <div className="mb-4">
                <svg
                  className="mx-auto h-16 w-16 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={1.5}
                    d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"
                  />
                </svg>
              </div>
              <p className="text-lg font-medium mb-2">
                {video?.status === "processing" && "Video is being processed"}
                {video?.status === "pending" &&
                  "Video is queued for processing"}
                {video?.status === "failed" && "Video processing failed"}
                {!video && "Loading video..."}
              </p>
              <p className="text-sm text-gray-400">
                {video?.status === "processing" &&
                  "Transcoding in progress, please wait..."}
                {video?.status === "pending" &&
                  "Waiting for a worker to start processing"}
                {video?.status === "failed" && "Check the error details below"}
                {!video && "Fetching video information..."}
              </p>
            </div>
          )}
        </div>
        <div className="mt-8">
          <div>
            <h1 className="text-3xl font-bold mb-2">
              {video?.original_name || "Video Details"}
            </h1>
            <p className="text-muted-foreground">
              {video?.status === "completed"
                ? "Watch your video and view processing details below."
                : "Video processing in progress. The player will be enabled once transcoding is complete."}
            </p>
          </div>
        </div>
      </section>
      <VideoProgress video={video} />
    </div>
  );
}

function VideoProgress({ video: loadedVideo }: { video: Video }) {
  const { videoId } = useParams<{ videoId: string }>();
  const [video, setVideo] = useOptimistic<Video>(loadedVideo);
  const [videoProgress, setVideoProgress] = useState<ProcessingProgress | null>(
    null
  );

  useEffect(() => {
    if (typeof window === "undefined" || !("EventSource" in window)) {
      return;
    }

    const source = new EventSource(`${API_BASE_URL}/progress/${videoId}`);

    source.onmessage = (event) => {
      try {
        const payload: ProcessingProgress = JSON.parse(event.data);
        setVideoProgress(payload);
        setVideo((prev) =>
          prev && prev.id === payload.video_id
            ? { ...prev, status: payload.status }
            : prev
        );
        if (payload.status === "completed" || payload.status === "failed") {
          loadVideoDetails(payload.video_id);
        }
      } catch (error) {
        console.error("Failed to parse SSE payload", error);
      }
    };

    source.onerror = () => {
      source.close();
    };

    return () => {
      source.close();
    };
  }, [setVideo, videoId]);

  return (
    <section>
      <div className="details-grid">
        <div className="panel detail-panel">
          <h2>Job details</h2>
          {video && (
            <div className="detail-grid">
              <div>
                <p className="label">Video ID</p>
                <p className="value mono">{video.id}</p>
              </div>
              <div>
                <p className="label">Original name</p>
                <p className="value">{video.original_name}</p>
              </div>
              <div>
                <p className="label">S3 path</p>
                <p className="value mono">{video.s3_path}</p>
              </div>
              <div>
                <p className="label">Source dimensions</p>
                <p className="value">
                  {video.source_width && video.source_height
                    ? `${video.source_width}×${video.source_height}`
                    : "—"}
                </p>
              </div>
              <div>
                <p className="label">Duration</p>
                <p className="value">{formatDuration(video.duration)}</p>
              </div>
              <div>
                <p className="label">File size</p>
                <p className="value">{formatBytes(video.file_size)}</p>
              </div>
              <div>
                <p className="label">Created</p>
                <p className="value">{formatDateTime(video.created_at)}</p>
              </div>
              <div>
                <p className="label">Updated</p>
                <p className="value">{formatDateTime(video.updated_at)}</p>
              </div>
              {video.completed_at && (
                <div>
                  <p className="label">Completed</p>
                  <p className="value">{formatDateTime(video.completed_at)}</p>
                </div>
              )}
              {video.error_message && (
                <div className="full-width">
                  <p className="label">Error</p>
                  <p className="value error-text">{video.error_message}</p>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="panel progress-panel">
          <h2>Progress & renditions</h2>
          {video && (
            <>
              <div className="progress-track">
                <div
                  className="progress-bar"
                  style={{
                    width: `${
                      videoProgress?.progress ??
                      (video.status === "completed" ? 100 : 0)
                    }%`,
                  }}
                />
              </div>
              <div className="progress-meta">
                <span
                  className={`status-pill ${statusMeta[video.status].tone}`}
                >
                  {statusMeta[video.status].label}
                </span>
                <span>
                  {videoProgress?.message || statusMeta[video.status].blurb}
                </span>
                {videoProgress?.current_resolution &&
                  videoProgress?.total_resolutions && (
                    <span>
                      Rendition {videoProgress.current_resolution} /{" "}
                      {videoProgress.total_resolutions}
                    </span>
                  )}
              </div>
              <div className="resolutions">
                {video.resolutions && video.resolutions.length > 0 ? (
                  video.resolutions.map((rendition) => (
                    <article key={rendition.id} className="rendition-card">
                      <header>
                        <p className="primary">{rendition.height}p</p>
                        <span className="secondary">
                          {formatBytes(rendition.file_size)}
                        </span>
                      </header>
                      <p className="mono">{rendition.s3_url}</p>
                      <footer>
                        <span>Bitrate: {rendition.bitrate} kbps</span>
                        <span>{formatDateTime(rendition.processed_at)}</span>
                      </footer>
                    </article>
                  ))
                ) : (
                  <p className="muted">No renditions reported yet.</p>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </section>
  );
}
