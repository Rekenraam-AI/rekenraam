/** Browser-local presentation preference. Never used by editing or export code. */
export const amountsStorageKey = 'rekenraam-hide-amounts';
function storedHidden(): boolean {
  try { return typeof localStorage !== 'undefined' && localStorage.getItem(amountsStorageKey) === 'true'; }
  catch { return false; }
}
// Read before the first app render, so remembered masking has no visible flash.
export const amountPrivacy = $state({ hidden: storedHidden() });

export function setAmountsHidden(hidden: boolean): void {
  amountPrivacy.hidden = hidden;
  try { localStorage.setItem(amountsStorageKey, String(hidden)); } catch { /* Session-only if storage is unavailable. */ }
}

export function syncAmountPrivacy(event: StorageEvent): void {
  if (event.key === amountsStorageKey || event.key === null) amountPrivacy.hidden = storedHidden();
}
