import type { Metadata } from "next";

import { PageHeader } from "@workspace/ui/components/page-header";

import { ActiveControls } from "@/components/security/active-controls";
import { SecurityDashboard } from "@/components/security/security-dashboard";

export const metadata: Metadata = { title: "Security posture" };

// The summary is read by the browser at view time; nothing here may be prerendered.
export const dynamic = "force-dynamic";

export default function SecurityPage() {
  return (
    <>
      <PageHeader
        title="Security posture"
        description="What the controls decided for this organization: allowed, blocked and redacted, which controls fired, what the model consumed and how long each phase took."
      />
      <div className="mt-6 flex flex-col gap-6">
        <ActiveControls />
        <SecurityDashboard />
      </div>
    </>
  );
}
