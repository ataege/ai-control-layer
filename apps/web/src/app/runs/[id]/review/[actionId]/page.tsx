import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PageHeader } from "@workspace/ui/components/page-header";

import { ReviewPanel } from "@/components/approval/review-panel";
import { isRecordId } from "@/lib/clients/actions-client";

export const metadata: Metadata = { title: "Review Action" };

// WEB-14: the approval preview of one stored action. The material is read from the server in the
// browser on every visit, so the waiting state survives a reload or a worker restart.
export default async function ReviewActionPage({
  params,
}: {
  params: Promise<{ id: string; actionId: string }>;
}) {
  const { id, actionId } = await params;
  if (!isRecordId(id) || !isRecordId(actionId)) notFound();
  return (
    <>
      <PageHeader
        title="Review Action"
        description="Approve or reject the exact outbound action the agent proposed."
      />
      <div className="mx-auto mt-6 max-w-3xl">
        <ReviewPanel runId={id} actionId={actionId} />
      </div>
    </>
  );
}
