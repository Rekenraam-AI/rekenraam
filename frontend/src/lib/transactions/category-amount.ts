import { sumByCommodity, type CommodityTotal } from '#lib/money/amount.ts';

/** The posting fields a category amount is computed from. */
export interface CategoryAmountPosting {
	account_id: number;
	account_class: string;
	account_system_role?: string | null;
	commodity_id: number;
	commodity_code: string;
	commodity_symbol?: string | null;
	quantity_value: string;
	quantity_scale: number;
}

export interface CategoryAmountTransaction {
	journal_entries?: { postings?: CategoryAmountPosting[] | null }[] | null;
}

export interface CategoryCommodityAmount extends CommodityTotal {
	commodityCode: string;
	commoditySymbol: string | null | undefined;
}

/**
 * The amount a transaction contributes to one category, per commodity (#222).
 *
 * The scope is exactly the category's own account, the same rule the list's
 * `category_id` filter matches with (`posting.account_id = ?`, no
 * descendants). A split that also posts to other categories therefore shows
 * only this category's legs here, never the transaction's whole category side.
 * Summing and the income sign flip stay in `sumByCommodity`.
 */
export function categoryAmounts(
	tx: CategoryAmountTransaction,
	categoryID: number
): CategoryCommodityAmount[] {
	const postings = (tx.journal_entries ?? [])
		.flatMap((entry) => entry.postings ?? [])
		.filter(
			(p) =>
				p.account_id === categoryID &&
				(p.account_class === 'income' || p.account_class === 'expense') &&
				!p.account_system_role
		);
	if (postings.length === 0) return [];

	const representative = new Map<number, CategoryAmountPosting>();
	for (const p of postings) {
		if (!representative.has(p.commodity_id)) representative.set(p.commodity_id, p);
	}

	return sumByCommodity(postings).map((total) => {
		const rep = representative.get(total.commodityID)!;
		return { ...total, commodityCode: rep.commodity_code, commoditySymbol: rep.commodity_symbol };
	});
}
