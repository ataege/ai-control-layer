import type { Metadata } from "next";
import { PageHeader } from "@workspace/ui/components/page-header";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";

import { SimulatedOutboxLabel, SyntheticDataLabel } from "@/components/labels";
import { TaskForm } from "../components/task-form";

export const metadata: Metadata = { title: "Task Passport" };

// The home page shows the task form and a plain explanation. It shows no run data: a run's real
// events appear on its own page, and a list of runs is not shown because none exists to read.
export default function HomePage() {
  return (
    <>
      <PageHeader
        title="Task Passport"
        description="Delegate one bounded job to an agent, under visible controls over what it can do, what information it can use and what it can spend."
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div>
          <TaskForm />
        </div>
        <div>
          <Card className="h-full">
            <CardHeader>
              <CardTitle>What happens when you start a task</CardTitle>
              <CardDescription>
                Nothing is shown here until you start one; this page never displays sample runs.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <ol className="list-decimal space-y-3 pl-5 text-sm">
                <li>
                  <span className="font-medium">Admission.</span> Your request is checked against
                  your authority. A request that asks for more than you hold is rejected with the
                  reason and what to change; nothing narrows it for you, and nothing runs until you
                  submit a revised request yourself.
                </li>
                <li>
                  <span className="font-medium">A passport.</span> An accepted request is issued as
                  a fixed grant for this task: the invoices, tools, report templates, destination
                  and limits it may use. The agent cannot widen it.
                </li>
                <li>
                  <span className="font-medium">Your run.</span> You are taken to the run&apos;s own
                  page, which shows what really happened: the controls&apos; decisions, any approval
                  waiting for your review, and the effects.
                </li>
              </ol>
              <div className="mt-6 space-y-2 border-t pt-4 text-sm">
                <p className="font-medium">What is real here and what is not</p>
                <div className="flex flex-col gap-2">
                  <SyntheticDataLabel detail />
                  <SimulatedOutboxLabel />
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
