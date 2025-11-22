import { BrowserRouter, Route, Routes } from "react-router";
import Dashboard from "./pages/Dashboard";
import Layout from "./components/Layout";
import VideoUpload from "./pages/VideoUpload";
import VideoList from "./pages/VideoList";
import VideoDetails from "./pages/VideoDetails";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/*" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="upload" element={<VideoUpload />} />
          <Route path="videos">
            <Route index element={<VideoList />} />
            <Route path=":videoId" element={<VideoDetails />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
