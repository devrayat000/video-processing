"use client";

import {
  Activity,
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
import { CircularProgress } from "@/components/custom-ui/circular-progress";

const API_BASE_URL = "http://localhost:8080";

type VideoStatus =
  | "waiting"
  | "started"
  | "processing"
  | "completed"
  | "failed";

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
  frames?: number;
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
  total_frames?: number;
  processed_frames?: number;
  error?: string;
  timestamp: string;
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
  const isCompleted =
    video.status === "completed" &&
    !!video.master_playlist_url &&
    !!video.completed_at;

  return (
    <div className="panel mb-8">
      <section>
        <div className="aspect-video bg-black rounded-lg overflow-hidden flex items-center justify-center relative">
          <CustomVideoPlayer
            src={video.master_playlist_url ?? ""}
            poster=""
            theme="dark"
            icons={playerIcons}
            videoContainerStyles={videoContainerStyles}
            type="auto"
            stableVolume={true}
            controlSize={40}
          />
          <Activity name="progress" mode={isCompleted ? "hidden" : "visible"}>
            <VideoProgress video={video} />
          </Activity>
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
    </div>
  );
}

function VideoProgress({ video: loadedVideo }: { video: Video }) {
  const { videoId } = useParams<{ videoId: string }>();
  const [, setVideo] = useOptimistic<Video>(loadedVideo);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string>();
  // const [videoProgress, setVideoProgress] = useState<ProcessingProgress | null>(
  //   null
  // );

  useEffect(() => {
    if (typeof window === "undefined" || !("EventSource" in window)) {
      return;
    }

    const source = new EventSource(`${API_BASE_URL}/progress/${videoId}`);

    source.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as ProcessingProgress;
        // setVideoProgress(payload);
        setVideo((prev) =>
          prev && prev.id === payload.video_id
            ? { ...prev, status: payload.status }
            : prev
        );
        switch (payload.status) {
          case "waiting":
          case "started":
            break;
          case "processing":
            setProgress(
              ((payload?.processed_frames ?? 0) * 100) /
                (payload?.total_frames ?? 1)
            );
            break;
          case "completed":
            setProgress(100);
            break;
          case "failed":
            setError(payload.error);
            break;
          default:
            break;
        }
        // if (payload.status === "completed" || payload.status === "failed") {
        //   loadVideoDetails(payload.video_id);
        // }
      } catch (error) {
        setError("Failed to parse SSE payload");
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
    <section className="absolute inset-0 grid place-items-center bg-black/75 text-white p-4">
      {!error ? (
        <CircularProgress value={progress} />
      ) : (
        <p className="font-mono text-center">{error}</p>
      )}
    </section>
  );
}
