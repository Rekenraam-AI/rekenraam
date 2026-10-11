/**
 * Same-day reordering (#185). Lists show the newest entries first, so the
 * entry above a row on the same day happened later and the one below
 * happened earlier. A move names the visible neighbour it passes, never an
 * invisible one, so a filtered list moves exactly what the user sees.
 */
export type ReorderDirection = 'earlier' | 'later';

/** The visible same-day neighbour a row passes when moved, if any. */
export function sameDayNeighbor<T>(rows: readonly T[], index: number, direction: ReorderDirection,
  dayOf: (row: T) => string): T | undefined {
  const row = rows[index];
  const neighbor = rows[direction === 'earlier' ? index + 1 : index - 1];
  return row !== undefined && neighbor !== undefined && dayOf(neighbor) === dayOf(row) ? neighbor : undefined;
}

/** Moving earlier places the row before its neighbour; later, after it. */
export function placePosition(direction: ReorderDirection): 'before' | 'after' {
  return direction === 'earlier' ? 'before' : 'after';
}
