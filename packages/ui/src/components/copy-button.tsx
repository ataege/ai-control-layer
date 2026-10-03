"use client";

import * as React from "react";
import { CheckIcon, CopyIcon, TriangleAlertIcon } from "lucide-react";

import { Button } from "@workspace/ui/components/button";

type CopyStatus = "idle" | "copied" | "failed";

// How long the confirmation stays visible before the button resets.
const FEEDBACK_DURATION_MS = 2000;

interface CopyButtonProps extends Omit<
  React.ComponentProps<typeof Button>,
  "value" | "onClick" | "children" | "asChild"
> {
  /** Text written to the clipboard. */
  value: string;
  label?: string;
  copiedLabel?: string;
  failedLabel?: string;
  /** Hide the text and show only the icon (the label stays available to screen readers). */
  iconOnly?: boolean;
}

function CopyButton({
  value,
  label = "Copy",
  copiedLabel = "Copied",
  failedLabel = "Copy failed",
  iconOnly = false,
  variant = "outline",
  size,
  ...props
}: CopyButtonProps) {
  const [copyStatus, setCopyStatus] = React.useState<CopyStatus>("idle");
  const resetTimerRef = React.useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // Drop a pending reset when the button unmounts.
  React.useEffect(() => () => clearTimeout(resetTimerRef.current), []);

  async function handleCopy() {
    let nextStatus: CopyStatus;
    try {
      await navigator.clipboard.writeText(value);
      nextStatus = "copied";
    } catch {
      // Clipboard access can be denied or unavailable on insecure origins.
      nextStatus = "failed";
    }
    setCopyStatus(nextStatus);
    clearTimeout(resetTimerRef.current);
    resetTimerRef.current = setTimeout(() => setCopyStatus("idle"), FEEDBACK_DURATION_MS);
  }

  const visibleLabel =
    copyStatus === "copied" ? copiedLabel : copyStatus === "failed" ? failedLabel : label;
  const StatusIcon =
    copyStatus === "copied" ? CheckIcon : copyStatus === "failed" ? TriangleAlertIcon : CopyIcon;

  return (
    <Button
      type="button"
      data-slot="copy-button"
      data-status={copyStatus}
      variant={variant}
      size={size ?? (iconOnly ? "icon" : "default")}
      onClick={handleCopy}
      {...props}
    >
      <StatusIcon aria-hidden="true" data-icon={iconOnly ? undefined : "inline-start"} />
      <span aria-live="polite" className={iconOnly ? "sr-only" : undefined}>
        {visibleLabel}
      </span>
    </Button>
  );
}

export { CopyButton };
export type { CopyButtonProps };
