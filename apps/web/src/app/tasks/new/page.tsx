import type { Metadata } from "next";
import { PageHeader } from "@workspace/ui/components/page-header";
import { TaskForm } from "@/components/task-form";

export const metadata: Metadata = { title: "New Task" };

export default function NewTaskPage() {
  return (
    <>
      <PageHeader title="New Task" description="Configure and delegate a new task." />
      <div className="mx-auto mt-6 max-w-2xl">
        <TaskForm />
      </div>
    </>
  );
}
