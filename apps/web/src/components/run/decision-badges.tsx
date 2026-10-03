import { Badge } from "@workspace/ui/components/badge";
import type { ControlResult, DecisionBadge } from "./event-model";

const RESULT_VARIANT: Record<ControlResult, "destructive" | "secondary" | "outline"> = {
  passed: "outline",
  blocked: "destructive",
  redacted: "secondary",
  not_applicable: "outline",
  unavailable: "destructive",
  unknown: "outline",
};

/**
 * Hybrid control decisions of one event (WEB-32): which control (deterministic, signature or
 * semantic), what it decided, and for a semantic verdict whether it came from the live model or
 * from a labelled fixture. Display only: the badges come from the stored events and assessments.
 */
export function DecisionBadges({ badges }: { badges: readonly DecisionBadge[] }) {
  if (badges.length === 0) {
    return null;
  }
  return (
    <ul className="mt-2 flex flex-col gap-1" aria-label="Control decisions">
      {badges.map((badge, index) => (
        <li
          key={`${badge.family}-${index}`}
          data-family={badge.family}
          data-result={badge.result}
          className="flex flex-col gap-0.5"
        >
          <span className="flex flex-wrap items-center gap-1.5">
            <Badge variant={RESULT_VARIANT[badge.result]}>{badge.text}</Badge>
            {badge.sourceLabel !== null ? (
              <Badge variant="secondary" title={badge.sourceLabel.full}>
                {badge.sourceLabel.short}
              </Badge>
            ) : null}
            {badge.result === "not_applicable" ? (
              <span className="text-xs text-muted-foreground">
                Nothing to classify here, so no model verdict was requested.
              </span>
            ) : null}
          </span>
          {badge.sourceLabel !== null ? (
            <span className="text-xs text-muted-foreground">{badge.sourceLabel.full}</span>
          ) : null}
          {badge.basis === "reason_code" ? (
            <span className="text-xs text-muted-foreground">
              From the decision&apos;s reason code; the exact controls are on the security page.
            </span>
          ) : null}
          {badge.consequence !== null ? (
            <span className="text-xs text-muted-foreground">{badge.consequence}</span>
          ) : null}
          {badge.detail !== null && badge.detail !== "" ? (
            <span className="text-xs text-muted-foreground">{badge.detail}</span>
          ) : null}
        </li>
      ))}
    </ul>
  );
}
