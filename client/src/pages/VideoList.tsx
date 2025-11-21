"use client";

import { cache, Suspense, use } from "react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useNavigate } from "react-router";

const API_BASE_URL = "http://localhost:8080";

type VideoStatus = "pending" | "processing" | "completed" | "failed";

type Video = {
  id: string;
  original_name: string;
  s3_path: string;
  status: VideoStatus;
  source_height?: number;
  source_width?: number;
  duration?: number;
  file_size?: number;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  error_message?: string;
};

const statusMeta: Record<
  VideoStatus,
  { label: string; tone: "pending" | "processing" | "completed" | "failed" }
> = {
  pending: {
    label: "Queued",
    tone: "pending",
  },
  processing: {
    label: "Processing",
    tone: "processing",
  },
  completed: {
    label: "Completed",
    tone: "completed",
  },
  failed: {
    label: "Failed",
    tone: "failed",
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

const loadVideos = cache(async () => {
  const response = await fetch(`${API_BASE_URL}/videos?limit=50`);
  if (!response.ok) {
    throw new Error(`Unable to fetch videos (${response.status})`);
  }
  const data: Video[] = await response.json();
  return data;
});

export default function VideoListPage() {
  const navigate = useNavigate();
  const videosPromise = loadVideos();

  return (
    <main className="max-w-7xl mx-auto py-8 px-4">
      <div className="mb-8">
        <h1 className="text-3xl font-bold mb-2">All Videos</h1>
        <p className="text-muted-foreground">
          Browse all video processing jobs and their current status.
        </p>
      </div>

      <div className="panel list-panel">
        <div className="panel-header">
          <div>
            <h2>Video Queue</h2>
            <p>
              Shows the 50 most recent jobs. Click a row to view details or play
              the video.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => navigate(0)}
            // disabled={loadingList}
          >
            Refresh
          </Button>
        </div>

        <Suspense fallback={<div className="skeleton">Loading jobs…</div>}>
          <VideoList videosPromise={videosPromise} />
        </Suspense>
      </div>
    </main>
  );
}

function VideoList({ videosPromise }: { videosPromise: Promise<Video[]> }) {
  const videos = use(videosPromise);
  const navigate = useNavigate();

  if (videos.length === 0) {
    return (
      <div className="empty-state">
        <p>No jobs yet.</p>
        <span>Submit a video to get started.</span>
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Video</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Size</TableHead>
          <TableHead>Duration</TableHead>
          <TableHead>Created</TableHead>
          <TableHead>Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {videos.map((video) => (
          <TableRow
            key={video.id}
            onClick={() => navigate(`/videos/${video.id}`)}
            className="cursor-pointer"
          >
            <TableCell>
              <p className="primary">{video.original_name || video.id}</p>
              <span className="secondary">{video.id}</span>
            </TableCell>
            <TableCell>
              <span className={`status-pill ${statusMeta[video.status].tone}`}>
                {statusMeta[video.status].label}
              </span>
            </TableCell>
            <TableCell>{formatBytes(video.file_size)}</TableCell>
            <TableCell>{formatDuration(video.duration)}</TableCell>
            <TableCell>{formatDateTime(video.created_at)}</TableCell>
            <TableCell onClick={(e) => e.stopPropagation()}>
              {video.status === "completed" && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => navigate(`/videos/${video.id}/player`)}
                >
                  Play
                </Button>
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
