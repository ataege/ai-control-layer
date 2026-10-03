import type { ReportView } from "@workspace/contracts";
import { CopyButton } from "@workspace/ui/components/copy-button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table";
import { ClassificationBadge } from "./classification-badge";

const DESTINATION_LABELS: Record<ReportView["destinationClass"], string> = {
  internal_reviewers: "Internal reviewers",
  registered_vendor_recipient: "Registered vendor recipient",
};

/**
 * The source trail of a stored report: the template and projection versions it was rendered with,
 * its content hash and, per source, the version, stored classification and the fields consumed.
 * Everything comes from the stored record; nothing is computed here.
 */
export function SourceTrail({ report }: { report: ReportView }) {
  const projection =
    report.projectionRule === null
      ? "None (the internal template reads sources directly)"
      : `${report.projectionRule} (version ${report.projectionRuleVersion})`;
  const facts: [string, string][] = [
    ["Template", `${report.template} (version ${report.templateVersion})`],
    ["Projection", projection],
    ["Destination class", DESTINATION_LABELS[report.destinationClass]],
  ];
  return (
    <section aria-labelledby="source-trail-heading" className="space-y-4">
      <h2 id="source-trail-heading" className="text-sm font-medium">
        Source trail
      </h2>
      <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[max-content_1fr]">
        {facts.map(([term, detail]) => (
          <div key={term} className="contents">
            <dt className="text-muted-foreground">{term}</dt>
            <dd className="font-mono text-xs break-all sm:text-sm">{detail}</dd>
          </div>
        ))}
        <div className="contents">
          <dt className="text-muted-foreground">Content hash</dt>
          <dd className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-xs break-all sm:text-sm">{report.contentHash}</span>
            <CopyButton value={report.contentHash} label="Copy hash" iconOnly size="icon-sm" />
          </dd>
        </div>
      </dl>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Source</TableHead>
            <TableHead>Version</TableHead>
            <TableHead>Classification</TableHead>
            <TableHead>Fields consumed</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {report.lineage.map((source) => (
            <TableRow key={`${source.sourceKind}:${source.sourceId}`}>
              <TableCell className="font-mono text-xs">
                {source.sourceKind} {source.sourceId}
              </TableCell>
              <TableCell>{source.sourceVersion}</TableCell>
              <TableCell>
                <ClassificationBadge classification={source.classification} />
              </TableCell>
              <TableCell className="font-mono text-xs break-words whitespace-normal">
                {source.consumedFields.join(", ")}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  );
}
