import { useCallback, useEffect, useState } from 'react'
import { useParams, Link } from 'react-router'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'

const API_BASE_URL = 'http://localhost:8080'

type VideoStatus = 'pending' | 'processing' | 'completed' | 'failed'

type Resolution = {
  id: string
  height: number
  width: number
  s3_url: string
  bitrate: number
  file_size: number
  processed_at: string
}

type Video = {
  id: string
  original_name: string
  s3_path: string
  status: VideoStatus
  source_height?: number
  source_width?: number
  duration?: number
  file_size?: number
  created_at: string
  updated_at: string
  completed_at?: string
  error_message?: string
  resolutions?: Resolution[]
}

type ProcessingProgress = {
  video_id: string
  status: VideoStatus
  current_resolution?: number
  total_resolutions?: number
  progress: number
  message?: string
  timestamp: string
}

const statusMeta: Record<VideoStatus, { label: string; tone: 'pending' | 'processing' | 'completed' | 'failed'; blurb: string }> = {
  pending: {
    label: 'Queued',
    tone: 'pending',
    blurb: 'Waiting for a worker to pick up the job.',
  },
  processing: {
    label: 'Processing',
    tone: 'processing',
    blurb: 'A worker is actively transcoding the video.',
  },
  completed: {
    label: 'Completed',
    tone: 'completed',
    blurb: 'All renditions are ready to deliver.',
  },
  failed: {
    label: 'Failed',
    tone: 'failed',
    blurb: 'The job encountered an error. Inspect the logs below.',
  },
}

const formatBytes = (value?: number) => {
  if (!value || Number.isNaN(value)) return '—'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const idx = Math.floor(Math.log(value) / Math.log(1024))
  const size = value / Math.pow(1024, idx)
  return `${size.toFixed(1)} ${units[idx]}`
}

const formatDuration = (value?: number) => {
  if (!value || Number.isNaN(value)) return '—'
  const minutes = Math.floor(value / 60)
  const seconds = Math.floor(value % 60)
  return `${minutes}m ${seconds.toString().padStart(2, '0')}s`
}

const formatDateTime = (value?: string) => {
  if (!value) return '—'
  const dt = new Date(value)
  if (Number.isNaN(dt.valueOf())) return value
  return dt.toLocaleString()
}

export default function VideoDetails() {
  const { videoId } = useParams<{ videoId: string }>()
  const [video, setVideo] = useState<Video | null>(null)
  const [videoProgress, setVideoProgress] = useState<ProcessingProgress | null>(null)
  const [detailsError, setDetailsError] = useState<string | null>(null)

  const loadVideoDetails = useCallback(async (id: string) => {
    if (!id) {
      setVideo(null)
      setDetailsError(null)
      return
    }
    setDetailsError(null)
    try {
      const response = await fetch(`${API_BASE_URL}/videos/${id}`)
      if (!response.ok) {
        throw new Error('Unable to load video details')
      }
      const data: Video = await response.json()
      setVideo(data)
    } catch (error) {
      setDetailsError(error instanceof Error ? error.message : 'Failed to load details')
      toast.error('Failed to load video details')
    }
  }, [])

  useEffect(() => {
    if (videoId) {
      loadVideoDetails(videoId)
    }
  }, [loadVideoDetails, videoId])

  useEffect(() => {
    if (!videoId || typeof window === 'undefined' || !('EventSource' in window)) {
      return
    }

    const source = new EventSource(`${API_BASE_URL}/progress/${videoId}`)

    source.onmessage = (event) => {
      try {
        const payload: ProcessingProgress = JSON.parse(event.data)
        setVideoProgress(payload)
        setVideo((prev) =>
          prev && prev.id === payload.video_id ? { ...prev, status: payload.status } : prev,
        )
        if (payload.status === 'completed' || payload.status === 'failed') {
          loadVideoDetails(payload.video_id)
        }
      } catch (error) {
        console.error('Failed to parse SSE payload', error)
      }
    }

    source.onerror = () => {
      source.close()
    }

    return () => {
      source.close()
    }
  }, [loadVideoDetails, videoId])

  if (!videoId) {
    return (
      <main className="max-w-7xl mx-auto py-8 px-4">
        <p>No video ID provided</p>
      </main>
    )
  }

  return (
    <main className="max-w-7xl mx-auto py-8 px-4">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold mb-2">Video Details</h1>
          <p className="text-muted-foreground">
            View detailed information and processing status for this video.
          </p>
        </div>
        {video?.status === 'completed' && (
          <Button asChild>
            <Link to={`/videos/${videoId}/player`}>Play Video</Link>
          </Button>
        )}
      </div>

      {detailsError && <p className="error mb-4">{detailsError}</p>}

      <section className="details-grid">
        <div className="panel detail-panel">
          <h2>Job details</h2>
          {video && (
            <div className="detail-grid">
              <div>
                <p className="label">Video ID</p>
                <p className="value mono">{video.id}</p>
              </div>
              <div>
                <p className="label">Original name</p>
                <p className="value">{video.original_name}</p>
              </div>
              <div>
                <p className="label">S3 path</p>
                <p className="value mono">{video.s3_path}</p>
              </div>
              <div>
                <p className="label">Source dimensions</p>
                <p className="value">
                  {video.source_width && video.source_height
                    ? `${video.source_width}×${video.source_height}`
                    : '—'}
                </p>
              </div>
              <div>
                <p className="label">Duration</p>
                <p className="value">{formatDuration(video.duration)}</p>
              </div>
              <div>
                <p className="label">File size</p>
                <p className="value">{formatBytes(video.file_size)}</p>
              </div>
              <div>
                <p className="label">Created</p>
                <p className="value">{formatDateTime(video.created_at)}</p>
              </div>
              <div>
                <p className="label">Updated</p>
                <p className="value">{formatDateTime(video.updated_at)}</p>
              </div>
              {video.completed_at && (
                <div>
                  <p className="label">Completed</p>
                  <p className="value">{formatDateTime(video.completed_at)}</p>
                </div>
              )}
              {video.error_message && (
                <div className="full-width">
                  <p className="label">Error</p>
                  <p className="value error-text">{video.error_message}</p>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="panel progress-panel">
          <h2>Progress & renditions</h2>
          {video && (
            <>
              <div className="progress-track">
                <div className="progress-bar" style={{ width: `${videoProgress?.progress ?? (video.status === 'completed' ? 100 : 0)}%` }} />
              </div>
              <div className="progress-meta">
                <span className={`status-pill ${statusMeta[video.status].tone}`}>
                  {statusMeta[video.status].label}
                </span>
                <span>{videoProgress?.message || statusMeta[video.status].blurb}</span>
                {videoProgress?.current_resolution && videoProgress?.total_resolutions && (
                  <span>
                    Rendition {videoProgress.current_resolution} / {videoProgress.total_resolutions}
                  </span>
                )}
              </div>
              <div className="resolutions">
                {video.resolutions && video.resolutions.length > 0 ? (
                  video.resolutions.map((rendition) => (
                    <article key={rendition.id} className="rendition-card">
                      <header>
                        <p className="primary">{rendition.height}p</p>
                        <span className="secondary">{formatBytes(rendition.file_size)}</span>
                      </header>
                      <p className="mono">{rendition.s3_url}</p>
                      <footer>
                        <span>Bitrate: {rendition.bitrate} kbps</span>
                        <span>{formatDateTime(rendition.processed_at)}</span>
                      </footer>
                    </article>
                  ))
                ) : (
                  <p className="muted">No renditions reported yet.</p>
                )}
              </div>
            </>
          )}
        </div>
      </section>
    </main>
  )
}
