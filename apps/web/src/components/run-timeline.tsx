import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@workspace/ui/components/card";
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
            <div className="text-sm text-muted-foreground italic">No events yet.</div>
          ) : (
            events.map((event) => (
              <div key={event.id} className="flex flex-col space-y-1 pb-4 border-b last:border-0 last:pb-0">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">{event.type}</span>
                  <span className="text-xs text-muted-foreground">
                    {new Date(event.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                {event.details && Object.keys(event.details).length > 0 && (
                  <span className="text-sm text-muted-foreground whitespace-pre-wrap font-mono bg-muted p-2 rounded-md">
                    {JSON.stringify(event.details, null, 2)}
                  </span>
                )}
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  );
}
