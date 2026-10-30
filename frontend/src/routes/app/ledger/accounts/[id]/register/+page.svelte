<script lang="ts">
  import { page } from '$app/state';
  import { createQuery } from '@tanstack/svelte-query';
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import { m } from '#lib/paraglide/messages.js';
  import { accountsQueryOptions } from '#lib/api/accounts.ts';
  import AccountRegister from '#lib/transactions/account-register.svelte';
  import TransactionWorkspace, { type TransactionWorkspaceCreate } from '#lib/transactions/transaction-workspace.svelte';
  import { accountEntryKind } from '#lib/transactions/entry-account.ts';
  import { investmentInstrumentsQueryOptions } from '#lib/api/investments.ts';
  import { holdingAccountPrefill } from '#lib/investments/investment-prefill.ts';

  const accountID = $derived(Number(page.params.id));

  const accountsQuery = createQuery(() => accountsQueryOptions(false, false));
  const account = $derived(accountsQuery.data?.accounts.find((a) => a.id === accountID));
  const accountName = $derived(account?.name ?? account?.code ?? String(accountID));
  const instrumentsQuery = createQuery(() => ({
    ...investmentInstrumentsQueryOptions(),
    enabled: !!account && accountEntryKind(account) === 'investments'
  }));

  // A new transaction starts in this account (#185). A holding account's
  // activity is entered as investment commands instead, with the holding and
  // its instrument pre-selected (#186).
  const create = $derived.by<TransactionWorkspaceCreate>(() => {
    if (!account) return { kind: 'none' };
    switch (accountEntryKind(account)) {
      case 'editor':
        return { kind: 'editor', defaults: { accountID: account.id } };
      case 'investments':
        return { kind: 'investment', prefill: holdingAccountPrefill(account, instrumentsQuery.data?.instruments ?? []) };
      default:
        return { kind: 'none' };
    }
  });
</script>

<TransactionWorkspace {create}>
  {#snippet header()}
    <a
      href="/app/ledger/accounts"
      class="inline-flex items-center gap-1.5 rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-semibold text-foreground transition hover:bg-control-hover"
    >
      <ArrowLeft size={14} aria-hidden="true" />
      {m.register_back_to_accounts()}
    </a>
    {#if accountName}
      <h2 class="text-sm font-semibold text-foreground">{accountName}</h2>
    {/if}
  {/snippet}
  {#snippet list(actions, csrfToken)}
    <AccountRegister {accountID} {csrfToken} {actions} onRowClick={(entry) => actions.showTransaction(entry.transaction_id)} />
  {/snippet}
</TransactionWorkspace>
