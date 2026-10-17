import { describe, expect, it } from 'vitest';
import { grossPrefill, parseProviderDividendEvidence, proposalCommodityID, proposalShape } from './provider-dividend.ts';

describe('provider dividend evidence', () => {
  it('reads schema-1 evidence and keeps unknown figures null', () => {
    const evidence = parseProviderDividendEvidence({
      schema: 1,
      symbol: 'ASML',
      mic: 'XAMS',
      ex_date: '2026-07-29',
      entitlement_date: '2026-07-28',
      amount_per_share: null,
      listing_currency: 'EUR',
      entitled_units: { value: '10', scale: 0 },
      estimated_gross: { value: 'x', scale: 0 },
      missing: ['payment_date', 7, 'amount_per_share']
    });
    expect(evidence).toMatchObject({
      symbol: 'ASML',
      exDate: '2026-07-29',
      amountPerShare: null,
      entitledUnits: { value: '10', scale: 0 },
      estimatedGross: null,
      missing: ['payment_date', 'amount_per_share'],
      unsupported: null
    });
    expect(parseProviderDividendEvidence({ schema: 2 })).toBeNull();
    expect(parseProviderDividendEvidence({})).toBeNull();
  });

  it('classifies proposals by what the owner can do', () => {
    expect(proposalShape({ kind: 'dividend_income', complete: false })).toBe('needs_completion');
    expect(proposalShape({ kind: 'dividend_income', amount_value: 1 })).toBe('complete');
    expect(proposalShape({ kind: 'informational', reason: 'short_position' })).toBe('informational');
    expect(proposalCommodityID({ commodity_id: 4 })).toBe(4);
    expect(proposalCommodityID({ commodity_id: '4' })).toBeUndefined();
  });

  it('pre-fills the received gross only in the same currency without rounding', () => {
    expect(grossPrefill({ value: '16', scale: 0 }, 'EUR', 'EUR', 2)).toEqual({ value: '1600', scale: 2 });
    expect(grossPrefill({ value: '16', scale: 0 }, 'EUR', 'USD', 2)).toBeNull();
    expect(grossPrefill({ value: '16001', scale: 3 }, 'EUR', 'EUR', 2)).toBeNull();
    expect(grossPrefill(null, 'EUR', 'EUR', 2)).toBeNull();
  });
});
