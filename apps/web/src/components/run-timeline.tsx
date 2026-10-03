import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@workspace/ui/components/card";
import { Badge } from "@workspace/ui/components/badge";
import { SanitizedEvent } from "@workspace/contracts";

export function RunTimeline({ events }: { events: SanitizedEvent[] }) {
  return (
    <Card className="h-full">
      <CardHeader>
        <CardTitle>Run Timeline</CardTitle>
        <CardDescription>Execution events for this task.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {events.length === 0 ? (
            <div className="text-sm text-muted-foreground italic bg-muted p-4 rounded-md text-center">No events yet.</div>
          ) : (
            events.map((event) => (
              <div key={event.id} className="flex flex-col space-y-2 pb-4 border-b last:border-0 last:pb-0">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold">{event.type || 'Unknown'}</span>
                    {event.details.isReplay && <Badge variant="secondary" className="text-[10px] h-5 px-1.5">REPLAY</Badge>}
                    {event.details.decision === 'denied' && <Badge variant="destructive" className="text-[10px] h-5 px-1.5">DENIED</Badge>}
                    {event.details.decision === 'allowed' && <Badge variant="default" className="text-[10px] h-5 px-1.5">ALLOWED</Badge>}
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {new Date(event.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                
                {/* Attempted Operation Details */}
                {event.details.action && (
                  <div className="text-sm">
                    <span className="font-medium text-foreground">Action: </span>
                    <span className="text-muted-foreground">{event.details.action}</span>
                    {event.details.tool && (
                      <span className="text-muted-foreground"> (Tool: {event.details.tool})</span>
                    )}
                  </div>
                )}
                
                {/* Denial & Correction Details */}
                {event.details.decision === 'denied' && (
                  <div className="bg-destructive/5 text-destructive p-3 rounded-md space-y-1 mt-1 text-sm border border-destructive/20">
                    <div className="font-medium">Attempt Denied</div>
                    {event.details.reasonCode && <div>Reason: <span className="font-mono text-xs bg-destructive/10 px-1 py-0.5 rounded">{event.details.reasonCode}</span></div>}
                    {event.details.rule && <div>Rule applied: <span className="font-mono text-xs bg-destructive/10 px-1 py-0.5 rounded">{event.details.rule}</span></div>}
                    {event.details.correctionRoute && <div>Correction route: <span className="font-mono text-xs bg-destructive/10 px-1 py-0.5 rounded">{event.details.correctionRoute}</span></div>}
                    {event.details.correctionCount !== undefined && <div className="text-muted-foreground mt-1 text-xs">Correction attempt {event.details.correctionCount}</div>}
                  </div>
                )}

                {/* General Message */}
                {event.details.message && event.details.decision !== 'denied' && (
                  <div className="text-sm text-muted-foreground">
                    {event.details.message}
                  </div>
                )}
                
                {/* Fallback for raw details (excluding ones we already render) */}
                {Object.keys(event.details).filter(k => !['message', 'tool', 'action', 'decision', 'reasonCode', 'rule', 'correctionRoute', 'correctionCount', 'isReplay'].includes(k)).length > 0 && (
                  <div className="mt-2 text-xs text-muted-foreground whitespace-pre-wrap font-mono bg-muted/50 p-2 rounded-md">
                    {JSON.stringify(Object.fromEntries(
                      Object.entries(event.details).filter(([k]) => !['message', 'tool', 'action', 'decision', 'reasonCode', 'rule', 'correctionRoute', 'correctionCount', 'isReplay'].includes(k))
                    ), null, 2)}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  );
}
