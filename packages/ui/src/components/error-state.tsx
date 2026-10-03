import * as React from "react";
import { CircleAlertIcon } from "lucide-react";

import { cn } from "@workspace/ui/lib/utils";

interface ErrorStateProps extends Omit<React.ComponentProps<"div">, "title"> {
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Optional recovery control, for example a retry button. */
  action?: React.ReactNode;
}

function ErrorState({ title, description, action, className, ...props }: ErrorStateProps) {
  return (
    <div
      data-slot="error-state"
      role="alert"
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-6 py-10 text-center",
        className,
      )}
      {...props}
    >
      <div
        aria-hidden="true"
        className="flex size-10 items-center justify-center rounded-lg bg-destructive/10 text-destructive"
      >
        <CircleAlertIcon className="size-5" />
      </div>
      <div className="flex flex-col gap-1">
        <p className="text-sm font-medium">{title}</p>
        {description ? (
          <p className="max-w-sm text-sm text-pretty text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {action}
    </div>
  );
}

export { ErrorState };
export type { ErrorStateProps };
