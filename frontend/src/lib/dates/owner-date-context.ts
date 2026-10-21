import { createContext } from 'svelte';

// The getter throws when no ancestor provided the date, instead of handing
// back undefined to fail later at the first call.
const [getOwnerToday, setOwnerToday] = createContext<() => string>();

export function provideOwnerToday(read: () => string): void {
  setOwnerToday(read);
}

/** Shared reactive server date. Capture it once for editable defaults; derived
 * discovery reads can follow it until the user supplies an explicit date. */
export function useOwnerToday(): () => string {
  return getOwnerToday();
}
