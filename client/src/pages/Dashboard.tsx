import { useCallback, useEffect, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'

const API_BASE_URL = 'http://localhost:8080'

type VideoStatus = 'pending' | 'processing' | 'completed' | 'failed'

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

export default function Dashboard() {
  const navigate = useNavigate()
  const [videos, setVideos] = useState<Video[]>([])
  const [loadingList, setLoadingList] = useState(true)
  const [listError, setListError] = useState<string | null>(null)

  const summary = useMemo(() => {
    const counts = { pending: 0, processing: 0, completed: 0, failed: 0 }
    videos.forEach((video) => {
      counts[video.status] += 1
    })
    return counts
  }, [videos])

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
    } catch (error) {
      setListError(error instanceof Error ? error.message : 'Failed to load videos')
      toast.error('Failed to load videos')
    } finally {
      setLoadingList(false)
    }
  }, [])

  useEffect(() => {
    loadVideos()
    const interval = setInterval(loadVideos, 15000)
    return () => clearInterval(interval)
  }, [loadVideos])

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
        <div className="panel list-panel">
          <div className="panel-header">
            <div>
              <h2>Recent Jobs</h2>
              <p>Shows the 50 most recent jobs. Click a row to view details.</p>
            </div>
            <Button type="button" variant="outline" size="sm" onClick={loadVideos} disabled={loadingList}>
              Refresh
            </Button>
          </div>

          {listError && <p className="error">{listError}</p>}
          {loadingList ? (
            <div className="skeleton">Loading jobs…</div>
          ) : videos.length === 0 ? (
            <div className="empty-state">
              <p>No jobs yet.</p>
              <span>Submit a video to get started.</span>
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
                {videos.map((video) => (
                  <TableRow
                    key={video.id}
                    onClick={() => navigate(`/videos/${video.id}`)}
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
    </main>
  )
}
