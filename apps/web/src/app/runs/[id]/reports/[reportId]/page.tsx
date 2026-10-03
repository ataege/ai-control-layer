import type { Metadata } from "next";
import Link from "next/link";
import { buttonVariants } from "@workspace/ui/components/button";
import { PageHeader } from "@workspace/ui/components/page-header";
import { ReportPanel } from "@/components/report/report-panel";

export const metadata: Metadata = { title: "Report" };

export default async function ReportPage({
  params,
}: {
  params: Promise<{ id: string; reportId: string }>;
}) {
  const { id, reportId } = await params;
  return (
    <>
      <PageHeader
        title="Report"
        description="The stored report of this run, rendered from the gateway's record."
        actions={
          <Link
            href={`/runs/${encodeURIComponent(id)}`}
            className={buttonVariants({ variant: "outline" })}
          >
            Back to run
          </Link>
        }
      />
      <div className="mx-auto mt-6 max-w-3xl">
        <ReportPanel key={`${id}:${reportId}`} runId={id} reportId={reportId} />
      </div>
    </>
  );
}
