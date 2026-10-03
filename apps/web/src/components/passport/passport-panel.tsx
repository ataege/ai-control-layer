import type { Passport } from "@workspace/contracts";
import { Badge } from "@workspace/ui/components/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";

import { formatInstant, limitRows } from "./passport-view";

interface PassportPanelProps {
  /** The run's passport as the gateway stored it (X-08). */
  passport: Passport;
}

/** A labelled group of values; an empty group says so instead of disappearing. */
function ScopeGroup({
  title,
  values,
  monospace = false,
  emphasis = "secondary",
}: {
  title: string;
  values: readonly string[];
  monospace?: boolean;
  emphasis?: "secondary" | "outline";
}) {
  return (
    <div className="space-y-1.5">
      <h4 className="text-xs font-medium text-muted-foreground">{title}</h4>
      {values.length === 0 ? (
        <p className="text-sm text-muted-foreground">None</p>
      ) : (
        <ul className="flex flex-wrap gap-1.5">
          {values.map((value) => (
            <li key={value}>
              <Badge variant={emphasis} className={monospace ? "font-mono" : undefined}>
                {value}
              </Badge>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * The Task Passport of a run: what the run may use and the ceilings it runs under. Display only;
 * the gateway enforces the passport, and nothing here grants or changes it.
 */
export function PassportPanel({ passport }: PassportPanelProps) {
  const { scope, limits } = passport;
  return (
    <Card data-testid="passport-panel">
      <CardHeader>
        <CardTitle>Task Passport</CardTitle>
        <CardDescription>
          Expires {formatInstant(passport.expiresAt)} · issued {formatInstant(passport.issuedAt)} ·
          admitted under catalog revision {passport.admissionCatalogRevisionId}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <section aria-label="Scope" className="space-y-4">
          <ScopeGroup title="Allowed tools" values={scope.tools} monospace />
          <ScopeGroup
            title="Tools that need approval"
            values={scope.approvalRequiredTools}
            monospace
          />
          <ScopeGroup title="Invoices" values={scope.invoiceIds} monospace />
          <ScopeGroup title="Vendors" values={scope.vendorIds} monospace />
          <ScopeGroup title="Report templates" values={scope.reportTemplates} monospace />
          <ScopeGroup
            title="Recipient references (not addresses)"
            values={scope.recipientReferences}
            monospace
            emphasis="outline"
          />
          <ScopeGroup title="Projection rules" values={scope.projectionRules} monospace />
          <ScopeGroup title="Allowed models" values={scope.allowedModels} monospace />
          <p className="text-sm">
            <span className="text-muted-foreground">Internal note readable: </span>
            {scope.internalNoteReadable ? "yes, for investigation" : "no"}
          </p>
        </section>
        <section aria-label="Limits" className="space-y-1.5">
          <h4 className="text-xs font-medium text-muted-foreground">Limits</h4>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
            {limitRows(limits).map((row) => (
              <div key={row.label} className="flex items-baseline justify-between gap-3">
                <dt className="text-muted-foreground">{row.label}</dt>
                <dd className="font-medium tabular-nums">{row.value}</dd>
              </div>
            ))}
          </dl>
        </section>
      </CardContent>
    </Card>
  );
}
