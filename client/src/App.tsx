import { useActionState, useCallback, useEffect, useMemo, useState, useOptimistic } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { toast } from 'sonner'
import './App.css'

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

type JobForm = {
  videoId: string
  originalName: string
  s3Path: string
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

const safeUUID = () =>
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `video-${Math.random().toString(36).slice(2, 9)}`

const defaultFormState = (): JobForm => ({
  videoId: safeUUID(),
  originalName: '',
  s3Path: '',
})

// Server action for submitting jobs
async function submitJob(_prevState: { error?: string; success?: boolean } | null, formData: FormData) {
  const videoId = formData.get('videoId') as string
  const originalName = formData.get('originalName') as string
  const s3Path = formData.get('s3Path') as string

  if (!originalName || !s3Path) {
    return { error: 'Original name and S3 path are required.' }
  }

  try {
    const response = await fetch(`${API_BASE_URL}/jobs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        video_id: videoId,
        original_name: originalName,
        s3_path: s3Path,
      }),
    })

    if (!response.ok) {
      const message = await response.text()
      throw new Error(message || 'Failed to submit job')
    }

    const result = await response.json()
    toast.success(`Job ${result.id || videoId} queued successfully.`)
    return { success: true }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : 'Failed to submit job'
    toast.error(errorMessage)
    return { error: errorMessage }
  }
}

function App() {
  const [videos, setVideos] = useState<Video[]>([])
  const [selectedVideoId, setSelectedVideoId] = useState<string | null>(null)
  const [selectedVideoDetails, setSelectedVideoDetails] = useState<Video | null>(null)
  const [videoProgress, setVideoProgress] = useState<ProcessingProgress | null>(null)
  const [loadingList, setLoadingList] = useState(true)
  const [listError, setListError] = useState<string | null>(null)
  const [detailsError, setDetailsError] = useState<string | null>(null)
  const [jobForm, setJobForm] = useState<JobForm>(defaultFormState)

  // Use useActionState for form submission
  const [formState, formAction, isPending] = useActionState(submitJob, null)

  // Use useOptimistic for optimistic updates
  const [optimisticVideos, addOptimisticVideo] = useOptimistic(
    videos,
    (state, newVideo: Video) => [newVideo, ...state]
  )

  const selectedVideo = useMemo(
    () => selectedVideoDetails ?? videos.find((video) => video.id === selectedVideoId) ?? null,
    [selectedVideoDetails, selectedVideoId, videos],
  )

  const summary = useMemo(() => {
    const counts = { pending: 0, processing: 0, completed: 0, failed: 0 }
    optimisticVideos.forEach((video) => {
      counts[video.status] += 1
    })
    return counts
  }, [optimisticVideos])

  const loadVideos = useCallback(async () => {
    setLoadingList(true)
    setListError(null)
    try {
      const response = await fetch(`${API_BASE_URL}/videos?limit=50`)
      if (!response.ok) {
        throw new Error(`Unable to fetch videos (${response.status})`)
      }
      const data: Video[] = await response.json()
      setVideos(data)
      if (!selectedVideoId && data.length > 0) {
        setSelectedVideoId(data[0].id)
      } else if (selectedVideoId && !data.some((video) => video.id === selectedVideoId)) {
        setSelectedVideoId(data[0]?.id ?? null)
      }
    } catch (error) {
      setListError(error instanceof Error ? error.message : 'Failed to load videos')
      toast.error('Failed to load videos')
    } finally {
      setLoadingList(false)
    }
  }, [selectedVideoId])

  const loadVideoDetails = useCallback(async (videoId: string) => {
    if (!videoId) {
      setSelectedVideoDetails(null)
      setDetailsError(null)
      return
    }
    setDetailsError(null)
    try {
      const response = await fetch(`${API_BASE_URL}/videos/${videoId}`)
      if (!response.ok) {
        throw new Error('Unable to load video details')
      }
      const data: Video = await response.json()
      setSelectedVideoDetails(data)
    } catch (error) {
      setDetailsError(error instanceof Error ? error.message : 'Failed to load details')
      toast.error('Failed to load video details')
    }
  }, [])

  useEffect(() => {
    loadVideos()
    const interval = setInterval(loadVideos, 15000)
    return () => clearInterval(interval)
  }, [loadVideos])

  useEffect(() => {
    if (selectedVideoId) {
      loadVideoDetails(selectedVideoId)
      setVideoProgress(null)
    } else {
      setSelectedVideoDetails(null)
    }
  }, [loadVideoDetails, selectedVideoId])

  useEffect(() => {
    if (!selectedVideoId || typeof window === 'undefined' || !('EventSource' in window)) {
      return
    }

    const source = new EventSource(`${API_BASE_URL}/progress/${selectedVideoId}`)

    source.onmessage = (event) => {
      try {
        const payload: ProcessingProgress = JSON.parse(event.data)
        setVideoProgress(payload)
        setVideos((prev) =>
          prev.map((video) =>
            video.id === payload.video_id ? { ...video, status: payload.status } : video,
          ),
        )
        setSelectedVideoDetails((prev) =>
          prev && prev.id === payload.video_id ? { ...prev, status: payload.status } : prev,
        )
        if (payload.status === 'completed' || payload.status === 'failed') {
          loadVideos()
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
  }, [loadVideoDetails, loadVideos, selectedVideoId])

  // Reset form on successful submission
  useEffect(() => {
    if (formState?.success) {
      setJobForm(defaultFormState())
      loadVideos()
    }
  }, [formState, loadVideos])

  const handleFormChange = (field: keyof JobForm, value: string) => {
    setJobForm((prev) => ({ ...prev, [field]: value }))
  }

  const handleInputChange = (field: keyof JobForm) => (event: React.ChangeEvent<HTMLInputElement>) => {
    handleFormChange(field, event.target.value)
  }

  const regenerateVideoId = () => {
    setJobForm((prev) => ({ ...prev, videoId: safeUUID() }))
  }

  return (
    <main className="app-shell">
      <header className="hero">
        <div>
          <p className="eyebrow">Video Workflow Console</p>
          <h1>Launch, monitor, and troubleshoot every transcoding job.</h1>
          <p className="lede">
            Submit raw assets by S3 path, track worker progress in real time, and inspect the renditions that
            roll out of your pipeline. The API is already running on port 8080—this UI is your mission control.
          </p>
        </div>
        <div className="metrics">
          {(['pending', 'processing', 'completed', 'failed'] as VideoStatus[]).map((status) => (
            <div key={status} className="metric-card">
              <p className="metric-label">{statusMeta[status].label}</p>
              <p className="metric-value">{summary[status as keyof typeof summary]}</p>
              <span className="metric-blurb">{statusMeta[status].blurb}</span>
            </div>
          ))}
        </div>
      </header>

      <section className="grid">
        <div className="panel form-panel">
          <div className="panel-header">
            <div>
              <h2>Submit a processing job</h2>
              <p>Provide a globally unique ID and the S3 object that should be processed by the backend.</p>
            </div>
            <Button type="button" variant="outline" size="sm" onClick={regenerateVideoId}>
              Generate ID
            </Button>
          </div>
          <form className="job-form" action={formAction}>
            <div className="form-field">
              <Label htmlFor="videoId">Video ID</Label>
              <Input
                id="videoId"
                name="videoId"
                value={jobForm.videoId}
                onChange={handleInputChange('videoId')}
                required
              />
              <span className="hint">Must be unique per job. Prefilled for convenience.</span>
            </div>
            <div className="form-field">
              <Label htmlFor="originalName">Original file name</Label>
              <Input
                id="originalName"
                name="originalName"
                value={jobForm.originalName}
                placeholder="eg. corporate_reel.mov"
                onChange={handleInputChange('originalName')}
                required
              />
            </div>
            <div className="form-field">
              <Label htmlFor="s3Path">S3 path</Label>
              <Input
                id="s3Path"
                name="s3Path"
                value={jobForm.s3Path}
                placeholder="s3://bucket/key/video.mov"
                onChange={handleInputChange('s3Path')}
                required
              />
              <span className="hint">The worker pulls bytes directly from this URI.</span>
            </div>
            <div className="form-actions">
              <Button type="submit" disabled={isPending}>
                {isPending ? 'Queuing...' : 'Queue job'}
              </Button>
            </div>
          </form>
        </div>

        <div className="panel list-panel">
          <div className="panel-header">
            <div>
              <h2>Live queue</h2>
              <p>Shows the 50 most recent jobs. Select a row for deep details.</p>
            </div>
            <Button type="button" variant="outline" size="sm" onClick={loadVideos} disabled={loadingList}>
              Refresh
            </Button>
          </div>

          {listError && <p className="error">{listError}</p>}
          {loadingList ? (
            <div className="skeleton">Loading jobs…</div>
          ) : optimisticVideos.length === 0 ? (
            <div className="empty-state">
              <p>No jobs yet.</p>
              <span>Submit a video above to get started.</span>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Video</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {optimisticVideos.map((video) => (
                  <TableRow
                    key={video.id}
                    data-state={video.id === selectedVideoId ? 'selected' : undefined}
                    onClick={() => setSelectedVideoId(video.id)}
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
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </section>

      <section className="details-grid">
        <div className="panel detail-panel">
          <h2>Job details</h2>
          {!selectedVideoId && <p>Select a job to see metadata.</p>}
          {detailsError && <p className="error">{detailsError}</p>}
          {selectedVideo && (
            <div className="detail-grid">
              <div>
                <p className="label">Video ID</p>
                <p className="value mono">{selectedVideo.id}</p>
              </div>
              <div>
                <p className="label">Original name</p>
                <p className="value">{selectedVideo.original_name}</p>
              </div>
              <div>
                <p className="label">S3 path</p>
                <p className="value mono">{selectedVideo.s3_path}</p>
              </div>
              <div>
                <p className="label">Source dimensions</p>
                <p className="value">
                  {selectedVideo.source_width && selectedVideo.source_height
                    ? `${selectedVideo.source_width}×${selectedVideo.source_height}`
                    : '—'}
                </p>
              </div>
              <div>
                <p className="label">Duration</p>
                <p className="value">{formatDuration(selectedVideo.duration)}</p>
              </div>
              <div>
                <p className="label">File size</p>
                <p className="value">{formatBytes(selectedVideo.file_size)}</p>
              </div>
              <div>
                <p className="label">Created</p>
                <p className="value">{formatDateTime(selectedVideo.created_at)}</p>
              </div>
              <div>
                <p className="label">Updated</p>
                <p className="value">{formatDateTime(selectedVideo.updated_at)}</p>
              </div>
              {selectedVideo.completed_at && (
                <div>
                  <p className="label">Completed</p>
                  <p className="value">{formatDateTime(selectedVideo.completed_at)}</p>
                </div>
              )}
              {selectedVideo.error_message && (
                <div className="full-width">
                  <p className="label">Error</p>
                  <p className="value error-text">{selectedVideo.error_message}</p>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="panel progress-panel">
          <h2>Progress & renditions</h2>
          {!selectedVideo && <p>Select a job to subscribe to live progress updates.</p>}
          {selectedVideo && (
            <>
              <div className="progress-track">
                <div className="progress-bar" style={{ width: `${videoProgress?.progress ?? (selectedVideo.status === 'completed' ? 100 : 0)}%` }} />
              </div>
              <div className="progress-meta">
                <span className={`status-pill ${statusMeta[selectedVideo.status].tone}`}>
                  {statusMeta[selectedVideo.status].label}
                </span>
                <span>{videoProgress?.message || statusMeta[selectedVideo.status].blurb}</span>
                {videoProgress?.current_resolution && videoProgress?.total_resolutions && (
                  <span>
                    Rendition {videoProgress.current_resolution} / {videoProgress.total_resolutions}
                  </span>
                )}
              </div>
              <div className="resolutions">
                {selectedVideo.resolutions && selectedVideo.resolutions.length > 0 ? (
                  selectedVideo.resolutions.map((rendition) => (
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

export default App
