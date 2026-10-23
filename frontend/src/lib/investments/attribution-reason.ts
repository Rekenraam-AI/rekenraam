import { m } from '#lib/paraglide/messages.js';

export function attributionReason(reason: string): string {
  switch (reason) {
    case 'cash_in_lieu_parent_unresolved':
      return m.investments_attribution_cash_parent_unresolved();
    case 'cash_in_lieu_parent_mismatch':
      return m.investments_attribution_cash_parent_mismatch();
    case 'no_short_at_entitlement_close':
      return m.investments_attribution_no_short();
    case 'provider_attribution_mismatch':
      return m.investments_attribution_provider_mismatch();
    case 'ex_date_after_payment':
      return m.investments_attribution_ex_date_after_payment();
    default:
      return m.investments_attribution_review();
  }
}
