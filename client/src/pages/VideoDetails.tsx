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
import { API_BASE_URL } from "@/lib/utils";
import type { ProcessingProgress, Video } from "@/types/video";

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
