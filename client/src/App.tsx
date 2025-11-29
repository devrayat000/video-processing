import {
  Route,
  createBrowserRouter,
  RouterProvider,
  createRoutesFromElements,
} from "react-router";
import Layout from "./components/Layout";

const routes = createBrowserRouter(
  createRoutesFromElements(
    <Route path="/" element={<Layout />}>
      <Route index lazy={() => import("./pages/Dashboard")} id="root" />
      <Route path="videos">
        <Route index lazy={() => import("./pages/VideoList")} />
        <Route path=":videoId" lazy={() => import("./pages/VideoDetails")} />
      </Route>
      <Route path="upload" lazy={() => import("./pages/VideoUpload")} />
    </Route>
  )
);

export default function App() {
  return <RouterProvider router={routes} />;
}
