import type { Metadata } from "next";
import { PageHeader } from "@workspace/ui/components/page-header";
import { JudgeConsole } from "@/components/judge/judge-console";

export const metadata: Metadata = { title: "Judge Console" };

export default function JudgePage() {
  return (
    <>
      <PageHeader
        title="Judge Console"
        description="Submit your own text or action proposal and see the control layer's decision."
      />
      <div className="mt-6">
        <JudgeConsole />
      </div>
    </>
  );
}
