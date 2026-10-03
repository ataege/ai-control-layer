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
              <CardTitle>Passport Summary</CardTitle>
              <div className="text-sm text-muted-foreground">
                Records and recipients outside the passport are excluded.
              </div>
            </CardHeader>
            <CardContent>
              <dl className="space-y-3 text-sm">
                <div className="flex flex-col gap-1 border-b pb-2">
                  <dt className="text-muted-foreground text-xs uppercase font-semibold">Goal</dt>
                  <dd>{run?.passport?.template || "N/A"}</dd>
                </div>
                
                <div className="flex flex-col gap-1 border-b pb-2">
                  <dt className="text-muted-foreground text-xs uppercase font-semibold">Allowed Scope</dt>
                  <dd>
                    <div className="grid grid-cols-2 gap-2 mt-1">
                      <span className="text-muted-foreground">Invoices:</span>
                      <span>{run?.passport?.invoiceIds?.length ? run.passport.invoiceIds.join(', ') : 'None'}</span>
                      <span className="text-muted-foreground">Vendor:</span>
                      <span>{run?.passport?.vendorId || 'None'}</span>
                      <span className="text-muted-foreground">Recipient:</span>
                      <span>{run?.passport?.destination || 'None'}</span>
                    </div>
                  </dd>
                </div>

                <div className="flex flex-col gap-1 border-b pb-2">
                  <dt className="text-muted-foreground text-xs uppercase font-semibold">Rules & Approvals</dt>
                  <dd>
                    <div className="grid grid-cols-2 gap-2 mt-1">
                      <span className="text-muted-foreground">Approval:</span>
                      <span>{run?.passport?.approvalRequirement || 'None'}</span>
                      <span className="text-muted-foreground">Rules:</span>
                      <span>{run?.passport?.rules?.length ? run.passport.rules.join(', ') : 'None'}</span>
                    </div>
                  </dd>
                </div>

                <div className="flex flex-col gap-1 border-b pb-2">
                  <dt className="text-muted-foreground text-xs uppercase font-semibold">Allowance & Expiry</dt>
                  <dd>
                    <div className="grid grid-cols-2 gap-2 mt-1">
                      <span className="text-muted-foreground">Used calls:</span>
                      <span>{run?.usage?.modelCalls} / {run?.passport?.limits?.modelCalls || '∞'}</span>
                      <span className="text-muted-foreground">Timeout:</span>
                      <span>{run?.passport?.limits?.timeoutSeconds ? `${run.passport.limits.timeoutSeconds}s` : 'None'}</span>
                      <span className="text-muted-foreground">Expires at:</span>
                      <span>{run?.passport?.expiresAt ? new Date(run.passport.expiresAt).toLocaleString() : 'None'}</span>
                    </div>
                  </dd>
                </div>

                <div className="flex flex-col gap-1 border-b pb-2">
                  <dt className="text-muted-foreground text-xs uppercase font-semibold">Versions</dt>
                  <dd>
                    <div className="grid grid-cols-2 gap-2 mt-1">
                      <span className="text-muted-foreground">Task:</span>
                      <span>{run?.passport?.versions?.task || 'N/A'}</span>
                      <span className="text-muted-foreground">Policy:</span>
                      <span>{run?.passport?.versions?.policy || 'N/A'}</span>
                    </div>
                  </dd>
                </div>
                
                <div className="flex justify-between pt-2">
                  <dt className="text-muted-foreground text-xs uppercase font-semibold">Status</dt>
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
