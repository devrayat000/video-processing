import "@formatjs/intl-durationformat/polyfill";

import { Suspense } from "react";
import { Button } from "@/components/ui/button";
import {
  Await,
  Link,
  useAsyncValue,
  useLoaderData,
  type LoaderFunctionArgs,
  useRevalidator,
  useRouteError,
  useAsyncError,
} from "react-router";
import { API_BASE_URL } from "@/lib/utils";
import type { Video } from "@/types/video";
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";

export async function loader({ request }: LoaderFunctionArgs) {
  const response = await fetch(`${API_BASE_URL}/videos?limit=50`, {
    signal: request.signal,
  });
  if (!response.ok) {
    throw new Error(`Unable to fetch videos (${response.status})`);
  }
  const data: Promise<Video[]> = response.json();
  return {
    videos: data,
  };
}

export function Component() {
  const revalidator = useRevalidator();
  const loaderData = useLoaderData<typeof loader>();

  return (
    <main className="app-shell">
      <section className="grid">
        <div className="panel list-panel">
          <div className="panel-header my-5 flex items-center justify-end">
            <Button
              type="button"
              variant="outline"
              size="default"
              onClick={() => revalidator.revalidate()}
              // disabled={loadingList}
            >
              Refresh
            </Button>
          </div>

          {/* {listError && <p className="error">{listError}</p>} */}

          <Suspense fallback={<div className="p-4">Loading videos...</div>}>
            <Await resolve={loaderData.videos} errorElement={<VideoError />}>
              <VideoList />
            </Await>
          </Suspense>
        </div>
      </section>
    </main>
  );
}
Component.displayName = "DashboardPage";

function VideoError() {
  const error = useAsyncError();

  return (
    <div className="p-4 text-red-600">
      <p>Sorry, an unexpected error has occurred.</p>
      <pre className="whitespace-pre-wrap">{String(error)}</pre>
    </div>
  );
}

function VideoList() {
  const videos = useAsyncValue() as Video[];

  if (videos.length === 0) {
    return <p className="p-4">No videos found.</p>;
  }

  return (
    <section className="flex flex-col gap-y-4">
      {videos.map((video) => {
        return <VideoItem key={video.id} video={video} />;
      })}
    </section>
  );
}

function VideoItem({ video }: { video: Video }) {
  return (
    <Item variant="outline" asChild role="listitem">
      <Link to={`/videos/${video.id}`}>
        <ItemMedia variant="image" className="h-40 w-auto aspect-video">
          <video controls={false} aria-disabled className="rounded-sm">
            <source src={video.s3_path} type={video.content_type} />
          </video>
        </ItemMedia>
        <ItemContent>
          <ItemTitle className="line-clamp-1">{video.original_name}</ItemTitle>
        </ItemContent>
        <ItemContent className="flex-none text-center">
          <ItemDescription>
            {new Intl.DurationFormat("en-US", {
              style: "digital",

              // seconds: "2-digit",
            }).format({
              seconds: Math.floor(video.duration ?? 0),
            })}
          </ItemDescription>
        </ItemContent>
      </Link>
    </Item>
  );
}

export function ErrorBoundary() {
  const error = useRouteError();

  return (
    <div className="app-shell">
      <section className="grid">
        <div className="panel list-panel">
          <div className="panel-header my-5 flex items-center justify-center">
            <h2 className="text-lg font-semibold">Error</h2>
          </div>
          <div className="p-4 text-red-600">
            <p>Sorry, an unexpected error has occurred.</p>
            <pre className="whitespace-pre-wrap">{String(error)}</pre>
          </div>
        </div>
      </section>
    </div>
  );
}
