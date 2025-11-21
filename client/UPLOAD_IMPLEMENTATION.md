# Video Upload Implementation

## Overview

The video upload feature has been implemented using the Dropzone component with AWS S3 multipart upload via `@aws-sdk/lib-storage`. This allows users to upload large video files efficiently with progress tracking.

## Components Used

### 1. Dropzone Component (`/client/src/components/ui/dropzone.tsx`)
A comprehensive file upload component with:
- Drag and drop support
- File validation (type, size, count)
- Progress tracking
- Error handling and retry logic
- Accessible UI with ARIA attributes

### 2. AWS S3 Multipart Upload (`@aws-sdk/lib-storage`)
- Efficient upload of large files (up to 5GB)
- Automatic chunking (5MB per part)
- Progress tracking callbacks
- Concurrent uploads (4 parts at a time)
- Automatic cleanup on error

## How It Works

### Upload Flow

1. **File Selection/Drop**
   - User selects or drops video files into the dropzone
   - Files are validated against:
     - Type: MP4, MOV, AVI, MKV, WebM, FLV, WMV
     - Size: Maximum 5GB per file
     - Count: Maximum 5 files at once

2. **S3 Upload**
   - Each file is uploaded to S3 using multipart upload
   - Files are stored in: `s3://bucket/uploads/{videoId}/{timestamp}-{filename}`
   - Progress is tracked and displayed (0-100%)

3. **Job Creation**
   - After successful upload, a processing job is created via API
   - Job includes: video ID, original filename, and S3 path

4. **Navigation**
   - User is automatically redirected to the video details page
   - Can track processing progress from there

## Implementation Details

### S3 Upload Function (`/client/src/lib/s3.ts`)

```typescript
export const uploadFileToS3 = async (
  file: File,
  key: string,
  onProgress?: (progress: number) => void
): Promise<string>
```

**Parameters:**
- `file`: The File object to upload
- `key`: S3 key (path) where file will be stored
- `onProgress`: Optional callback receiving progress percentage (0-100)

**Returns:** S3 URL in format `s3://bucket/key`

**Configuration:**
- `queueSize: 4` - Number of concurrent part uploads
- `partSize: 5MB` - Size of each multipart chunk (S3 minimum)
- `leavePartsOnError: false` - Auto-cleanup failed uploads

### VideoUpload Component (`/client/src/pages/VideoUpload.tsx`)

**Key Features:**
- Uses `useDropzone` hook for file management
- Tracks upload progress per file
- Displays file status (pending, success, error)
- Supports retry on failed uploads (up to 3 attempts)
- Auto-navigation on successful upload

**State Management:**
```typescript
const [uploadProgress, setUploadProgress] = useState<Record<string, number>>({})
```

Tracks upload percentage for each file by video ID.

## Environment Configuration

Required environment variables in `/client/.env`:

```bash
VITE_AWS_REGION=us-east-1
VITE_AWS_ACCESS_KEY_ID=your_access_key_here
VITE_AWS_SECRET_ACCESS_KEY=your_secret_key_here
VITE_AWS_S3_BUCKET=your_bucket_name_here
```

See `/client/.env.example` for reference.

## File Validation

### Accepted Formats
```typescript
accept: {
  'video/*': ['.mp4', '.mov', '.avi', '.mkv', '.webm', '.flv', '.wmv']
}
```

### Size Limits
- **Maximum per file:** 5GB (5 * 1024 * 1024 * 1024 bytes)
- **Maximum files:** 5 concurrent uploads

### Error Messages
The Dropzone component automatically generates user-friendly error messages:
- `"only video/mp4, video/quicktime, ... are allowed"` - Invalid file type
- `"max size is 5120.00MB"` - File too large
- `"max 5 files"` - Too many files

## UI Components

### Upload Area
- Displays drag-and-drop zone
- Shows upload icon and instructions
- Highlights on drag-over
- Browse button for manual selection

### File List
Each uploaded file shows:
- ✅ Success icon (green check) when complete
- ❌ Error icon (red X) when failed
- File name and size
- Upload progress bar
- Progress percentage during upload
- Retry button (on error, if retries available)
- Remove button (when not uploading)

### Progress Indicators
```tsx
<InfiniteProgress status="pending" />  // Animated during upload
<InfiniteProgress status="success" />  // Green on complete
<InfiniteProgress status="error" />    // Red on failure
```

## API Integration

### Submit Job Endpoint

**POST** `/jobs`

**Request Body:**
```json
{
  "video_id": "uuid",
  "original_name": "video.mp4",
  "s3_path": "s3://bucket/uploads/uuid/timestamp-video.mp4"
}
```

**Response:**
```json
{
  "id": "job-id-or-video-id"
}
```

## Error Handling

### Upload Errors
- Network failures
- AWS credential issues
- S3 permission errors
- File size exceeded
- Invalid file type

All errors are caught and displayed with user-friendly messages via toast notifications.

### Retry Logic
- **Auto-retry:** Disabled by default
- **Manual retry:** Available for up to 3 attempts per file
- **Retry button:** Shown only when retries are available

## Usage Example

```tsx
import { VideoUpload } from '@/pages/VideoUpload'

// In your router
<Route path="upload" element={<VideoUpload />} />
```

The component is fully self-contained and requires no additional props.

## Accessibility

The Dropzone component implements comprehensive accessibility:
- ARIA labels and descriptions
- Keyboard navigation support
- Focus management
- Screen reader announcements
- Error messages linked via `aria-describedby`
- Progress status via `role="progressbar"`

## Performance Considerations

### Multipart Upload Benefits
1. **Resumability:** Failed parts can be retried individually
2. **Parallelization:** Multiple parts upload concurrently
3. **Large Files:** Required for files >5GB on S3
4. **Reliability:** Network interruptions only affect individual parts

### Memory Management
- Files are streamed in chunks (5MB)
- No need to load entire file into memory
- Progress updates prevent UI freezing

## Security

### Client-Side
- Validates file types and sizes before upload
- Uses AWS credentials from environment variables
- Never exposes credentials in client code

### Server-Side
Ensure your backend:
- Validates S3 paths and video IDs
- Implements proper IAM permissions
- Restricts S3 bucket access
- Validates uploaded content

## Troubleshooting

### "AWS credentials not configured"
1. Create `.env` file in `/client` directory
2. Add all required AWS variables
3. Restart dev server

### "S3 bucket not configured"
1. Add `VITE_AWS_S3_BUCKET` to `.env`
2. Ensure bucket exists and has proper permissions
3. Restart dev server

### Upload Fails Silently
1. Check browser console for errors
2. Verify AWS credentials are valid
3. Check S3 bucket permissions (PutObject)
4. Ensure CORS is configured on S3 bucket

### Progress Not Updating
- This is normal for very fast uploads on local networks
- Progress updates occur per-chunk (5MB)
- Small files may complete before progress updates

## Future Enhancements

Potential improvements:
1. **Resume uploads** - Save upload state to localStorage
2. **Pause/resume** - Allow users to pause ongoing uploads
3. **Thumbnail generation** - Extract video thumbnail on upload
4. **Batch operations** - Select multiple files for retry/remove
5. **Upload queue** - Sequential upload mode option
6. **Compression** - Client-side video compression before upload

## Dependencies

```json
{
  "@aws-sdk/client-s3": "^3.937.0",
  "@aws-sdk/lib-storage": "latest",
  "@aws-sdk/s3-request-presigner": "^3.937.0",
  "react-dropzone": "^14.x",
  "lucide-react": "latest"
}
```

## References

- [AWS SDK for JavaScript v3](https://docs.aws.amazon.com/AWSJavaScriptSDK/v3/latest/)
- [@aws-sdk/lib-storage Documentation](https://docs.aws.amazon.com/AWSJavaScriptSDK/v3/latest/modules/_aws_sdk_lib_storage.html)
- [React Dropzone](https://react-dropzone.js.org/)
- [S3 Multipart Upload](https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html)
