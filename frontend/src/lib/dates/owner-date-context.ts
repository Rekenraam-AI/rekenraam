import { getContext, setContext } from 'svelte';

const ownerDateContext = Symbol('owner-local-today');

export function provideOwnerToday(read: () => string): void {
  setContext(ownerDateContext, read);
}

/** Shared reactive server date. Capture it once for editable defaults; derived
 * discovery reads can follow it until the user supplies an explicit date. */
export function useOwnerToday(): () => string {
  return getContext<() => string>(ownerDateContext);
}
