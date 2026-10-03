import { Badge } from "@workspace/ui/components/badge";
import { cn } from "@workspace/ui/lib/utils";

import { labelDetail, type LabelText, type LabelTone } from "@/lib/labels";

// One look for every truthful label (WEB-13). The compact badge carries the short text and the full
// sentence as its title; `detail` also prints the rest of the sentence beside it, for a view where the
// reader needs the explanation (the outbox, a replay) and not only the mark.

const TONE_CLASSES: Record<LabelTone, string> = {
  simulation: "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  replay: "border-violet-500/40 bg-violet-500/10 text-violet-700 dark:text-violet-300",
  fixture: "border-sky-500/40 bg-sky-500/10 text-sky-700 dark:text-sky-300",
  live: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  demonstration: "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  estimate: "border-border bg-muted text-muted-foreground",
  synthetic: "border-border bg-muted text-muted-foreground",
  evidence: "border-border bg-muted text-muted-foreground",
};

export interface LabelBadgeProps {
  label: LabelText;
  /** Also show the rest of the specification's sentence next to the badge. */
  detail?: boolean;
  /** Extra text after the label, for example the replayed fixture's id. */
  suffix?: string;
  className?: string;
}

export function LabelBadge({ label, detail = false, suffix, className }: LabelBadgeProps) {
  return (
    <span
      data-label={label.id}
      className={cn("inline-flex flex-wrap items-center gap-x-2 gap-y-1", className)}
    >
      <Badge variant="outline" className={TONE_CLASSES[label.tone]} title={label.full}>
        {label.short}
        {suffix === undefined ? null : <span className="font-normal opacity-80">: {suffix}</span>}
      </Badge>
      {detail ? <span className="text-xs text-muted-foreground">{labelDetail(label)}</span> : null}
    </span>
  );
}
