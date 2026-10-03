import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@workspace/ui/components/card";

const MOCK_EVENTS = [
  { id: 1, type: "START", message: "Task requested by user", time: "10:00 AM" },
  { id: 2, type: "TOOL_CALL", message: "Called read_invoice", time: "10:01 AM" },
  { id: 3, type: "TOOL_RESULT", message: "Invoice data extracted", time: "10:02 AM" },
  { id: 4, type: "FINISH", message: "Task completed successfully", time: "10:05 AM" },
];

export function RunTimeline() {
  return (
    <Card className="h-full">
      <CardHeader>
        <CardTitle>Run Timeline</CardTitle>
        <CardDescription>Live execution events for the current task.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {MOCK_EVENTS.map((event) => (
            <div key={event.id} className="flex flex-col space-y-1 pb-4 border-b last:border-0 last:pb-0">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">{event.type}</span>
                <span className="text-xs text-muted-foreground">{event.time}</span>
              </div>
              <span className="text-sm text-muted-foreground">{event.message}</span>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
