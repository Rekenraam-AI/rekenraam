// Corporate-action correction forms (#311) show a few source-evidence fields
// (reference, ex date) of a recorded object that may hold JSON numbers or
// metadata the browser cannot round-trip exactly. A correction therefore
// sends only the fields the owner changed, and the server applies them to the
// recorded evidence: a cleared field removes its key (null), an unchanged one
// is not sent at all.

/** The text a form pre-fills for a recorded evidence key; non-strings show empty. */
export function evidenceText(recorded: Record<string, unknown> | undefined, key: string): string {
  const value = recorded?.[key];
  return typeof value === 'string' ? value : '';
}

/** Top-level edits for the fields the owner changed, or undefined when none. */
export function sourceEvidenceEdits(
  recorded: Record<string, unknown> | undefined,
  fields: Record<string, string>
): Record<string, string | null> | undefined {
  const edits: Record<string, string | null> = {};
  for (const [key, raw] of Object.entries(fields)) {
    const value = raw.trim();
    if (value === evidenceText(recorded, key).trim()) continue;
    edits[key] = value || null;
  }
  return Object.keys(edits).length > 0 ? edits : undefined;
}

/** Fresh evidence for a new record from the form's non-empty fields. */
export function newSourceEvidence(fields: Record<string, string>): Record<string, string> | undefined {
  const evidence = Object.fromEntries(
    Object.entries(fields).map(([key, raw]) => [key, raw.trim()]).filter(([, value]) => value !== ''));
  return Object.keys(evidence).length > 0 ? evidence : undefined;
}
