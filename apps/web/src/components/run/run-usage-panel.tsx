import type { RunUsage } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { LabelBadge } from "@/components/labels";
import { Badge } from "@workspace/ui/components/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table";
import { describeUsage, formatCount, type UsageView } from "./usage-model";

/**
 * WEB-19: shown whenever a request has no usable usage report. Its reserved tokens stay held, so
 * the real usage is unknown: the figure is uncertain, not zero, and not a measured amount.
 */
export function UnknownUsageNotice({ view }: { view: UsageView }) {
  if (!view.uncertain) {
    return null;
  }
  return (
    <Alert data-part="unknown-usage" data-uncertain="true">
      <AlertTitle className="flex flex-wrap items-center gap-2">
        Usage is uncertain
        {view.uncertainLabel !== null ? <LabelBadge label={view.uncertainLabel} /> : null}
      </AlertTitle>
      <AlertDescription>
        <p>
          {view.unknownCalls} model {view.unknownCalls === 1 ? "request has" : "requests have"} no
          usable usage report. The tokens reserved for them stay held and are not counted as used or
          as zero; the true usage is unknown until it is reconciled.
        </p>
        {view.uncertainLabel !== null ? (
          <p>
            <LabelBadge label={view.uncertainLabel} detail />
          </p>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

/**
 * WEB-16: the usage of one run with reported tokens (measured by the provider), reserved allowance
 * (held, not used) and cost kept apart. Limits come from the run's ledger, never from constants, and
 * no cost is shown because none is estimated.
 */
export function RunUsagePanel({ usage }: { usage: RunUsage }) {
  const view = describeUsage(usage);
  const { ledger } = view;
  return (
    <Card data-part="usage">
      <CardHeader>
        <CardTitle>Model usage</CardTitle>
        <CardDescription>
          Reported tokens were measured by the model provider. Reserved tokens are allowance held
          for requests that are running or whose usage is unknown; they are not usage.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <UnknownUsageNotice view={view} />
        <Table>
          <TableCaption className="mt-0 mb-2 caption-top text-left font-medium text-foreground">
            Requests and tokens by purpose
          </TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Purpose</TableHead>
              <TableHead>Requests recorded</TableHead>
              <TableHead>Reported tokens</TableHead>
              <TableHead>Reserved tokens (held)</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {view.purposes.map((row) => (
              <TableRow key={row.purpose} data-purpose={row.purpose} data-uncertain={row.uncertain}>
                <TableCell>{row.label}</TableCell>
                <TableCell>
                  {row.dispatched}
                  <span className="block text-xs text-muted-foreground">
                    {row.completed} completed · {row.failed} failed or refused before sending ·{" "}
                    {row.inFlight} running · {row.unknownCalls} with unknown usage
                  </span>
                </TableCell>
                <TableCell data-cell="reported">
                  {formatCount(row.reportedTokens)}
                  {row.uncertain ? (
                    <Badge variant="secondary" className="ml-2">
                      + uncertain
                    </Badge>
                  ) : null}
                </TableCell>
                <TableCell data-cell="reserved">{formatCount(row.reservedTokens)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {ledger === null ? (
          <p data-part="no-ledger" className="text-sm text-muted-foreground">
            No allowance ledger exists for this run yet, so nothing was reserved or reported.
          </p>
        ) : (
          <>
            {ledger.paused ? (
              <p className="text-sm font-medium" data-part="ledger-paused">
                The allowance ledger is paused: no further model request is reserved.
              </p>
            ) : null}
            <Table>
              <TableCaption className="mt-0 mb-2 caption-top text-left font-medium text-foreground">
                Token allowance, from this run&apos;s own limits
              </TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead>Allowance</TableHead>
                  <TableHead>Limit</TableHead>
                  <TableHead>Reported</TableHead>
                  <TableHead>Reserved</TableHead>
                  <TableHead>Available</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ledger.allowances.map((row) => (
                  <TableRow key={row.name}>
                    <TableCell>{row.name}</TableCell>
                    <TableCell>
                      {row.limit === null ? "No separate limit" : formatCount(row.limit)}
                    </TableCell>
                    <TableCell>{formatCount(row.reported)}</TableCell>
                    <TableCell>{formatCount(row.reserved)}</TableCell>
                    <TableCell>
                      {row.available === null ? "—" : formatCount(row.available)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <Table>
              <TableCaption className="mt-0 mb-2 caption-top text-left font-medium text-foreground">
                Request limits
              </TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead>Requests</TableHead>
                  <TableHead>Counted</TableHead>
                  <TableHead>Limit</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ledger.calls.map((row) => (
                  <TableRow key={row.name} data-reached={row.reached}>
                    <TableCell>{row.name}</TableCell>
                    <TableCell>{row.used}</TableCell>
                    <TableCell>
                      {row.limit}
                      {row.reached ? (
                        <Badge variant="outline" className="ml-2">
                          reached
                        </Badge>
                      ) : null}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <p className="text-xs text-muted-foreground">
              Request time limit {ledger.requestTimeoutSeconds} s · at most{" "}
              {ledger.maxConcurrentCalls} requests at once ({ledger.callsInFlight} slots held). A
              request counts when it is reserved and is never refunded.
            </p>
          </>
        )}
        <p data-part="cost" className="text-sm text-muted-foreground">
          {view.costNote}
        </p>
        <p className="text-xs text-muted-foreground" data-part="tool-attempts">
          Tool attempts: {view.toolAttempts.total} in total · {view.toolAttempts.succeeded}{" "}
          succeeded · {view.toolAttempts.failed} failed · {view.toolAttempts.aborted} aborted ·{" "}
          {view.toolAttempts.open} without a recorded outcome.
        </p>
      </CardContent>
    </Card>
  );
}
