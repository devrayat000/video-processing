# Video Player Implementation Guide

## Overview

The video player page has been successfully implemented using `@ntxmjs/react-custom-video-player` v0.1.2. This is a production-ready, feature-rich video player with HLS streaming support, custom themes, and a comprehensive control interface.

## Installation

The following packages have been installed:

```bash
bun add @ntxmjs/react-custom-video-player@^0.1.2
bun add react-router@^7.9.6
bun add @aws-sdk/client-s3@^3.937.0
bun add @aws-sdk/s3-request-presigner
bun add hls.js  # Peer dependency for HLS streaming
```

## File Structure

### New Files Created

1. **`/client/src/lib/videoPlayerConfig.tsx`**
   - Exports configured `playerIcons` and `videoContainerStyles`
   - Centralizes video player configuration

2. **`/client/src/components/VideoPlayerIcons.tsx`**
   - Contains all SVG icon components for the video player
   - Includes: Play, Pause, Volume, Fullscreen, PiP, Settings, Speed, Captions, etc.

3. **`/client/src/types/react-custom-video-player.d.ts`**
   - TypeScript type declarations for the package
   - Defines interfaces for `CustomVideoPlayer`, `IconSet`, `VideoContainerStyles`, etc.

### Updated Files

1. **`/client/src/pages/VideoPlayer.tsx`**
   - Uses `CustomVideoPlayer` component from `@ntxmjs/react-custom-video-player`
   - Fetches video details from API
   - Generates presigned S3 URLs for video playback
   - Displays video with adaptive quality selection

## Environment Configuration

Create a `.env` file in the `/client` directory with the following variables:

```bash
VITE_AWS_REGION=us-east-1
VITE_AWS_ACCESS_KEY_ID=your_access_key_here
VITE_AWS_SECRET_ACCESS_KEY=your_secret_key_here
```

See `.env.example` for reference.

## Video Player Features

Based on the `@ntxmjs/react-custom-video-player` documentation, the player includes:

### Media Support
- ✅ HLS streaming (`.m3u8`) with adaptive bitrate
- ✅ Native MP4, WebM, and OGG formats
- ✅ Automatic video type detection
- ✅ Preview thumbnails on timeline hover

### User Interface
- ✅ Dark and light themes
- ✅ Responsive, mobile-first design
- ✅ Idle detection (auto-hide controls)
- ✅ Theater mode
- ✅ Picture-in-Picture (PiP) support
- ✅ Fullscreen mode (mobile-optimized)

### Subtitles & Captions
- ✅ Multi-language VTT format support
- ✅ Custom caption styling with backdrop blur
- ✅ Keyboard shortcuts for cycling captions
- ✅ Auto-positioning based on control visibility

### Playback Controls
- ✅ Adjustable playback speeds (0.25x to 2x)
- ✅ Visual volume slider
- ✅ Stable Volume (Web Audio API compression)
- ✅ Sleep timer (10, 20, 30, 45, 60 minutes)
- ✅ Progress bar with buffering status
- ✅ Precise seeking with hover preview

### Keyboard Shortcuts

| Key | Action | Description |
|-----|--------|-------------|
| Space / K | Play / Pause | Toggle video playback |
| J | Seek -10s | Jump backward 10 seconds |
| L | Seek +10s | Jump forward 10 seconds |
| ← | Seek -5s | Jump backward 5 seconds |
| → | Seek +5s | Jump forward 5 seconds |
| ↑ | Volume Up | Increase volume by 5% |
| ↓ | Volume Down | Decrease volume by 5% |
| M | Mute / Unmute | Toggle audio mute |
| F | Fullscreen | Toggle fullscreen mode |
| I | Picture-in-Picture | Toggle PiP mode |
| C | Cycle Captions | Cycle through captions |
| , | Speed Down | Decrease playback speed |
| . | Speed Up | Increase playback speed |
| Esc | Close Panel | Close settings menu |

## Usage

### Basic Implementation

The VideoPlayer component is already configured and ready to use:

```tsx
import { CustomVideoPlayer } from '@ntxmjs/react-custom-video-player'
import { playerIcons, videoContainerStyles } from '@/lib/videoPlayerConfig'

<CustomVideoPlayer
  src={videoUrl}
  poster=""
  theme="dark"
  icons={playerIcons}
  videoContainerStyles={videoContainerStyles}
  type="auto"
  stableVolume={true}
  controlSize={40}
/>
```

### Component Props

According to the package documentation:

```tsx
interface CustomVideoPlayerProps {
  src: string                           // Video source URL (required)
  poster: string                        // Poster image URL (required)
  theme: 'light' | 'dark' | Theme      // Player theme (required)
  icons: IconSet                        // Custom icon components (required)
  videoContainerStyles: VideoContainerStyles // Container styles (required)
  captions?: CaptionTrack[]            // Optional subtitle tracks
  startTime?: number                    // Initial playback time (seconds)
  stableVolume?: boolean               // Enable audio compression
  className?: string                    // Custom CSS class
  controlSize?: number                  // Control button size (px)
  type?: 'hls' | 'mp4' | 'auto'        // Force video type
}
```

## API Integration

The VideoPlayer component integrates with your backend API:

1. **Fetch Video Details**: `GET /videos/:videoId`
2. **Check Processing Status**: Ensures video status is "completed"
3. **Get Resolutions**: Retrieves available video quality options
4. **Generate Presigned URLs**: Creates temporary S3 access URLs
5. **Display Player**: Renders video with highest available quality

## Mobile Support

The video player is fully mobile-optimized with:

- Touch-optimized controls with proper event handling
- Fullscreen orientation support
- Responsive design breakpoints:
  - `≤350px`: Minimal controls
  - `≤520px`: Hidden volume controls
  - `≤620px`: Fixed settings panel

## Customization

### Adding Custom Themes

You can create custom themes by following the package documentation:

```tsx
const customTheme: Theme = {
  colors: {
    background: "linear-gradient(135deg, #0a1a0a, #0d1f0d, #0a1a0a)",
    // ... other color properties
  },
  fonts: {
    primary: "Roboto, Arial, sans-serif",
    // ... other font properties
  },
  borderRadius: {
    small: "4px",
    medium: "8px",
    large: "12px",
  },
  spacing: {
    small: "8px",
    medium: "12px",
    large: "16px",
  },
}

<CustomVideoPlayer theme={customTheme} src="video.m3u8" />
```

### Adding Subtitles

```tsx
const captions: CaptionTrack[] = [
  {
    src: "https://example.com/subtitles.en.vtt",
    srclang: "en",
    label: "English",
    default: true,
  },
  {
    src: "https://example.com/subtitles.es.vtt",
    srclang: "es",
    label: "Spanish",
    default: false,
  },
]

<CustomVideoPlayer captions={captions} {...otherProps} />
```

## Troubleshooting

### AWS Credentials Not Configured

If you see the error "AWS credentials not configured":

1. Create a `.env` file in `/client` directory
2. Add your AWS credentials (see Environment Configuration above)
3. Restart the development server

### Video Not Playing

1. Check that video processing is complete (status: "completed")
2. Verify S3 URLs are valid
3. Ensure presigned URLs are being generated correctly
4. Check browser console for errors

### TypeScript Errors

If TypeScript complains about the package import:

1. The type declarations are in `/client/src/types/react-custom-video-player.d.ts`
2. Restart the TypeScript server: `Cmd/Ctrl + Shift + P` → "TypeScript: Restart TS Server"

## References

- Package Documentation: https://www.npmjs.com/package/@ntxmjs/react-custom-video-player
- React Router Docs: https://reactrouter.com/
- AWS SDK for JavaScript: https://docs.aws.amazon.com/sdk-for-javascript/
- HLS.js Documentation: https://github.com/video-dev/hls.js/

## Next Steps

1. **Test the video player** with actual video files
2. **Configure AWS credentials** in your `.env` file
3. **Customize the theme** if needed to match your brand
4. **Add subtitle support** for multi-language videos
5. **Implement quality switching** if you want manual quality control
