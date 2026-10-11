import { describe, expect, it } from 'vitest';
import { placePosition, sameDayNeighbor } from './reorder';

// Newest first, as every list shows them.
const rows = [
  { id: 4, day: '2026-06-08' },
  { id: 3, day: '2026-06-07' },
  { id: 2, day: '2026-06-07' },
  { id: 1, day: '2026-06-06' }
];
const day = (row: { day: string }) => row.day;

describe('sameDayNeighbor (#185)', () => {
  it('moves earlier past the row below and later past the row above', () => {
    expect(sameDayNeighbor(rows, 1, 'earlier', day)?.id).toBe(2);
    expect(sameDayNeighbor(rows, 2, 'later', day)?.id).toBe(3);
  });

  it('never crosses into another day', () => {
    expect(sameDayNeighbor(rows, 2, 'earlier', day)).toBeUndefined();
    expect(sameDayNeighbor(rows, 1, 'later', day)).toBeUndefined();
    expect(sameDayNeighbor(rows, 0, 'later', day)).toBeUndefined();
    expect(sameDayNeighbor(rows, 3, 'earlier', day)).toBeUndefined();
  });

  it('places earlier before and later after the neighbour', () => {
    expect(placePosition('earlier')).toBe('before');
    expect(placePosition('later')).toBe('after');
  });
});
