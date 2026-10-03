import type { ReportView } from "@workspace/contracts";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { ClassificationBadge } from "./classification-badge";
import { SourceTrail } from "./source-trail";

/** The stored content, or the plain statement that it is withheld. Never reconstructed. */
export function ReportBody({ report }: { report: ReportView }) {
  if (report.contentWithheld || report.content === null) {
    return (
      <div
        role="note"
        data-testid="content-withheld"
        className="rounded-md border border-dashed p-4 text-sm"
      >
        <p className="font-medium">Content withheld</p>
        <p className="mt-1 text-muted-foreground">
          The content of this report is not shown to you. Its identity and the stored record below
          are shown as the gateway holds them; nothing is filled in.
        </p>
      </div>
    );
  }
  // Text only: the content renders as plain text and never as markup.
  return (
    <pre
      data-testid="report-content"
      className="rounded-md border bg-muted/40 p-4 text-sm break-words whitespace-pre-wrap"
    >
      {report.content}
    </pre>
  );
}

/** Identity of the stored report, from its fields only. */
export function ReportIdentity({ report }: { report: ReportView }) {
  const rows: [string, string][] = [
    ["Report", report.reportId],
    ["Run", report.runId],
    ["Version", String(report.version)],
  ];
  return (
    <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[max-content_1fr]">
      {rows.map(([term, detail]) => (
        <div key={term} className="contents">
          <dt className="text-muted-foreground">{term}</dt>
          <dd className="font-mono text-xs break-all sm:text-sm">{detail}</dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * One stored report: its title beside its stored classification, its identity, its content or the
 * withheld statement, and its source trail.
 */
export function ReportContent({ report }: { report: ReportView }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{report.title}</CardTitle>
        <CardDescription>
          Rendered from the stored report by its registered template.
        </CardDescription>
        <CardAction>
          <ClassificationBadge classification={report.classification} />
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-6">
        <ReportIdentity report={report} />
        <ReportBody report={report} />
        <SourceTrail report={report} />
      </CardContent>
    </Card>
  );
}
