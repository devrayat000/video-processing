"use client";

import { cache, Suspense, use } from "react";
import { useParams, Link } from "react-router";
import { Button } from "@/components/ui/button";
import { CustomVideoPlayer } from "@ntxmjs/react-custom-video-player";
import { hasAwsCredentials } from "@/lib/s3";
import { playerIcons, videoContainerStyles } from "@/lib/videoPlayerConfig";

const API_BASE_URL = "http://localhost:8080";

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
  status: string;
  source_height?: number;
  source_width?: number;
  duration?: number;
  resolutions?: Resolution[];
};

const loadVideoDetails = cache(async (id: string) => {
  // Check if AWS credentials are configured
  if (!hasAwsCredentials()) {
    throw new Error(
      "AWS credentials not configured. Please set up your .env file."
    );
  }

  const response = await fetch(`${API_BASE_URL}/videos/${id}`);
  if (!response.ok) {
    throw new Error("Unable to load video details");
  }
  const data: Video = await response.json();
  // setVideo(data);

  if (data.status !== "completed") {
    throw new Error("Video processing is not complete yet.");
  }

  if (!data.resolutions || data.resolutions.length === 0) {
    throw new Error("No video renditions available.");
  }
  return data;

  // // Use the highest quality resolution available
  // const highestQuality = data.resolutions.sort(
  //   (a, b) => b.height - a.height
  // )[0];
  // const presignedUrl = await getPresignedUrl(highestQuality.s3_url);
  // return presignedUrl;
});

export default function VideoPlayerPage() {
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
      <div className="mb-8 flex items-center justify-between">
        <div>
          {/* <h1 className="text-3xl font-bold mb-2">
            {video?.original_name || "Video Player"}
          </h1> */}
          <p className="text-muted-foreground">
            Watch your processed video with adaptive quality selection.
          </p>
        </div>
        <Button asChild variant="outline">
          <Link to={`/videos/${videoId}`}>Back to Details</Link>
        </Button>
      </div>

      {/* {error && (
        <div className="panel">
          <p className="error">{error}</p>
          <Button asChild className="mt-4">
            <Link to={`/videos/${videoId}`}>View Details</Link>
          </Button>
        </div>
      )} */}

      <Suspense
        fallback={
          <div className="flex items-center justify-center h-96">
            <p>Loading video...</p>
          </div>
        }
      >
        <VideoPlayer videoPromise={loadVideoDetails(videoId)} />
      </Suspense>
    </main>
  );
}

function VideoPlayer({ videoPromise }: { videoPromise: Promise<Video> }) {
  const video = use(videoPromise);

  return (
    <div className="panel">
      <div className="aspect-video bg-black rounded-lg overflow-hidden">
        <CustomVideoPlayer
          src={video.s3_path}
          poster=""
          theme="dark"
          icons={playerIcons}
          videoContainerStyles={videoContainerStyles}
          type="auto"
          stableVolume={true}
          controlSize={40}
        />
      </div>

      {/* {video?.resolutions && (
            <div className="mt-6">
              <h3 className="font-semibold mb-3">Available Quality Options</h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {video.resolutions
                  .sort((a, b) => b.height - a.height)
                  .map((resolution) => (
                    <div key={resolution.id} className="border rounded p-3">
                      <p className="font-medium">{resolution.height}p</p>
                      <p className="text-sm text-muted-foreground">
                        {resolution.width}×{resolution.height}
                      </p>
                      <p className="text-sm text-muted-foreground">
                        {resolution.bitrate} kbps
                      </p>
                    </div>
                  ))}
              </div>
            </div>
          )} */}
    </div>
  );
}
