import * as React from "react";

import { cn } from "@workspace/ui/lib/utils";

interface PageHeaderProps extends Omit<React.ComponentProps<"header">, "title"> {
  title: React.ReactNode;
  description?: React.ReactNode;
  /** Buttons or links shown next to the title. */
  actions?: React.ReactNode;
  /** Heading element for the title. Use 2 or 3 when the header is nested inside a page. */
  headingLevel?: 1 | 2 | 3;
}

function PageHeader({
  title,
  description,
  actions,
  headingLevel = 1,
  className,
  ...props
}: PageHeaderProps) {
  const HeadingTag = `h${headingLevel}` as const;

  return (
    <header
      data-slot="page-header"
      className={cn("flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between", className)}
      {...props}
    >
      <div className="flex min-w-0 flex-col gap-1">
        <HeadingTag className="font-heading text-2xl font-semibold tracking-tight text-balance">
          {title}
        </HeadingTag>
        {description ? (
          <p className="max-w-2xl text-sm text-pretty text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  );
}

export { PageHeader };
export type { PageHeaderProps };
