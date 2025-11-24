import "@formatjs/intl-durationformat/polyfill";

import { cache, Suspense, use } from "react";
import { Button } from "@/components/ui/button";
import { Link, useNavigate } from "react-router";
import { API_BASE_URL } from "@/lib/utils";
import type { Video } from "@/types/video";
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";

const loadVideos = cache(async () => {
  const response = await fetch(`${API_BASE_URL}/videos?limit=50`);
  if (!response.ok) {
    throw new Error(`Unable to fetch videos (${response.status})`);
  }
  const data: Video[] = await response.json();
  return data;
});

export default function Dashboard() {
  const navigate = useNavigate();

  return (
    <main className="app-shell">
      <section className="grid">
        <div className="panel list-panel">
          <div className="panel-header my-5 flex items-center justify-end">
            <Button
              type="button"
              variant="outline"
              size="default"
              onClick={() => navigate(0)}
              // disabled={loadingList}
            >
              Refresh
            </Button>
          </div>

          {/* {listError && <p className="error">{listError}</p>} */}

          <Suspense fallback={<div className="p-4">Loading videos...</div>}>
            <VideoList videosPromise={loadVideos()} />
          </Suspense>
        </div>
      </section>
    </main>
  );
}

function VideoList({ videosPromise }: { videosPromise: Promise<Video[]> }) {
  const videos = use(videosPromise);

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
