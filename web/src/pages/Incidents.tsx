import { useCallback, useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { Layers, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useProjectStore } from "@/store/project";
import { incidentService } from "@/services/incident.service";
import { IncidentTimeline } from "@/components/activity/IncidentTimeline";
import { usePageTitle } from "@/utils/usePageTitle";
import type { Incident } from "@/types";

// Deliberately small. The timeline is a scrolling feed, so a page only needs to cover
// roughly a screenful - fetching more just delays the first paint.
const PAGE_SIZE = 15;

function NoProjectSelected() {
  return (
    <div className='flex flex-col items-center justify-center py-24 text-center'>
      <div className='mb-4 flex size-11 items-center justify-center rounded-xl border border-rim bg-surface'>
        <Layers className='size-4 text-ink-faint' />
      </div>
      <p className='text-sm font-semibold text-ink'>No project selected</p>
      <p className='mt-1 max-w-xs text-xs text-ink-faint'>
        Select a project from the sidebar to view its incidents.
      </p>
    </div>
  );
}

export function Incidents() {
  usePageTitle("Incidents");
  const { activeProject } = useProjectStore();
  const queryClient = useQueryClient();
  const [resolvingId, setResolvingId] = useState<number | null>(null);
  const projectKey = activeProject?.Key ?? "";

  const { data, isLoading, isError, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery({
      queryKey: ["incidents", projectKey],
      enabled: !!projectKey,
      initialPageParam: 1,
      // Picks up incidents opened or resolved elsewhere. Note this refetches every page
      // loaded so far, not just the newest, so it is deliberately slower than a single-page
      // feed would need - the running durations already tick client-side without it.
      refetchInterval: 30_000,
      queryFn: ({ pageParam }) =>
        incidentService.list(projectKey, { page: pageParam, per_page: PAGE_SIZE }),
      getNextPageParam: (lastPage, allPages) => {
        const loaded = allPages.reduce((sum, p) => sum + p.incidents.length, 0);
        return loaded < lastPage.total ? allPages.length + 1 : undefined;
      },
    });

  const incidents = data?.pages.flatMap((p) => p.incidents) ?? [];

  // A callback ref rather than an effect: the sentinel only exists once there is a page to
  // hang it under, and an effect with a ref that starts null would never get a second
  // chance to observe it.
  const observerRef = useRef<IntersectionObserver | null>(null);
  const sentinelRef = useCallback(
    (node: HTMLDivElement | null) => {
      observerRef.current?.disconnect();
      observerRef.current = null;
      if (!node) return;

      const observer = new IntersectionObserver((entries) => {
        // Guarded on both flags so scrolling past the sentinel repeatedly cannot queue a
        // second request for a page already in flight.
        if (entries[0]?.isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      });
      observer.observe(node);
      observerRef.current = observer;
    },
    [hasNextPage, isFetchingNextPage, fetchNextPage],
  );

  async function handleResolve(incident: Incident) {
    setResolvingId(incident.ID);
    try {
      await incidentService.resolve(projectKey, incident.ID);
      await queryClient.invalidateQueries({ queryKey: ["incidents", projectKey] });
      // The dashboard's open-incident count is derived from the same data.
      await queryClient.invalidateQueries({ queryKey: ["incident-counts", projectKey] });
      toast.success("Incident marked as resolved.");
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        "Failed to resolve incident.";
      toast.error(msg);
    } finally {
      setResolvingId(null);
    }
  }

  if (!activeProject) return <NoProjectSelected />;

  return (
    <div className='space-y-4'>
      <div>
        <h1 className='text-base font-semibold text-ink'>Incidents</h1>
        <p className='mt-0.5 text-xs text-ink-faint'>{activeProject.Name}</p>
        <p className='mt-1 max-w-xl text-[0.6875rem] text-ink-faint'>
          Every failure paired with its recovery. An incident opens when something breaks and
          closes when the matching all-clear arrives. Ongoing incidents are listed first.
        </p>
      </div>

      <IncidentTimeline
        incidents={incidents}
        isLoading={isLoading}
        isError={isError}
        onResolve={(incident) => void handleResolve(incident)}
        resolvingId={resolvingId}
        footer={
          hasNextPage && (
            <div ref={sentinelRef} className='flex items-center justify-center gap-2 py-6 text-[0.6875rem] text-ink-faint'>
              {isFetchingNextPage && <Loader2 className='size-3 animate-spin' />}
              {isFetchingNextPage ? "Loading more" : ""}
            </div>
          )
        }
      />
    </div>
  );
}
