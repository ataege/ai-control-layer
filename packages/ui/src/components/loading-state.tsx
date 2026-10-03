import * as React from "react";
import { LoaderCircleIcon } from "lucide-react";

import { cn } from "@workspace/ui/lib/utils";

interface LoadingStateProps extends React.ComponentProps<"div"> {
  /** Text announced to assistive technology and shown next to the spinner. */
  label?: string;
}

function LoadingState({ label = "Loading", className, ...props }: LoadingStateProps) {
  return (
    <div
      data-slot="loading-state"
      role="status"
      aria-live="polite"
      className={cn(
        "flex items-center justify-center gap-2 px-6 py-10 text-sm text-muted-foreground",
        className,
      )}
      {...props}
    >
      <LoaderCircleIcon aria-hidden="true" className="size-4 animate-spin" />
      <span>{label}</span>
    </div>
  );
}

export { LoadingState };
export type { LoadingStateProps };
