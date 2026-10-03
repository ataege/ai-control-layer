import type { ReportView } from "@workspace/contracts";
import { Badge } from "@workspace/ui/components/badge";

export type ReportClassification = ReportView["classification"];

// The words for the two stored classifications. This is a lookup of the label the gateway stored
// with the report; the interface never derives a label from titles, content or sources.
export const CLASSIFICATION_LABELS: Record<ReportClassification, string> = {
  internal_only: "Internal only",
  vendor_shareable: "Vendor shareable",
};

/** The stored classification of a report or of one of its sources. */
export function ClassificationBadge({ classification }: { classification: ReportClassification }) {
  return (
    <Badge
      data-classification={classification}
      variant={classification === "internal_only" ? "destructive" : "secondary"}
    >
      {CLASSIFICATION_LABELS[classification]}
    </Badge>
  );
}
