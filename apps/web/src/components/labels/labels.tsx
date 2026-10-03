import {
  developmentDemonstrationLabel,
  estimatedCostLabel,
  LABELS,
  outboxEffectLabel,
  recordingLabel,
  replayLabel,
  verdictSourceLabel,
  type EstimatedCost,
} from "@/lib/labels";

import { LabelBadge } from "./label-badge";

// The named label components (WEB-13). Each derives its label from server data and renders nothing
// when the data carries no mark, so a view can place one next to any value without deciding itself.

interface DetailProps {
  detail?: boolean;
  className?: string;
}

/** The simulated outbox: a queued database message is a prototype effect, not email delivery. */
export function SimulatedOutboxLabel({ detail = true, className }: DetailProps) {
  return <LabelBadge label={LABELS.simulatedOutbox} detail={detail} className={className} />;
}

/** The label of an event's effect: the simulated outbox when it queued an outbox message, else nothing. */
export function OutboxEffectLabel({
  effect,
  detail = true,
  className,
}: DetailProps & { effect: string | null | undefined }) {
  const label = outboxEffectLabel(effect);
  return label === null ? null : <LabelBadge label={label} detail={detail} className={className} />;
}

/** A replayed proposal, from the action's or event's `replaySource`; nothing for a model proposal. */
export function ReplayLabel({
  replaySource,
  detail = true,
  className,
}: DetailProps & { replaySource: string | null | undefined }) {
  const replay = replayLabel(replaySource);
  if (replay === null) return null;
  return (
    <LabelBadge
      label={replay.label}
      detail={detail}
      className={className}
      suffix={replay.fixtureId ?? undefined}
    />
  );
}

/** A semantic verdict's source: "Live model" or "Fixture verdict"; nothing when it has no verdict. */
export function VerdictSourceLabel({
  verdictSource,
  detail = false,
  className,
}: DetailProps & { verdictSource: string | null | undefined }) {
  const label = verdictSourceLabel(verdictSource);
  return label === null ? null : <LabelBadge label={label} detail={detail} className={className} />;
}

/** The seeded operator, recognized from the session's email; nothing for any other user. */
export function DevelopmentDemonstrationLabel({
  email,
  detail = false,
  className,
}: DetailProps & { email: string | null | undefined }) {
  const label = developmentDemonstrationLabel(email);
  return label === null ? null : <LabelBadge label={label} detail={detail} className={className} />;
}

/** A cost figure's label; nothing when there is no pricing rule and no unresolved reservation. */
export function EstimatedCostLabel({
  pricingRule,
  unresolved,
  detail = true,
  className,
}: DetailProps & EstimatedCost) {
  const label = estimatedCostLabel({ pricingRule, unresolved });
  return label === null ? null : <LabelBadge label={label} detail={detail} className={className} />;
}

export function TestDoubleLabel({ detail = true, className }: DetailProps) {
  return <LabelBadge label={LABELS.testDouble} detail={detail} className={className} />;
}

export function TestEvidenceLabel({ detail = true, className }: DetailProps) {
  return <LabelBadge label={LABELS.testEvidence} detail={detail} className={className} />;
}

export function SyntheticDataLabel({ detail = false, className }: DetailProps) {
  return <LabelBadge label={LABELS.syntheticData} detail={detail} className={className} />;
}

/** "Recording from build <id>"; nothing without a build id. */
export function RecordingLabel({
  buildId,
  className,
}: {
  buildId: string | null | undefined;
  className?: string;
}) {
  const label = recordingLabel(buildId);
  return label === null ? null : <LabelBadge label={label} detail={false} className={className} />;
}
