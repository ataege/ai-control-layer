"use client";

import Link from "next/link";
import { useEffect, useId, useState } from "react";
import { ArrowLeftIcon, DownloadIcon, LoaderCircleIcon } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { ErrorState } from "@workspace/ui/components/error-state";
import { Input } from "@workspace/ui/components/input";
import { Label } from "@workspace/ui/components/label";
import { LoadingState } from "@workspace/ui/components/loading-state";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select";

import {
  buildExportQuery,
  EXPORT_LIMIT_DEFAULT,
  EXPORT_LIMIT_MAXIMUM,
  EXPORT_LIMIT_MINIMUM,
  exportFileName,
  fetchExportPage,
  probeExportAuthorization,
  type ExportAuthorization,
  type ExportFormat,
  type ExportKind,
  type ExportPage,
  type SecurityFailure,
} from "@/lib/clients/security-client";

import { previewOfPage } from "./export-preview";
import { saveTextFile } from "./save-text-file";

interface FetchedPage {
  kind: ExportKind;
  after: string;
  limit: string;
  page: ExportPage;
}

/** The authorized audit export (WEB-31): the API decides who may export; the control is hidden otherwise. */
export function AuditExport() {
  const [authorization, setAuthorization] = useState<ExportAuthorization | "checking">("checking");

  useEffect(() => {
    const controller = new AbortController();
    void probeExportAuthorization({ signal: controller.signal }).then((answer) => {
      if (!controller.signal.aborted) setAuthorization(answer);
    });
    return () => controller.abort();
  }, []);

  if (authorization === "checking") return <LoadingState label="Checking export authorization…" />;
  if (authorization === "forbidden") return <Refusal />;
  if (authorization === "unauthorized") {
    return (
      <ErrorState
        title="Sign in to export audit records"
        description="Your session has ended."
        action={
          <Button asChild>
            <Link href="/login?callbackUrl=/security/export">Sign in</Link>
          </Button>
        }
      />
    );
  }
  if (authorization === "unavailable") {
    return (
      <ErrorState
        title="The export could not be checked"
        description="The security records are not available right now, so export authorization could not be confirmed."
        action={
          <Button variant="outline" onClick={() => window.location.reload()}>
            Try again
          </Button>
        }
      />
    );
  }
  return <ExportForm />;
}

/** What a viewer without the reviewer role sees: a refusal, and no export control at all. */
function Refusal() {
  return (
    <div className="flex flex-col gap-4">
      <Alert variant="destructive" role="alert">
        <AlertTitle>The reviewer role is required (403)</AlertTitle>
        <AlertDescription>
          This account cannot export audit records, so no export control is shown. Ask a reviewer of
          your organization, or use the security posture page, which any signed-in operator can
          read.
        </AlertDescription>
      </Alert>
      <div>
        <Button variant="outline" asChild>
          <Link href="/security">
            <ArrowLeftIcon aria-hidden="true" />
            Security posture
          </Link>
        </Button>
      </div>
    </div>
  );
}

function ExportForm() {
  const formId = useId();
  const [kind, setKind] = useState<ExportKind>("events");
  const [after, setAfter] = useState("");
  const [limit, setLimit] = useState(String(EXPORT_LIMIT_DEFAULT));
  const [busy, setBusy] = useState<"fetch" | ExportFormat | null>(null);
  const [fetched, setFetched] = useState<FetchedPage | null>(null);
  const [failure, setFailure] = useState<SecurityFailure | null>(null);
  const [saved, setSaved] = useState<string | null>(null);

  const query = buildExportQuery({ kind, format: "json", after, limit });
  const afterProblem = query.ok ? null : (query.problems.after ?? null);
  const limitProblem = query.ok ? null : (query.problems.limit ?? null);

  function clearResult() {
    setFetched(null);
    setFailure(null);
    setSaved(null);
  }

  async function fetchPage() {
    if (!query.ok) return;
    setBusy("fetch");
    clearResult();
    const result = await fetchExportPage({ kind, format: "json", after, limit });
    setBusy(null);
    if (result.ok) setFetched({ kind, after, limit, page: result.page });
    else setFailure(result.failure);
  }

  async function download(format: ExportFormat) {
    if (!fetched) return;
    setBusy(format);
    setFailure(null);
    setSaved(null);
    const parameters = { kind: fetched.kind, format, after: fetched.after, limit: fetched.limit };
    // JSON is the page already read, byte for byte; CSV asks the API for the same page as CSV.
    const result =
      format === "json"
        ? ({ ok: true, page: fetched.page } as const)
        : await fetchExportPage(parameters);
    setBusy(null);
    if (!result.ok) {
      setFailure(result.failure);
      return;
    }
    const fileName = exportFileName(parameters);
    saveTextFile(fileName, result.page.body, result.page.contentType);
    setSaved(fileName);
  }

  return (
    <div className="flex flex-col gap-6">
      <Alert>
        <AlertTitle>Sanitized records only</AlertTitle>
        <AlertDescription>
          The export holds stable codes, revisions and masked metadata: no prompts, notes, addresses
          or model text. A download is saved exactly as the server sent it.
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Choose a page</CardTitle>
          <CardDescription>
            Pages follow the gateway&apos;s window cursor: each holds records in id order, and every
            committed record appears exactly once across pages.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="grid gap-4 sm:grid-cols-3"
            onSubmit={(event) => {
              event.preventDefault();
              void fetchPage();
            }}
            noValidate
          >
            <div className="flex flex-col gap-2">
              <Label htmlFor={`${formId}-kind`}>Records</Label>
              <Select
                value={kind}
                onValueChange={(value) => {
                  setKind(value as ExportKind);
                  clearResult();
                }}
              >
                <SelectTrigger id={`${formId}-kind`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="events">Decision events</SelectItem>
                  <SelectItem value="assessments">Control assessments</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`${formId}-after`}>After cursor (optional)</Label>
              <Input
                id={`${formId}-after`}
                value={after}
                onChange={(event) => {
                  setAfter(event.target.value);
                  clearResult();
                }}
                placeholder="v1.0.0.0"
                autoComplete="off"
                spellCheck={false}
                aria-invalid={afterProblem !== null}
                aria-describedby={afterProblem ? `${formId}-after-problem` : undefined}
              />
              {afterProblem ? (
                <p id={`${formId}-after-problem`} className="text-xs text-destructive">
                  {afterProblem}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor={`${formId}-limit`}>Records per page</Label>
              <Input
                id={`${formId}-limit`}
                value={limit}
                onChange={(event) => {
                  setLimit(event.target.value);
                  clearResult();
                }}
                inputMode="numeric"
                autoComplete="off"
                aria-invalid={limitProblem !== null}
                aria-describedby={`${formId}-limit-help`}
              />
              <p
                id={`${formId}-limit-help`}
                className={
                  limitProblem ? "text-xs text-destructive" : "text-xs text-muted-foreground"
                }
              >
                {limitProblem ?? `${EXPORT_LIMIT_MINIMUM} to ${EXPORT_LIMIT_MAXIMUM}`}
              </p>
            </div>
            <div className="sm:col-span-3">
              <Button type="submit" disabled={!query.ok || busy !== null}>
                {busy === "fetch" ? (
                  <LoaderCircleIcon aria-hidden="true" className="animate-spin" />
                ) : null}
                Read this page
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {failure ? <FailureAlert failure={failure} /> : null}
      {fetched ? (
        <PageResult
          fetched={fetched}
          busy={busy}
          saved={saved}
          onDownload={(format) => void download(format)}
          onNext={() => {
            setAfter(fetched.page.nextCursor ?? "");
            clearResult();
          }}
        />
      ) : null}

      <div>
        <Button variant="outline" asChild>
          <Link href="/security">
            <ArrowLeftIcon aria-hidden="true" />
            Security posture
          </Link>
        </Button>
      </div>
    </div>
  );
}

function FailureAlert({ failure }: { failure: SecurityFailure }) {
  return (
    <Alert variant="destructive" role="alert">
      <AlertTitle>
        {failure.kind === "forbidden"
          ? "The reviewer role is required (403)"
          : "The export did not complete"}
      </AlertTitle>
      <AlertDescription>{failure.message}</AlertDescription>
    </Alert>
  );
}

function PageResult({
  fetched,
  busy,
  saved,
  onDownload,
  onNext,
}: {
  fetched: FetchedPage;
  busy: "fetch" | ExportFormat | null;
  saved: string | null;
  onDownload: (format: ExportFormat) => void;
  onNext: () => void;
}) {
  const { page } = fetched;
  const preview = previewOfPage(page.body);
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">
          {page.recordCount ?? 0} {fetched.kind === "events" ? "events" : "assessments"} read
        </CardTitle>
        <CardDescription>
          Next cursor <code className="font-mono text-xs">{page.nextCursor}</code>
          {page.requestId ? <> · request {page.requestId}</> : null}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => onDownload("json")} disabled={busy !== null}>
            <DownloadIcon aria-hidden="true" />
            Download JSON
          </Button>
          <Button variant="secondary" onClick={() => onDownload("csv")} disabled={busy !== null}>
            {busy === "csv" ? (
              <LoaderCircleIcon aria-hidden="true" className="animate-spin" />
            ) : (
              <DownloadIcon aria-hidden="true" />
            )}
            Download CSV
          </Button>
          <Button variant="outline" onClick={onNext} disabled={busy !== null}>
            Next page
          </Button>
        </div>
        {saved ? (
          <p role="status" className="text-sm text-muted-foreground">
            Saved <code className="font-mono text-xs">{saved}</code>
          </p>
        ) : null}
        <p className="text-xs text-muted-foreground">
          CSV asks the API for the same page as CSV; spreadsheet formula characters are neutralized
          there. The preview is indented for reading; both downloads are saved byte for byte as
          served.
        </p>
        <pre
          tabIndex={0}
          aria-label="Preview of the JSON page"
          className="max-h-72 overflow-auto rounded-lg border bg-muted/40 p-3 font-mono text-xs"
        >
          {preview}
        </pre>
      </CardContent>
    </Card>
  );
}
