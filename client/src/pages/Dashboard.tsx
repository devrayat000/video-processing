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
          <div className="panel-header">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => navigate(0)}
              // disabled={loadingList}
            >
              Refresh
            </Button>
          </div>

          {/* {listError && <p className="error">{listError}</p>}
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
                      <p className="primary">
                        {video.original_name || video.id}
                      </p>
                      <span className="secondary">{video.id}</span>
                    </TableCell>
                    <TableCell>
                      <span
                        className={`status-pill ${
                          statusMeta[video.status].tone
                        }`}
                      >
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
          )} */}

          <Suspense>
            <VideoList videosPromise={loadVideos()} />
          </Suspense>
        </div>
      </section>
    </main>
  );
}

function VideoList({ videosPromise }: { videosPromise: Promise<Video[]> }) {
  const videos = use(videosPromise);

  return (
    <section>
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
        <ItemMedia variant="image">
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
              seconds: parseInt(video.duration),
            })}
          </ItemDescription>
        </ItemContent>
      </Link>
    </Item>
  );
}
