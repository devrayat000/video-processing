import { Fragment } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router'
import './index.css'
import { Toaster } from '@/components/ui/sonner'
import Layout from '@/components/Layout'
import Dashboard from '@/pages/Dashboard'
import VideoUpload from '@/pages/VideoUpload'
import VideoList from '@/pages/VideoList'
import VideoDetails from '@/pages/VideoDetails'
import VideoPlayer from '@/pages/VideoPlayer'

createRoot(document.getElementById('root')!).render(
  <Fragment>
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/upload" element={<VideoUpload />} />
          <Route path="/videos" element={<VideoList />} />
          <Route path="/videos/:videoId" element={<VideoDetails />} />
          <Route path="/videos/:videoId/player" element={<VideoPlayer />} />
        </Route>
      </Routes>
    </BrowserRouter>
    <Toaster />
  </Fragment>,
)
