"use client";

import { useEffect, useState } from "react";
import { RefreshCwIcon } from "lucide-react";

import { Badge } from "@workspace/ui/components/badge";
import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { CopyButton } from "@workspace/ui/components/copy-button";
import { PageHeader } from "@workspace/ui/components/page-header";
import { Skeleton } from "@workspace/ui/components/skeleton";
import { cn } from "@workspace/ui/lib/utils";

import { fetchJson, type FetchJsonResult } from "@/lib/fetch-json";
import {
  describeApiLiveness,
  describeApiReadiness,
  describeGatewayDatabase,
  describeGatewayReachability,
  type ServiceCheckOutcome,
  type ServiceCheckState,
} from "@/lib/service-checks";

type ProxyRouteKey = "liveness" | "readiness" | "gatewayDiagnostics";

const PROXY_ROUTES: Record<ProxyRouteKey, string> = {
  liveness: "/api/health/live",
  readiness: "/api/health/ready",
  gatewayDiagnostics: "/api/diagnostics/gateway",
};

// undefined = still waiting for that response.
type ProxyResults = Record<ProxyRouteKey, FetchJsonResult<unknown> | undefined>;

const PENDING_RESULTS: ProxyResults = {
  liveness: undefined,
  readiness: undefined,
  gatewayDiagnostics: undefined,
};

interface ServiceCheckDefinition {
  title: string;
  description: string;
  source: ProxyRouteKey;
  describe: (result: FetchJsonResult<unknown>) => ServiceCheckOutcome;
}

const SERVICE_CHECKS: readonly ServiceCheckDefinition[] = [
  {
    title: "API liveness",
    description: "The API process answers requests.",
    source: "liveness",
    describe: describeApiLiveness,
  },
  {
    title: "API readiness",
    description: "The API can query its PostgreSQL database.",
    source: "readiness",
    describe: describeApiReadiness,
  },
  {
    title: "Gateway reachability",
    description: "The API can call the gateway with its service token.",
    source: "gatewayDiagnostics",
    describe: describeGatewayReachability,
  },
  {
    title: "Gateway database readiness",
    description: "The gateway can reach its PostgreSQL database.",
    source: "gatewayDiagnostics",
    describe: describeGatewayDatabase,
  },
];

const STATE_PRESENTATION: Record<ServiceCheckState, { label: string; badgeClassName: string }> = {
  healthy: {
    label: "Healthy",
    badgeClassName:
      "bg-emerald-600/10 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300",
  },
  degraded: {
    label: "Degraded",
    badgeClassName: "bg-amber-500/15 text-amber-700 dark:bg-amber-400/15 dark:text-amber-300",
  },
  unavailable: {
    label: "Unavailable",
    badgeClassName: "bg-destructive/10 text-destructive",
  },
  error: {
    label: "Error",
    badgeClassName: "border-destructive/40 bg-transparent text-destructive",
  },
};

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className="flex min-w-0 items-center gap-1 text-right">{children}</dd>
    </div>
  );
}

function PendingCheckBody() {
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-2">
      <span className="sr-only">Checking</span>
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-4 w-1/2" />
      <Skeleton className="h-4 w-2/3" />
    </div>
  );
}

function CheckOutcomeBody({ outcome }: { outcome: ServiceCheckOutcome }) {
  return (
    <div className="flex flex-col gap-3">
      <p>{outcome.summary}</p>
      <dl className="flex flex-col gap-1.5 border-t pt-3">
        <DetailRow label={outcome.httpStatusLabel ?? "HTTP status"}>
          {outcome.httpStatus ?? "No response"}
        </DetailRow>
        {outcome.latencyMs !== undefined && outcome.latencyLabel ? (
          <DetailRow label={outcome.latencyLabel}>{outcome.latencyMs} ms</DetailRow>
        ) : null}
        {outcome.facts.map((fact) => (
          <DetailRow key={fact.label} label={fact.label}>
            {fact.value}
          </DetailRow>
        ))}
        <DetailRow label="Request ID">
          {outcome.requestId ? (
            <>
              <code className="truncate font-mono text-xs">{outcome.requestId}</code>
              <CopyButton
                value={outcome.requestId}
                label="Copy request ID"
                iconOnly
                variant="ghost"
                size="icon-xs"
              />
            </>
          ) : (
            "Not available"
          )}
        </DetailRow>
      </dl>
    </div>
  );
}

function ServiceCheckCard({
  definition,
  result,
}: {
  definition: ServiceCheckDefinition;
  result: FetchJsonResult<unknown> | undefined;
}) {
  // No result yet means no outcome: a state is only derived from a real response.
  const outcome = result ? definition.describe(result) : undefined;
  const presentation = outcome ? STATE_PRESENTATION[outcome.state] : undefined;

  return (
    <Card data-check-state={outcome?.state ?? "loading"}>
      <CardHeader>
        <CardTitle>{definition.title}</CardTitle>
        <CardDescription>{definition.description}</CardDescription>
        <CardAction>
          {presentation ? (
            <Badge className={cn(presentation.badgeClassName)}>{presentation.label}</Badge>
          ) : (
            <Badge variant="outline">Checking</Badge>
          )}
        </CardAction>
      </CardHeader>
      <CardContent>
        {outcome ? <CheckOutcomeBody outcome={outcome} /> : <PendingCheckBody />}
      </CardContent>
    </Card>
  );
}

export function DiagnosticsPanel() {
  const [proxyResults, setProxyResults] = useState<ProxyResults>(PENDING_RESULTS);
  const [refreshCount, setRefreshCount] = useState(0);
  const [lastCompletedAt, setLastCompletedAt] = useState<Date | undefined>(undefined);

  useEffect(() => {
    const abortController = new AbortController();
    const routeKeys = Object.keys(PROXY_ROUTES) as ProxyRouteKey[];

    // Each card updates as soon as its own response arrives.
    const pendingRequests = routeKeys.map(async (routeKey) => {
      const result = await fetchJson<unknown>(PROXY_ROUTES[routeKey], {
        signal: abortController.signal,
      });
      if (!abortController.signal.aborted) {
        setProxyResults((previousResults) => ({ ...previousResults, [routeKey]: result }));
      }
    });
    void Promise.all(pendingRequests).then(() => {
      if (!abortController.signal.aborted) {
        setLastCompletedAt(new Date());
      }
    });

    return () => abortController.abort();
  }, [refreshCount]);

  const isChecking = Object.values(proxyResults).some((result) => result === undefined);

  function handleRefresh() {
    // Clear old results first so a stale state is never shown during the new run.
    setProxyResults(PENDING_RESULTS);
    setRefreshCount((previousCount) => previousCount + 1);
  }

  return (
    <>
      <PageHeader
        title="Service diagnostics"
        description="Live checks through the web server's proxy routes. Every state below comes from a real response."
        actions={
          <Button variant="outline" onClick={handleRefresh} disabled={isChecking}>
            <RefreshCwIcon
              aria-hidden="true"
              data-icon="inline-start"
              className={cn(isChecking && "animate-spin")}
            />
            {isChecking ? "Checking" : "Refresh"}
          </Button>
        }
      />

      <div className="grid gap-4 lg:grid-cols-2">
        {SERVICE_CHECKS.map((definition) => (
          <ServiceCheckCard
            key={definition.title}
            definition={definition}
            result={proxyResults[definition.source]}
          />
        ))}
      </div>

      <p aria-live="polite" className="text-xs text-muted-foreground">
        {isChecking || !lastCompletedAt
          ? "Checks in progress."
          : `Last checked at ${lastCompletedAt.toLocaleTimeString()}.`}
      </p>
    </>
  );
}
