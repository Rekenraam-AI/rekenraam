import { sumByCommodity, type PostingAmount } from '#lib/money/amount.ts';

type ActivityPosting = PostingAmount & {
  account_id: number;
  account_system_role?: string | null;
};

export type ActivityTransaction = {
  journal_entries?: { postings?: ActivityPosting[] | null }[] | null;
};

function holdingPostings(tx: ActivityTransaction, commodityID: number): ActivityPosting[] {
  return (tx.journal_entries ?? [])
    .flatMap((entry) => entry.postings ?? [])
    .filter((p) => p.commodity_id === commodityID && p.account_class === 'asset' && !p.account_system_role);
}

/**
 * How many units of the instrument a transaction moved into (positive) or
 * out of (negative) the owner's holdings (#186). Clearing and other system
 * legs are left out, so a buy shows the units bought and a transfer between
 * two holdings nets to zero. Undefined when no holding posting carries it.
 */
export function instrumentUnits(tx: ActivityTransaction, commodityID: number): { value: string; scale: number } | undefined {
  const postings = holdingPostings(tx, commodityID);
  if (postings.length === 0) return undefined;
  const [total] = sumByCommodity(postings);
  return { value: total.value, scale: total.scale };
}

/** The holding accounts a transaction moved the instrument through. */
export function instrumentAccountIDs(tx: ActivityTransaction, commodityID: number): number[] {
  return [...new Set(holdingPostings(tx, commodityID).map((p) => p.account_id))];
}
