/**
 * Provider dividend evidence (#111): the normalized_json schema 1 the backend
 * stages for a per-share announcement. Decimals are exact {value, scale}
 * pairs; null means the source or the ledger did not establish the figure.
 */
export type ProviderDecimal = { value: string; scale: number };

export type ProviderDividendEvidence = {
  symbol: string;
  mic: string;
  exDate: string;
  entitlementDate: string;
  amountPerShare: ProviderDecimal | null;
  listingCurrency: string;
  entitledUnits: ProviderDecimal | null;
  estimatedGross: ProviderDecimal | null;
  missing: string[];
  unsupported: string | null;
};

function decimal(raw: unknown): ProviderDecimal | null {
  if (!raw || typeof raw !== 'object') return null;
  const { value, scale } = raw as { value?: unknown; scale?: unknown };
  if (typeof value !== 'string' || !/^-?\d+$/.test(value) || typeof scale !== 'number' || !Number.isInteger(scale) || scale < 0) {
    return null;
  }
  return { value, scale };
}

function text(raw: unknown): string {
  return typeof raw === 'string' ? raw : '';
}

/** Reads schema-1 evidence; anything else is not a provider dividend. */
export function parseProviderDividendEvidence(raw: Record<string, unknown> | null | undefined): ProviderDividendEvidence | null {
  if (!raw || raw.schema !== 1) return null;
  return {
    symbol: text(raw.symbol),
    mic: text(raw.mic),
    exDate: text(raw.ex_date),
    entitlementDate: text(raw.entitlement_date),
    amountPerShare: decimal(raw.amount_per_share),
    listingCurrency: text(raw.listing_currency),
    entitledUnits: decimal(raw.entitled_units),
    estimatedGross: decimal(raw.estimated_gross),
    missing: Array.isArray(raw.missing) ? raw.missing.filter((code): code is string => typeof code === 'string') : [],
    unsupported: typeof raw.unsupported === 'string' && raw.unsupported !== '' ? raw.unsupported : null
  };
}

export type ProposalShape = 'complete' | 'needs_completion' | 'informational';

/** How a suggestion can be acted on, from its proposed_transaction. */
export function proposalShape(proposal: Record<string, unknown> | null | undefined): ProposalShape {
  if (proposal?.kind === 'informational') return 'informational';
  if (proposal?.kind === 'dividend_income' && proposal.complete === false) return 'needs_completion';
  return 'complete';
}

export function proposalCommodityID(proposal: Record<string, unknown> | null | undefined): number | undefined {
  const id = proposal?.commodity_id;
  return typeof id === 'number' && Number.isInteger(id) && id > 0 ? id : undefined;
}

/**
 * The estimate may pre-fill the received gross only when it is stated in the
 * cash account's currency and fits that currency's scale exactly. Anything
 * else would invent a conversion or a rounding the owner never confirmed.
 */
export function grossPrefill(
  estimate: ProviderDecimal | null,
  listingCurrency: string,
  cashCurrency: string,
  cashScale: number
): ProviderDecimal | null {
  if (!estimate || !listingCurrency || listingCurrency !== cashCurrency || estimate.scale > cashScale) return null;
  const padding = cashScale - estimate.scale;
  return { value: estimate.value + '0'.repeat(padding), scale: cashScale };
}
