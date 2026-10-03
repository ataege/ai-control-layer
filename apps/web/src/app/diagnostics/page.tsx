import type { Metadata } from "next";

import { DiagnosticsPanel } from "./diagnostics-panel";

export const metadata: Metadata = { title: "Diagnostics" };

// Health is fetched by the browser at view time; nothing here may be prerendered.
export const dynamic = "force-dynamic";

export default function DiagnosticsPage() {
  return <DiagnosticsPanel />;
}
