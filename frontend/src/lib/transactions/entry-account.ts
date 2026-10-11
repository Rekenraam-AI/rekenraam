import { isSubledgerManagedAccountKind } from '#lib/accounts/subledger-accounts.ts';

type EntryAccount = {
  account_class: string;
  account_kind?: string | null;
  status: string;
  is_system?: boolean | null;
  allows_postings: boolean;
};

/**
 * Whether the generic transaction editor may post to an account: an active,
 * non-system asset or liability that allows postings and is not owned by the
 * investment subledger (T-96). The editor's account picker and every screen
 * that pre-selects an account for a new transaction (#185) use this one rule,
 * so a pre-selected account is always one the picker offers.
 */
export function isOrdinaryEntryAccount(account: EntryAccount): boolean {
  return (account.account_class === 'asset' || account.account_class === 'liability') &&
    account.status === 'active' && !account.is_system && account.allows_postings &&
    !isSubledgerManagedAccountKind(account.account_kind);
}

/**
 * How an account's own screen offers a new transaction: the generic editor
 * with the account pre-selected, investment entry for a holding account (its
 * activity goes through investment commands, #186), or nothing for an account
 * that takes no manual postings.
 */
export function accountEntryKind(account: EntryAccount): 'editor' | 'investments' | 'none' {
  if (isOrdinaryEntryAccount(account)) return 'editor';
  if (account.status === 'active' && isSubledgerManagedAccountKind(account.account_kind)) return 'investments';
  return 'none';
}
