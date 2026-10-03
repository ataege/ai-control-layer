// A node:test reporter that writes one JSON line per finished test, so `pnpm verify:controls` can
// read the fixture self-checks as machine-readable results (node has no built-in JSON reporter).
export default async function* jsonLinesReporter(source) {
  for await (const event of source) {
    if (event.type !== "test:pass" && event.type !== "test:fail") continue;
    // Suites report too; only leaf tests are cases.
    if (event.data.details?.type === "suite") continue;
    yield `${JSON.stringify({
      outcome: event.type === "test:pass" ? "pass" : "fail",
      name: event.data.name,
      file: event.data.file,
      durationMs: event.data.details?.duration_ms ?? null,
      skipped: Boolean(event.data.skip || event.data.todo),
    })}\n`;
  }
}
