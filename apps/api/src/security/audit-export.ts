import { z } from "zod";
import recordContract from "@workspace/contracts/schemas/assessment-record.schema.json" with { type: "json" };
import assessmentContract from "@workspace/contracts/schemas/assessment-page.schema.json" with { type: "json" };
import eventContract from "@workspace/contracts/schemas/security-event-page.schema.json" with { type: "json" };
import type { AssessmentRecord } from "@workspace/contracts";
import { SafeEventSchema } from "../runs/run-events.schema.js";

export function validWindowCursor(cursor: string): boolean {
  if (!/^v1\.[0-9]{1,20}\.[0-9]{1,20}\.[0-9]{1,19}$/.test(cursor)) return false;
  const [, lowText, highText, idText] = cursor.split(".");
  const low = BigInt(lowText!);
  const high = BigInt(highText!);
  const id = BigInt(idText!);
  return (
    low <= 18446744073709551615n &&
    high <= 18446744073709551615n &&
    id <= 9223372036854775807n &&
    (high === 0n ? id === 0n : high > low)
  );
}

const cursorSchema = z.string().refine(validWindowCursor);
export const ExportQuerySchema = z.strictObject({
  kind: z.enum(["events", "assessments"]),
  format: z.enum(["json", "csv"]).optional(),
  after: cursorSchema.optional(),
  limit: z
    .string()
    .regex(/^[1-9][0-9]{0,2}$/)
    .refine((value) => Number(value) <= 500)
    .optional(),
});

const { allOf: conditions, ...recordShape } = recordContract;
void conditions;
const assessmentSchema = z
  .fromJSONSchema(recordShape as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const record = value as AssessmentRecord;
    const mustBeUnclassified =
      record.controlClass === "deterministic" || record.verdictSource === null;
    if (
      mustBeUnclassified &&
      (record.verdictSource !== null ||
        record.verdict !== null ||
        record.securityModelCallId !== null)
    ) {
      context.addIssue({ code: "custom", message: "Invalid assessment verdict source" });
    }
    if (
      record.controlClass === "semantic" &&
      record.verdictSource === null &&
      record.outcome !== "not_applicable"
    ) {
      context.addIssue({ code: "custom", message: "Unclassified semantic assessment" });
    }
  });

// The keys and bounds are the shared Go page schemas, with their record refs resolved strictly.
export const AssessmentPageSchema = z.strictObject({
  records: z.array(assessmentSchema).max(assessmentContract.properties.records.maxItems),
  nextCursor: cursorSchema.regex(new RegExp(assessmentContract.properties.nextCursor.pattern)),
});
export const SecurityEventPageSchema = z.strictObject({
  events: z.array(SafeEventSchema).max(eventContract.properties.events.maxItems),
  nextCursor: cursorSchema.regex(new RegExp(eventContract.properties.nextCursor.pattern)),
});

/** Serialize only existing record fields; nested metadata remains one JSON cell. */
export function auditCsv(records: object[], columns: string[]): string {
  const cell = (value: unknown): string => {
    let text =
      value === null || value === undefined
        ? ""
        : typeof value === "object"
          ? JSON.stringify(value)
          : typeof value === "string"
            ? value
            : typeof value === "number" || typeof value === "boolean"
              ? String(value)
              : "";
    // Neutralize formula triggers even when prefixed with whitespace/control characters.
    const firstContent = [...text].find(
      (character) => character.charCodeAt(0) > 32 && !/\s/u.test(character),
    );
    if (firstContent && "=+-@".includes(firstContent)) text = "'" + text;
    return '"' + text.replaceAll('"', '""') + '"';
  };
  return (
    [
      columns.map(cell).join(","),
      ...records.map((record) =>
        columns.map((key) => cell((record as Record<string, unknown>)[key])).join(","),
      ),
    ].join("\r\n") + "\r\n"
  );
}
