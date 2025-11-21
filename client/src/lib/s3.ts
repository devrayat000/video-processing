import { S3Client, GetObjectCommand } from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";

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
