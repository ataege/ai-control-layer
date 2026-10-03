import type { Metadata } from "next";

import { PageHeader } from "@workspace/ui/components/page-header";

import { AuditExport } from "@/components/security/audit-export";

export const metadata: Metadata = { title: "Audit export" };

// Authorization is decided by the API when the page loads; nothing here may be prerendered.
export const dynamic = "force-dynamic";

export default function AuditExportPage() {
  return (
    <>
      <PageHeader
        title="Audit export"
        description="Download sanitized audit records as JSON or CSV, one page at a time. Reviewers only."
      />
      <div className="mt-6">
        <AuditExport />
      </div>
    </>
  );
}
