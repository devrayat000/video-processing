import { useState } from "react";
import { toast } from "sonner";
import { useNavigate } from "react-router";
import { Upload, X, FileVideo, CheckCircle2 } from "lucide-react";
import {
  Dropzone,
  DropZoneArea,
  DropzoneDescription,
  DropzoneFileList,
  DropzoneFileListItem,
  DropzoneMessage,
  DropzoneRemoveFile,
  DropzoneRetryFile,
  DropzoneTrigger,
  InfiniteProgress,
  useDropzone,
} from "@/components/ui/dropzone";
import { uploadFileToS3 } from "@/lib/s3";
import type { VideoJob } from "@/types/video";
import { API_BASE_URL } from "@/lib/utils";

const safeUUID = () =>
  typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `video-${Math.random().toString(36).slice(2, 9)}`;

type UploadResult = {
  videoId: string;
  s3Path: string;
  originalName: string;
};

type UploadError = string;

const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
};

// Submit job to backend after upload
const submitJobToBackend = async (params: VideoJob): Promise<string> => {
  const response = await fetch(`${API_BASE_URL}/jobs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      video_id: params.id,
      original_name: params.name,
      s3_path: params.url,
      content_type: params.type,
    }),
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || "Failed to submit job");
  }

  const result = await response.json();
  return result.id || params.id;
};

export default function VideoUpload() {
  const navigate = useNavigate();
  const [uploadProgress, setUploadProgress] = useState<Record<string, number>>(
    {}
  );

  const dropzone = useDropzone<UploadResult, UploadError>({
    validation: {
      accept: {
        "video/*": [".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv"],
      },
      maxSize: 5 * 1024 * 1024 * 1024, // 5GB
      maxFiles: 5,
    },
    onDropFile: async (file) => {
      const videoId = safeUUID();
      const timestamp = Date.now();
      const key = `${videoId}/${timestamp}-${file.name}`;

      try {
        // Upload file to S3 with progress tracking
        const s3Path = await uploadFileToS3(file, key, (progress) => {
          setUploadProgress((prev) => ({ ...prev, [videoId]: progress }));
        });

        // Submit job to backend
        const toastId = toast.promise(
          submitJobToBackend({
            id: videoId,
            name: file.name,
            url: s3Path,
            type: file.type,
          }),
          {
            loading: "Uploading and submitting job...",
            success: (jobId) =>
              `Video uploaded and job ${jobId} queued successfully!`,
          }
        );

        return {
          status: "success",
          result: {
            videoId: await toastId.unwrap(),
            s3Path,
            originalName: file.name,
          },
        };
      } catch (error) {
        const errorMessage =
          error instanceof Error ? error.message : "Upload failed";
        toast.error(errorMessage);
        return {
          status: "error",
          error: errorMessage,
        };
      } finally {
        // Clean up progress tracking
        setUploadProgress((prev) => {
          const next = { ...prev };
          delete next[videoId];
          return next;
        });
      }
    },
    onFileUploaded: (result) => {
      // Navigate to video details after a short delay
      setTimeout(() => {
        navigate(`/videos/${result.videoId}`);
      }, 2000);
    },
    maxRetryCount: 3,
    autoRetry: false,
  });

  return (
    <main className="max-w-4xl mx-auto py-8 px-4">
      <div className="mb-8">
        <h1 className="text-3xl font-bold mb-2">Upload Video</h1>
        <p className="text-muted-foreground">
          Upload video files directly to S3 and automatically queue them for
          processing.
        </p>
      </div>

      <Dropzone {...dropzone}>
        <div className="panel">
          <div className="space-y-6">
            <DropZoneArea className="min-h-[200px] flex-col gap-4">
              <FileVideo className="h-12 w-12 text-muted-foreground" />
              <div className="text-center">
                <p className="text-lg font-medium">Drop video files here</p>
                <DropzoneDescription>
                  or click the button below to browse
                </DropzoneDescription>
              </div>
              <DropzoneTrigger>
                <Upload className="mr-2 h-4 w-4 inline" />
                Browse Files
              </DropzoneTrigger>
              <p className="text-xs text-muted-foreground text-center">
                Supported formats: MP4, MOV, AVI, MKV, WebM, FLV, WMV (Max 5GB
                per file, up to 5 files)
              </p>
            </DropZoneArea>

            <DropzoneMessage />

            {dropzone.fileStatuses.length > 0 && (
              <div className="space-y-2">
                <h3 className="font-semibold">
                  Uploads ({dropzone.fileStatuses.length})
                </h3>
                <DropzoneFileList>
                  {dropzone.fileStatuses.map((file) => {
                    const progress = uploadProgress[file.id] || 0;
                    const isPending = file.status === "pending";
                    const isSuccess = file.status === "success";
                    const isError = file.status === "error";

                    return (
                      <DropzoneFileListItem key={file.id} file={file}>
                        <div className="flex items-start justify-between gap-4">
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-2">
                              {isSuccess && (
                                <CheckCircle2 className="h-4 w-4 text-green-600 shrink-0" />
                              )}
                              {isError && (
                                <X className="h-4 w-4 text-destructive shrink-0" />
                              )}
                              <p className="font-medium truncate">
                                {file.fileName}
                              </p>
                            </div>
                            <p className="text-sm text-muted-foreground mb-2">
                              {formatFileSize(file.file.size)}
                              {isPending &&
                                progress > 0 &&
                                ` • ${progress}% uploaded`}
                              {isSuccess && " • Upload complete"}
                            </p>
                            {isPending && (
                              <InfiniteProgress
                                status="pending"
                                className="mb-2"
                              />
                            )}
                            {isSuccess && (
                              <InfiniteProgress
                                status="success"
                                className="mb-2"
                              />
                            )}
                            {isError && (
                              <InfiniteProgress
                                status="error"
                                className="mb-2"
                              />
                            )}
                          </div>
                          <div className="flex gap-2 shrink-0">
                            {isError && dropzone.canRetry(file.id) && (
                              <DropzoneRetryFile />
                            )}
                            {!isPending && <DropzoneRemoveFile />}
                          </div>
                        </div>
                      </DropzoneFileListItem>
                    );
                  })}
                </DropzoneFileList>
              </div>
            )}
          </div>
        </div>
      </Dropzone>

      <div className="mt-6 p-4 border rounded-lg bg-muted/50">
        <h3 className="font-semibold mb-2">How it works</h3>
        <ol className="list-decimal list-inside space-y-1 text-sm text-muted-foreground">
          <li>Select or drop video files into the upload area</li>
          <li>
            Files are uploaded directly to S3 using secure multipart upload
          </li>
          <li>
            A processing job is automatically created for each uploaded video
          </li>
          <li>
            You'll be redirected to the video details page to track progress
          </li>
        </ol>
      </div>
    </main>
  );
}
