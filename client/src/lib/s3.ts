import { API_BASE_URL } from "./utils";

const gcsEndpoint =
  import.meta.env.VITE_GCS_ENDPOINT || "https://storage.googleapis.com";
const gcsBucketEnv = import.meta.env.VITE_GCS_BUCKET;

const ensureGcsBucket = (): string => {
  if (!gcsBucketEnv) {
    throw new Error(
      "GCS bucket not configured. Please set VITE_GCS_BUCKET in your .env file."
    );
  }
  return gcsBucketEnv;
};

const encodeKeyForUrl = (key: string): string =>
  key
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");

const DEFAULT_PART_SIZE = 5 * 1024 * 1024; // 5MB per part

export const parseGcsUrl = (
  gcsUrl: string
): { bucket: string; key: string } => {
  const gsMatch = gcsUrl.match(/^gs:\/\/([^/]+)\/(.+)$/);
  if (gsMatch) {
    return { bucket: gsMatch[1], key: gsMatch[2] };
  }

  const httpMatch = gcsUrl.match(/^https?:\/\/[^/]+\/([^/]+)\/(.+)$/);
  if (httpMatch) {
    return { bucket: httpMatch[1], key: httpMatch[2] };
  }

  throw new Error(
    "Invalid GCS URL format. Expected: gs://bucket/key or https://host/bucket/key"
  );
};

export const parseS3Url = parseGcsUrl;

export const getPresignedUrl = async (
  gcsUrl: string,
  expiresIn: number = 3600
): Promise<string> => {
  const { bucket, key } = parseGcsUrl(gcsUrl);
  const params = new URLSearchParams({ bucket, key });
  if (expiresIn) {
    params.set("expires", expiresIn.toString());
  }

  const response = await fetch(
    `${API_BASE_URL}/upload/signed-url?${params.toString()}`
  );

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || "Failed to generate signed URL");
  }

  const data = await response.json();
  if (!data.url) {
    throw new Error("Signed URL response missing 'url' field");
  }

  return data.url;
};

interface UploadState {
  uploadUrl: string;
  bytesUploaded: number;
  file: File;
  key: string;
}

const uploadStates = new Map<string, UploadState>();

async function initResumableUpload(
  bucket: string,
  key: string,
  contentType: string
): Promise<string> {
  const response = await fetch(`${API_BASE_URL}/upload/init`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      bucket,
      key,
      content_type: contentType,
    }),
  });

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || "Failed to initialize upload");
  }

  const data = await response.json();
  if (!data.upload_url) {
    throw new Error("Upload init response missing 'upload_url'");
  }

  return data.upload_url;
}

function uploadChunk(
  uploadUrl: string,
  file: File,
  start: number,
  end: number,
  totalSize: number,
  onProgress?: (loaded: number, total: number) => void
): Promise<{ bytesUploaded: number; complete: boolean }> {
  return new Promise((resolve, reject) => {
    const chunk = file.slice(start, end);
    const xhr = new XMLHttpRequest();

    xhr.open("PUT", uploadUrl, true);
    xhr.setRequestHeader(
      "Content-Type",
      file.type || "application/octet-stream"
    );
    xhr.setRequestHeader(
      "Content-Range",
      `bytes ${start}-${end - 1}/${totalSize}`
    );

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && onProgress) {
        onProgress(start + event.loaded, totalSize);
      }
    };

    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 201) {
        resolve({ bytesUploaded: totalSize, complete: true });
      } else if (xhr.status === 308) {
        const range = xhr.getResponseHeader("Range");
        if (range) {
          const match = range.match(/bytes=0-(\d+)/);
          const bytesUploaded = match ? parseInt(match[1], 10) + 1 : end;
          resolve({ bytesUploaded, complete: false });
        } else {
          resolve({ bytesUploaded: end, complete: false });
        }
      } else {
        reject(
          new Error(`Chunk upload failed: ${xhr.status} ${xhr.statusText}`)
        );
      }
    };

    xhr.onerror = () => reject(new Error("Network error during chunk upload"));
    xhr.ontimeout = () => reject(new Error("Upload timeout"));
    xhr.timeout = 60000;

    xhr.send(chunk);
  });
}

async function queryUploadStatus(
  uploadUrl: string,
  totalSize: number
): Promise<number> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", uploadUrl, true);
    xhr.setRequestHeader("Content-Range", `bytes */${totalSize}`);

    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 201) {
        resolve(totalSize);
      } else if (xhr.status === 308) {
        const range = xhr.getResponseHeader("Range");
        if (range) {
          const match = range.match(/bytes=0-(\d+)/);
          resolve(match ? parseInt(match[1], 10) + 1 : 0);
        } else {
          resolve(0);
        }
      } else {
        reject(new Error(`Failed to query upload status: ${xhr.status}`));
      }
    };

    xhr.onerror = () =>
      reject(new Error("Network error querying upload status"));
    xhr.send();
  });
}

export const uploadFileToS3 = async (
  file: File,
  key: string,
  onProgress?: (progress: number) => void
): Promise<string> => {
  const bucket = ensureGcsBucket();
  const fileSize = file.size;
  const stateKey = `${bucket}/${key}`;

  let state = uploadStates.get(stateKey);
  let uploadUrl = state?.uploadUrl ?? "";
  let bytesUploaded = 0;

  if (state && state.file === file) {
    uploadUrl = state.uploadUrl;
    try {
      bytesUploaded = await queryUploadStatus(uploadUrl, fileSize);
      if (bytesUploaded >= fileSize) {
        uploadStates.delete(stateKey);
        return `${gcsEndpoint}/${bucket}/${encodeKeyForUrl(key)}`;
      }
    } catch {
      state = undefined;
    }
  }

  if (!state) {
    uploadUrl = await initResumableUpload(
      bucket,
      key,
      file.type || "application/octet-stream"
    );
    state = { uploadUrl, bytesUploaded: 0, file, key };
    uploadStates.set(stateKey, state);
  }

  const partSize = DEFAULT_PART_SIZE;
  let currentByte = bytesUploaded;

  while (currentByte < fileSize) {
    const end = Math.min(currentByte + partSize, fileSize);
    const result = await uploadChunk(
      uploadUrl,
      file,
      currentByte,
      end,
      fileSize,
      (loaded, total) => {
        if (onProgress) {
          const percentage = Math.round((loaded / total) * 100);
          onProgress(percentage);
        }
      }
    );

    if (result.complete) {
      break;
    }

    currentByte = result.bytesUploaded;
    if (state) {
      state.bytesUploaded = currentByte;
    }
  }

  uploadStates.delete(stateKey);

  return `${gcsEndpoint}/${bucket}/${encodeKeyForUrl(key)}`;
};

export const uploadFileToGcs = uploadFileToS3;
