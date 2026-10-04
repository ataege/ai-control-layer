// Singular and plural wording for counts shown to the operator: "1 message", never "1 messages".
// Exactly 1 takes the singular; 0 and every other count take the plural ("0 messages", "2 messages").

/** The singular for a count of exactly 1, the plural for every other count. */
export function pluralize(count: number, singular: string, plural: string): string {
  return count === 1 ? singular : plural;
}

/** A count with its noun: "1 message", "0 messages", "2 messages". The plural defaults to adding "s". */
export function countOf(count: number, singular: string, plural: string = `${singular}s`): string {
  return `${count} ${pluralize(count, singular, plural)}`;
}
