import type { components } from '#lib/api/schema.js';

export type BasisSourceDeclaration = components['schemas']['InvestmentBasisResolutionSourceDeclaration'];

export const basisSourceTextMaxLength = 500;

export function basisSourceTextTooLong(text: string): boolean {
  return [...text.trim()].length > basisSourceTextMaxLength;
}

export function basisSourceDeclaration(
  kind: string, reference: string, rationale: string, knownOn: string
): BasisSourceDeclaration | null {
  if ((kind === 'statement' || kind === 'provider') && reference.trim() && !basisSourceTextTooLong(reference)) {
    return { kind, reference: reference.trim() };
  }
  if (kind === 'owner_assertion' && rationale.trim() && !basisSourceTextTooLong(rationale) && /^\d{4}-\d{2}-\d{2}$/.test(knownOn)) {
    return { kind, rationale: rationale.trim(), known_on: knownOn };
  }
  return null;
}

// Older evidence remains untyped. Display it as recorded, and require the
// owner to choose a declaration kind explicitly when replacing it.
export function basisSourceFields(evidence: Record<string, unknown> | undefined) {
  const text = (key: string) => typeof evidence?.[key] === 'string' ? evidence[key] as string : '';
  const kind = text('kind');
  return {
    kind: ['statement', 'provider', 'owner_assertion'].includes(kind) ? kind : '',
    reference: text('reference') || text('statement'),
    rationale: text('rationale'),
    knownOn: text('known_on')
  };
}
