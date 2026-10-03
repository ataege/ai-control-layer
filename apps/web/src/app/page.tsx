import type { Metadata } from "next";
import { PageHeader } from "@workspace/ui/components/page-header";
import { TaskForm } from "../components/task-form";
import { RunTimeline } from "../components/run-timeline";

export const metadata: Metadata = { title: "Task Passport" };

export default function HomePage() {
  return (
    <>
      <PageHeader
        title="Task Passport"
        description="Create and monitor delegated tasks."
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div>
          <TaskForm />
        </div>
        <div>
          <RunTimeline events={[]} />
        </div>
      </div>
    </>
  );
}
