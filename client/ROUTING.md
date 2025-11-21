# Video Processing Client - Routing Guide

This application now uses **React Router v7** with declarative routing for navigation between different pages.

## Routes

The application has the following routes:

### 1. **Dashboard** (`/`)
- Overview of all video processing jobs
- Summary metrics showing pending, processing, completed, and failed jobs
- Quick access to recent video jobs
- Click on any video row to navigate to its details page

### 2. **Upload** (`/upload`)
- Submit new video processing jobs
- Generate unique video IDs
- Provide S3 path to source video
- Automatically navigates to video details page after successful submission

### 3. **Videos** (`/videos`)
- Complete list of all video processing jobs
- Status indicators for each video
- "Play" button for completed videos
- Click rows to view video details

### 4. **Video Details** (`/videos/:videoId`)
- Detailed information about a specific video
- Real-time progress updates via Server-Sent Events (SSE)
- Processing status and progress bar
- List of all available renditions/resolutions
- Error messages if processing fails
- "Play Video" button for completed videos

### 5. **Video Player** (`/videos/:videoId/player`)
- Full video playback interface using `@ntxmjs/react-custom-video-player`
- Adaptive quality selection
- Automatic presigned URL generation for S3 videos
- Display of available quality options (resolutions)

## Navigation

The app includes a navigation bar at the top with links to:
- Dashboard
- Upload
- Videos

The navigation automatically highlights the current page.

## Features

### Real-time Updates
- Dashboard and video lists refresh every 15 seconds
- Video details page subscribes to SSE for live progress updates
- Status changes are reflected immediately in the UI

### Video Player Integration
- Uses `@ntxmjs/react-custom-video-player` for playback
- Generates presigned URLs from S3 using `@aws-sdk/s3-request-presigner`
- Supports multiple quality levels (resolutions)
- Only shows for completed videos

### AWS S3 Configuration
Create a `.env` file based on `.env.example`:

```env
VITE_AWS_REGION=us-east-1
VITE_AWS_ACCESS_KEY_ID=your_access_key_here
VITE_AWS_SECRET_ACCESS_KEY=your_secret_key_here
```

**Important:** The AWS credentials are used client-side to generate presigned URLs. For production, consider implementing a backend endpoint to generate these URLs instead.

## Component Structure

```
src/
├── main.tsx                 # Router setup
├── App.tsx                  # Original component (preserved)
├── components/
│   ├── Layout.tsx          # Main layout with navigation
│   └── ui/                 # UI components (buttons, inputs, etc.)
└── pages/
    ├── Dashboard.tsx       # Overview dashboard
    ├── VideoUpload.tsx     # Upload/submit jobs
    ├── VideoList.tsx       # List all videos
    ├── VideoDetails.tsx    # Individual video details
    └── VideoPlayer.tsx     # Video playback
```

## Development

Start the development server:
```bash
bun run dev
```

The app will be available at `http://localhost:5173` (or the port Vite assigns).

## API Integration

All pages communicate with the backend API at `http://localhost:8080`:
- `GET /videos` - List videos
- `GET /videos/:id` - Get video details
- `POST /jobs` - Submit new processing job
- `GET /progress/:id` - SSE endpoint for real-time updates

## Type Safety

The application includes TypeScript type declarations for:
- Video entities
- Resolution/rendition data
- Processing progress updates
- Form data structures
- External libraries (`@ntxmjs/react-custom-video-player`)
