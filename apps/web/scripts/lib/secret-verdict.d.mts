export const MODEL_DEPENDENT_DENIALS: string[];

export interface SecretVerdictInput {
  decision: string;
  reasonCode: string | null;
  content?: { text?: string } | null;
  [field: string]: unknown;
}

export function classifySecretVerdict(
  verdict: SecretVerdictInput,
  secretValue: string,
): { status: "pass" | "note" | "fail"; detail: string };
