// Small coloured status output helpers. Colours are disabled for pipes and NO_COLOR.
const useColor = Boolean(process.stdout.isTTY) && !process.env.NO_COLOR;

const COLOR_CODES = { green: 32, red: 31, yellow: 33, cyan: 36, dim: 2, bold: 1 };

export function colorize(colorName, text) {
  return useColor ? `\x1b[${COLOR_CODES[colorName]}m${text}\x1b[0m` : text;
}

const STATUS_STYLES = {
  ok: { label: "OK", color: "green" },
  warn: { label: "WARN", color: "yellow" },
  fail: { label: "FAIL", color: "red" },
  info: { label: "INFO", color: "cyan" },
  PASS: { label: "PASS", color: "green" },
  FAIL: { label: "FAIL", color: "red" },
  SKIPPED: { label: "SKIPPED", color: "yellow" },
};

/** Prints one "[STATUS] message" line. */
export function printStatus(status, message) {
  const style = STATUS_STYLES[status];
  console.log(`${colorize(style.color, `[${style.label}]`.padEnd(6))} ${message}`);
}

export function printHeading(title) {
  console.log(`\n${colorize("bold", title)}`);
}

/**
 * Prints a result table. Each row is { name, status: "PASS"|"FAIL"|"SKIPPED", detail }.
 */
export function printResultTable(nameColumnTitle, rows) {
  const nameWidth = Math.max(nameColumnTitle.length, ...rows.map((row) => row.name.length));
  const statusWidth = "SKIPPED".length;
  console.log(`${nameColumnTitle.padEnd(nameWidth)}  ${"RESULT".padEnd(statusWidth)}  DETAIL`);
  console.log(`${"-".repeat(nameWidth)}  ${"-".repeat(statusWidth)}  ${"-".repeat(6)}`);
  for (const row of rows) {
    const style = STATUS_STYLES[row.status];
    const statusCell = colorize(style.color, style.label.padEnd(statusWidth));
    console.log(`${row.name.padEnd(nameWidth)}  ${statusCell}  ${row.detail ?? ""}`);
  }
}

/** Counts rows per status, for a one-line summary. */
export function countByStatus(rows) {
  const counts = { PASS: 0, FAIL: 0, SKIPPED: 0 };
  for (const row of rows) counts[row.status] += 1;
  return counts;
}
