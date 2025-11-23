export type VideoStatus =
  | "waiting"
  | "started"
  | "processing"
  | "completed"
  | "failed";

export type Resolution = {
  id: string;
  height: number;
  width: number;
  s3_url: string;
  bitrate: number;
  file_size: number;
  processed_at: string;
};

export type Video = {
  id: string;
  original_name: string;
  s3_path: string;
  content_type: string;
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

export type ProcessingProgress = {
  video_id: string;
  status: VideoStatus;
  total_frames?: number;
  processed_frames?: number;
  error?: string;
  timestamp: string;
};

export interface VideoJob {
  id: string;
  name: string;
  url: string;
  type: string;
}
