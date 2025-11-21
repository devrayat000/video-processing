import { S3Client, GetObjectCommand } from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { Upload } from "@aws-sdk/lib-storage";

// Configure S3 client with environment variables
const createS3Client = () => {
  const region = import.meta.env.VITE_AWS_REGION;
  const accessKeyId = import.meta.env.VITE_AWS_ACCESS_KEY_ID;
  const secretAccessKey = import.meta.env.VITE_AWS_SECRET_ACCESS_KEY;

  // If credentials are not provided, return a client that will fail gracefully
  if (!region || !accessKeyId || !secretAccessKey) {
    console.warn(
      "AWS credentials not configured. Video playback may not work."
    );
  }

  return new S3Client({
    region: region || "us-east-1",
    endpoint: "http://localhost:9000",
    forcePathStyle: true,
    credentials:
      accessKeyId && secretAccessKey
        ? {
            accessKeyId,
            secretAccessKey,
          }
        : undefined,
  });
};

export const s3Client = createS3Client();

/**
 * Parse an S3 URL (s3://bucket/key) into bucket and key components
 */
export const parseS3Url = (s3Url: string): { bucket: string; key: string } => {
  const match = s3Url.match(/^s3:\/\/([^/]+)\/(.+)$/);
  if (!match) {
    throw new Error("Invalid S3 URL format. Expected: s3://bucket/key");
  }
  return { bucket: match[1], key: match[2] };
};

/**
 * Generate a presigned URL for an S3 object
 * @param s3Url - The S3 URL (s3://bucket/key format)
 * @param expiresIn - URL expiration time in seconds (default: 1 hour)
 * @returns A presigned URL that can be used to access the object
 */
export const getPresignedUrl = async (
  s3Url: string,
  expiresIn: number = 3600
): Promise<string> => {
  try {
    const { bucket, key } = parseS3Url(s3Url);
    const command = new GetObjectCommand({ Bucket: bucket, Key: key });
    const url = await getSignedUrl(s3Client, command, { expiresIn });
    return url;
  } catch (error) {
    console.error("Failed to generate presigned URL:", error);
    // In development, you might want to return the original URL
    // In production, you should handle this error appropriately
    throw error;
  }
};

/**
 * Check if AWS credentials are configured
 */
export const hasAwsCredentials = (): boolean => {
  return !!(
    import.meta.env.VITE_AWS_REGION &&
    import.meta.env.VITE_AWS_ACCESS_KEY_ID &&
    import.meta.env.VITE_AWS_SECRET_ACCESS_KEY
  );
};

/**
 * Get the configured S3 bucket name from environment variables
 */
export const getS3Bucket = (): string => {
  const bucket = import.meta.env.VITE_AWS_S3_BUCKET;
  if (!bucket) {
    throw new Error(
      "S3 bucket not configured. Please set VITE_AWS_S3_BUCKET in your .env file"
    );
  }
  return bucket;
};

/**
 * Upload a file to S3 using multipart upload
 * @param file - The file to upload
 * @param key - The S3 key (path) where the file will be stored
 * @param onProgress - Optional callback for upload progress (0-100)
 * @returns The S3 URL of the uploaded file
 */
export const uploadFileToS3 = async (
  file: File,
  key: string,
  onProgress?: (progress: number) => void
): Promise<string> => {
  if (!hasAwsCredentials()) {
    throw new Error(
      "AWS credentials not configured. Please set up your .env file."
    );
  }

  const bucket = getS3Bucket();

  try {
    const upload = new Upload({
      client: s3Client,
      params: {
        Bucket: bucket,
        Key: key,
        Body: file,
        ContentType: file.type,
      },
      // Multipart upload configuration
      queueSize: 4, // Number of concurrent uploads
      partSize: 5 * 1024 * 1024, // 5MB per part (minimum allowed by S3)
      leavePartsOnError: false, // Clean up on error
    });

    // Track upload progress
    upload.on("httpUploadProgress", (progress) => {
      if (onProgress && progress.loaded && progress.total) {
        const percentage = Math.round((progress.loaded / progress.total) * 100);
        onProgress(percentage);
      }
    });

    await upload.done();

    // Return the S3 URL
    return `http://localhost:9000/${bucket}/${key}`;
  } catch (error) {
    console.error("Failed to upload file to S3:", error);
    throw error;
  }
};
