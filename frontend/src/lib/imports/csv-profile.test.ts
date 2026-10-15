import { describe, expect, it } from 'vitest';
import { rankCSVProfiles, uniqueCSVProfileSuggestion, validCSVInvestmentMapping } from './csv-profile';

describe('CSV profile suggestions', () => {
  const profile = (id: number, config: object) => ({ id, config: JSON.stringify(config) });
  const base = {
    delimiter: 'semicolon', date_column: 'Datum', amount_column: 'Bedrag',
    date_layout: 'DMY', decimal_separator: ',', invert_amount: false
  };

  it('ranks an exact header and normalized filename match first', () => {
    const ranked = rankCSVProfiles([
      profile(1, base),
      profile(2, { ...base, headers: ['Datum', 'Bedrag'], source_filename: 'statement-2026-07.csv' })
    ], 'statement-2026-08.csv', 'semicolon', ['Datum', 'Bedrag']);
    expect(ranked.map((entry) => [entry.profile.id, entry.score])).toEqual([[2, 90], [1, 20]]);
    expect(uniqueCSVProfileSuggestion(ranked)?.id).toBe(2);
  });

  it('does not auto-select between equally compatible mappings', () => {
    const ranked = rankCSVProfiles([profile(1, base), profile(2, base)], 'statement.csv', 'semicolon', ['Datum', 'Bedrag']);
    expect(uniqueCSVProfileSuggestion(ranked)).toBeNull();
  });

  it('keeps profiles created before matching hints and invert_amount were stored compatible', () => {
    const legacy = { ...base } as Partial<typeof base>;
    delete legacy.invert_amount;
    const ranked = rankCSVProfiles([profile(1, legacy)], 'statement.csv', 'semicolon', ['Datum', 'Bedrag']);
    expect(uniqueCSVProfileSuggestion(ranked)?.id).toBe(1);
  });

  it('excludes a profile whose mapped columns are missing', () => {
    expect(rankCSVProfiles([profile(1, base)], 'statement.csv', 'semicolon', ['Date', 'Amount'])).toEqual([]);
  });
});


describe('investment CSV profile matching', () => {
  const investment = { kind: 'column' as const, kind_column: 'Type', dividend_value: 'DIV', withholding_value: 'TAX', instrument_column: 'Code', gross_column: 'Gross', withholding_column: 'Tax' };
  it('requires separate gross evidence and distinct source kinds', () => {
    expect(validCSVInvestmentMapping({ ...investment, gross_column: '' })).toBe(false);
    expect(validCSVInvestmentMapping({ ...investment, withholding_value: 'DIV' })).toBe(false);
    expect(validCSVInvestmentMapping({ kind: 'withholding', instrument_column: 'Code' })).toBe(true);
  });
  it('does not suggest a mapping when investment evidence columns are missing', () => {
    const profiles = [{ id: 1, config: JSON.stringify({ delimiter: 'semicolon', date_column: 'Date', amount_column: 'Cash', date_layout: 'YMD', decimal_separator: ',', investment }) }];
    expect(rankCSVProfiles(profiles, 'income.csv', 'semicolon', ['Date', 'Cash', 'Code', 'Type'])).toEqual([]);
    expect(rankCSVProfiles(profiles, 'income.csv', 'semicolon', ['Date', 'Cash', 'Code', 'Type', 'Gross', 'Tax'])).toHaveLength(1);
  });
});
