import type { Metadata } from "next";
import { PageHeader } from "@workspace/ui/components/page-header";
import { TaskForm } from "@/components/task-form";

export const metadata: Metadata = { title: "New Task" };

export default function NewTaskPage() {
  return (
    <>
      <PageHeader
        title="New Task"
        description="Configure and delegate a new task."
      />
      <div className="max-w-2xl mx-auto mt-6">
        <TaskForm />
      </div>
    </>
  );
}
