import type { ImportStagedRow } from '#lib/api/imports.ts';

export function sourceCorrectionKind(row: ImportStagedRow): 'buy' | 'sale' | null {
  if (unsupportedSourceFill(row) || !row.source_changed || !row.source_transaction_id ||
    (row.commit_status !== 'pending' && row.commit_status !== 'skipped')) return null;
  try {
    const raw = JSON.parse(row.raw) as { kind?: string; side?: string };
    if (raw?.kind !== 'trading212_order_fill' || typeof raw.side !== 'string') return null;
    const side = raw.side.trim().toUpperCase();
    if (side === 'BUY' && row.source_buy_operation) return 'buy';
    if (side === 'SELL' && row.source_sale_operation) return 'sale';
    return null;
  } catch {
    return null;
  }
}

/** Only documented ordinary executions may enter the trade import path. */
export function unsupportedSourceFill(row: Pick<ImportStagedRow, 'raw'>): boolean {
  try {
    const raw = JSON.parse(row.raw) as { kind?: string; fill_type?: string } | null;
    return raw?.kind === 'trading212_order_fill' && raw.fill_type !== 'TRADE';
  } catch { return false; }
}

/**
 * A pending Trading 212 STOCK_SPLIT fill. Its fill type supplies no ratio or
 * entitlement, so it never posts; the user may link it to a split they
 * recorded, which keeps a re-fetch from posting it again (T-122).
 */
export function linkableSplitFill(row: Pick<ImportStagedRow, 'raw' | 'commit_status' | 'dedupe_status'>): boolean {
  if (row.commit_status === 'committed' || row.dedupe_status === 'duplicate') return false;
  try {
    const raw = JSON.parse(row.raw) as { kind?: string; fill_type?: string } | null;
    return raw?.kind === 'trading212_order_fill' && raw.fill_type === 'STOCK_SPLIT';
  } catch { return false; }
}

/**
 * A pending unsupported BUY or SELL fill other than a split (#245). It never
 * posts and is never classified as a short by itself; the owner may link it
 * to a short sale (SELL) or cover (BUY) they recorded natively.
 */
export function linkableShortFill(row: Pick<ImportStagedRow, 'raw' | 'commit_status' | 'dedupe_status'>): boolean {
  if (row.commit_status === 'committed' || row.dedupe_status === 'duplicate') return false;
  try {
    const raw = JSON.parse(row.raw) as { kind?: string; fill_type?: string; side?: unknown } | null;
    if (raw?.kind !== 'trading212_order_fill' || raw.fill_type === 'TRADE' || raw.fill_type === 'STOCK_SPLIT') return false;
    const side = typeof raw.side === 'string' ? raw.side.trim().toUpperCase() : '';
    return side === 'BUY' || side === 'SELL';
  } catch { return false; }
}

/** Whether a committed unsupported fill was linked to a split or a short. */
export function linkedFillKind(row: Pick<ImportStagedRow, 'raw'>): 'split' | 'short' {
  try {
    const raw = JSON.parse(row.raw) as { fill_type?: string } | null;
    return raw?.fill_type === 'STOCK_SPLIT' ? 'split' : 'short';
  } catch { return 'short'; }
}
