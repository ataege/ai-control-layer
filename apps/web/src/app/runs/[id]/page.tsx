"use client";

import { useEffect, useState, use } from "react";
import { ProductClient, getSafeMessage } from "@/lib/product-client";
import { RunView, SanitizedEvent } from "@workspace/contracts";
import { PageHeader } from "@workspace/ui/components/page-header";
import { Button } from "@workspace/ui/components/button";
import { LoadingState } from "@workspace/ui/components/loading-state";
import { ErrorState } from "@workspace/ui/components/error-state";
import { RunTimeline } from "@/components/run-timeline";
import { Card, CardContent, CardHeader, CardTitle } from "@workspace/ui/components/card";

export default function RunPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [run, setRun] = useState<RunView | undefined>(undefined);
  const [events, setEvents] = useState<SanitizedEvent[]>([]);
  const [error, setError] = useState<string | undefined>(undefined);
  const [isInitialLoading, setIsInitialLoading] = useState(true);
  
  useEffect(() => {
    const abortController = new AbortController();
    let isMounted = true;
    let currentCursor: string | undefined = undefined;
    let pollTimeoutId: NodeJS.Timeout;

    async function loadRunAndPollEvents() {
      try {
        // Load the initial run view
        const runResult = await ProductClient.getRun(id, { signal: abortController.signal });
        if (!isMounted) return;
        
        if (!runResult.ok) {
          setError(getSafeMessage(runResult.error));
          setIsInitialLoading(false);
          return;
        }
        
        setRun(runResult.data);
        setIsInitialLoading(false);

        // Start polling events
        async function poll() {
          if (!isMounted) return;
          try {
            const eventsResult = await ProductClient.getRunEvents(id, currentCursor, { signal: abortController.signal });
            if (!isMounted) return;
            
            if (eventsResult.ok) {
              const { events: newEvents, nextCursor } = eventsResult.data;
              if (newEvents.length > 0) {
                setEvents((prev) => [...prev, ...newEvents]);
              }
              currentCursor = nextCursor;
              setError(undefined); // Clear any polling errors if we succeed
            } else {
              // If polling fails, keep real events and show the failure
              setError(getSafeMessage(eventsResult.error));
            }
          } catch (e: any) {
            if (e.name !== 'AbortError' && isMounted) {
              setError("An error occurred while polling for updates.");
            }
          } finally {
            if (isMounted) {
              pollTimeoutId = setTimeout(poll, 3000);
            }
          }
        }
        
        poll();
      } catch (e: any) {
        if (e.name !== 'AbortError' && isMounted) {
          setError("Failed to load run details.");
          setIsInitialLoading(false);
        }
      }
    }

    loadRunAndPollEvents();

    return () => {
      isMounted = false;
      abortController.abort();
      if (pollTimeoutId) clearTimeout(pollTimeoutId);
    };
  }, [id]);

  if (isInitialLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <LoadingState />
      </div>
    );
  }

  // If initial load failed completely (we have an error and no run state)
  if (error && !run) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <ErrorState
          title="Could not load run"
          description={error}
          action={<Button variant="outline" onClick={() => window.location.reload()}>Retry</Button>}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      <PageHeader
        title={`Task Run ${run?.id}`}
        description={`Status: ${run?.status}`}
      />
      
      {/* If we have an error during polling but still have run data, show it here without unmounting the real events */}
      {error && (
        <Card className="border-destructive/50 bg-destructive/5 text-destructive">
          <CardContent className="pt-6">
            <p className="text-sm font-medium">Update Failed</p>
            <p className="text-sm">{error}</p>
          </CardContent>
        </Card>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Details</CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Template</dt>
                  <dd>{run?.passport?.template}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Passport ID</dt>
                  <dd>{run?.passport?.id}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Status</dt>
                  <dd className="font-medium capitalize">{run?.status}</dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>
        
        <div className="space-y-6">
          <RunTimeline events={events} />
        </div>
      </div>
    </div>
  );
}
