import { describe, expect, it } from 'vitest';
import { parsePriceAmount } from './form.ts';

describe('exact bounded price entry', () => {
  it('preserves coefficients beyond browser safe integers', () => {
    expect(parsePriceAmount('0.009007199254740993')).toEqual({ value: '9007199254740993', scale: 18 });
    expect(parsePriceAmount('9223372036854775807')).toEqual({ value: '9223372036854775807', scale: 0 });
  });
  it('refuses zero, negative, overflow, exponent and ambiguous separators', () => {
    for (const value of ['0', '-1', '9223372036854775808', '1e3', '1,50', '0.0000000000000000001']) expect(parsePriceAmount(value)).toBeNull();
  });
  it('preserves price and quoted base quantity scales', () => {
    expect(parsePriceAmount('125.50')).toEqual({ value: '12550', scale: 2 });
    expect(parsePriceAmount('0.005')).toEqual({ value: '5', scale: 3 });
  });
});
