import { useActionState, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'

const API_BASE_URL = 'http://localhost:8080'

const safeUUID = () =>
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `video-${Math.random().toString(36).slice(2, 9)}`

type JobForm = {
  videoId: string
  originalName: string
  s3Path: string
}

const defaultFormState = (): JobForm => ({
  videoId: safeUUID(),
  originalName: '',
  s3Path: '',
})

// Server action for submitting jobs
async function submitJob(_prevState: { error?: string; success?: boolean; videoId?: string } | null, formData: FormData) {
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
    return { success: true, videoId: result.id || videoId }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : 'Failed to submit job'
    toast.error(errorMessage)
    return { error: errorMessage }
  }
}

export default function VideoUpload() {
  const navigate = useNavigate()
  const [jobForm, setJobForm] = useState<JobForm>(defaultFormState)
  const [formState, formAction, isPending] = useActionState(submitJob, null)

  const handleInputChange = (field: keyof JobForm) => (event: React.ChangeEvent<HTMLInputElement>) => {
    setJobForm((prev) => ({ ...prev, [field]: event.target.value }))
  }

  const regenerateVideoId = () => {
    setJobForm((prev) => ({ ...prev, videoId: safeUUID() }))
  }

  // Navigate to video details on success
  if (formState?.success && formState.videoId) {
    setTimeout(() => {
      navigate(`/videos/${formState.videoId}`)
    }, 1000)
  }

  return (
    <main className="max-w-4xl mx-auto py-8 px-4">
      <div className="mb-8">
        <h1 className="text-3xl font-bold mb-2">Upload Video</h1>
        <p className="text-muted-foreground">
          Submit a video processing job by providing the S3 path to your video file.
        </p>
      </div>

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
          {formState?.error && (
            <p className="text-sm text-destructive mt-2">{formState.error}</p>
          )}
        </form>
      </div>
    </main>
  )
}
