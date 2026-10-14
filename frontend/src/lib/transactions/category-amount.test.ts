import { describe, expect, it } from 'vitest';
import { categoryAmounts, type CategoryAmountPosting } from './category-amount';

const GROCERIES = 10;
const DINING = 11;
const SALARY = 20;
const BONUS = 21;
const EUR = 1;
const USD = 2;

const posting = (
	account_id: number,
	account_class: string,
	quantity_value: string,
	quantity_scale = 2,
	commodity_id = EUR,
	account_system_role: string | null = null
): CategoryAmountPosting => ({
	account_id,
	account_class,
	account_system_role,
	commodity_id,
	commodity_code: commodity_id === EUR ? 'EUR' : 'USD',
	commodity_symbol: commodity_id === EUR ? '€' : '$',
	quantity_value,
	quantity_scale
});

const tx = (...postings: CategoryAmountPosting[]) => ({ journal_entries: [{ postings }] });
const values = (rows: ReturnType<typeof categoryAmounts>) =>
	rows.map((r) => [r.commodityCode, r.value, r.scale]);

describe('categoryAmounts (#222)', () => {
	// Checking pays 30.00: 10.00 groceries, 20.00 dining.
	const split = tx(
		posting(100, 'asset', '-3000'),
		posting(GROCERIES, 'expense', '1000'),
		posting(DINING, 'expense', '2000')
	);

	it('shows each category only its own split leg', () => {
		expect(values(categoryAmounts(split, GROCERIES))).toEqual([['EUR', '1000', 2]]);
		expect(values(categoryAmounts(split, DINING))).toEqual([['EUR', '2000', 2]]);
	});

	it('shows nothing for a category the transaction does not post to', () => {
		expect(categoryAmounts(split, 999)).toEqual([]);
	});

	it('reads income positive and keeps other income legs out', () => {
		const pay = tx(
			posting(100, 'asset', '350000'),
			posting(SALARY, 'income', '-300000'),
			posting(BONUS, 'income', '-50000')
		);
		expect(values(categoryAmounts(pay, SALARY))).toEqual([['EUR', '300000', 2]]);
		expect(values(categoryAmounts(pay, BONUS))).toEqual([['EUR', '50000', 2]]);
	});

	it('keeps a refund in the same expense category negative', () => {
		const refund = tx(posting(100, 'asset', '1500'), posting(GROCERIES, 'expense', '-1500'));
		expect(values(categoryAmounts(refund, GROCERIES))).toEqual([['EUR', '-1500', 2]]);
	});

	it('sums repeated legs of one category across commodities and scales', () => {
		const mixed = {
			journal_entries: [
				{ postings: [posting(GROCERIES, 'expense', '500', 2), posting(DINING, 'expense', '9999', 2)] },
				{ postings: [posting(GROCERIES, 'expense', '1234', 4), posting(GROCERIES, 'expense', '700', 2, USD)] }
			]
		};
		// 5.00 + 0.1234 = 5.1234 EUR; 7.00 USD; dining's 99.99 stays out.
		expect(values(categoryAmounts(mixed, GROCERIES))).toEqual([
			['EUR', '51234', 4],
			['USD', '700', 2]
		]);
	});

	it('ignores system-role postings even on a matching account', () => {
		const sys = tx(posting(GROCERIES, 'expense', '1000', 2, EUR, 'fx_trading'));
		expect(categoryAmounts(sys, GROCERIES)).toEqual([]);
	});
});
