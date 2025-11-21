import { BrowserRouter, Route, Routes } from "react-router";
import Dashboard from "./pages/Dashboard";
import Layout from "./components/Layout";
import VideoUpload from "./pages/VideoUpload";
import VideoList from "./pages/VideoList";
import VideoDetails from "./pages/VideoDetails";
import VideoPlayer from "./pages/VideoPlayer";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/*" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="upload" element={<VideoUpload />} />
          <Route path="videos" element={<VideoList />} />
          <Route path="videos/:videoId" element={<VideoDetails />} />
          <Route path="videos/:videoId/player" element={<VideoPlayer />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
