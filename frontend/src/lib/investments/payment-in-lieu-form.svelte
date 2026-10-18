<script lang="ts">
  import { untrack } from 'svelte';
  import { createQuery, useQueryClient } from '@tanstack/svelte-query';
  import { parseISO } from 'date-fns';
  import { checkpointOverride, isStaleImpactRefusal, type ReviewedCheckpoints } from '#lib/investments/gain-impact.ts';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { useOwnerToday } from '#lib/dates/owner-date-context.ts';
  import { accountsQueryOptions, type AccountResponse } from '#lib/api/accounts.ts';
  import { currenciesQueryOptions, type CurrencyResponse } from '#lib/api/currencies.ts';
  import { isSubledgerManagedAccountKind } from '#lib/accounts/subledger-accounts.ts';
  import {
    investmentInstrumentsQueryOptions,
    paymentInLieuReconciliationImpact,
    previewPaymentInLieuReplacementReconciliation,
    recordPaymentInLieu,
    replacePaymentInLieu,
    type InvestmentCorrectionPaymentInLieuTerms,
    type PaymentInLieuRequest,
    type ReconciliationImpactResponse
  } from '#lib/api/investments.ts';
  import { formatEditableAmount } from '#lib/money/amount.ts';
  import { parseDividendAmounts, type AmountFieldError } from '#lib/investments/form-amounts.ts';
  import ReconciliationConfirm from '#lib/investments/reconciliation-confirm.svelte';
  import type { InvestmentPrefill } from '#lib/investments/investment-prefill.ts';
  import { invalidateInvestmentReads } from './invalidate';

  // A short holder's payment in lieu of a dividend (#255): cash paid to an
  // expense account, with an optional recovery credited back. It never
  // changes the short lot. A correction keeps the short holding, instrument,
  // ex-date, payment date and cash account, and shows them read-only.
  let {
    csrfToken,
    onSaved,
    onCancel,
    prefill,
    correction
  }: {
    csrfToken: string;
    onSaved: () => void;
    onCancel: () => void;
    prefill?: InvestmentPrefill;
    correction?: { transactionID: number; terms: InvestmentCorrectionPaymentInLieuTerms };
  } = $props();
  const initialPrefill = untrack(() => prefill);
  const initial = untrack(() => correction);
  const terms = initial?.terms;
  const ownerToday = useOwnerToday();

  const queryClient = useQueryClient();
  const accountsQuery = createQuery(() => accountsQueryOptions(false, false));
  const currenciesQuery = createQuery(() => currenciesQueryOptions());
  const instrumentsQuery = createQuery(() => investmentInstrumentsQueryOptions());

  let holdingAccountID = $state(terms ? String(terms.holding_account_id)
    : initialPrefill?.holdingAccountID !== undefined ? String(initialPrefill.holdingAccountID) : '');
  let commodityID = $state(terms ? String(terms.commodity_id)
    : initialPrefill?.commodityID !== undefined ? String(initialPrefill.commodityID) : '');
  let exDate = $state(terms?.ex_date ?? ownerToday());
  let paymentDate = $state(terms?.event_date ?? ownerToday());
  let cashAccountID = $state(terms ? String(terms.cash_account_id) : '');
  let expenseAccountID = $state(terms ? String(terms.expense_account_id) : '');
  let amountStr = $state(terms ? formatEditableAmount(terms.amount_value, terms.amount_scale) : '');
  let showRecovery = $state(terms?.recovery_value !== undefined);
  let recoveryStr = $state(terms?.recovery_value !== undefined && terms.recovery_scale !== undefined
    ? formatEditableAmount(terms.recovery_value, terms.recovery_scale) : '');
  let recoveryAccountID = $state(terms?.recovery_account_id ? String(terms.recovery_account_id) : '');
  let memo = $state(terms?.memo ?? '');
  let reason = $state('');
  let pending = $state(false);
  let formError = $state<unknown>(undefined);
  let review = $state<{ impacts: ReconciliationImpactResponse['affected_checkpoints']; payload: PaymentInLieuRequest } | null>(null);
  let firstInput = $state<HTMLInputElement | HTMLSelectElement>();

  $effect(() => {
    firstInput?.focus();
  });

  const accounts = $derived(accountsQuery.data?.accounts ?? []);
  const instruments = $derived(instrumentsQuery.data?.instruments ?? []);
  const accountName = (id: number) => accounts.find((account: AccountResponse) => account.id === id)?.name ?? `#${id}`;
  const holdingAccounts = $derived(accounts.filter((account: AccountResponse) =>
    isSubledgerManagedAccountKind(account.account_kind) && account.status === 'active'));
  const cashAccounts = $derived(accounts.filter((account: AccountResponse) =>
    ['asset', 'liability'].includes(account.account_class) && account.allows_postings && !account.system_role &&
    !isSubledgerManagedAccountKind(account.account_kind) && (account.status === 'active' || account.id === terms?.cash_account_id)));
  const expenseAccounts = $derived(accounts.filter((account: AccountResponse) =>
    account.account_class === 'expense' && account.allows_postings &&
    (account.status === 'active' || account.id === terms?.expense_account_id)));
  // Matches the backend recovery role: the same expense, income, or a
  // receivable or liability.
  const recoveryAccounts = $derived(accounts.filter((account: AccountResponse) =>
    ['asset', 'liability', 'expense', 'income'].includes(account.account_class) && account.allows_postings && !account.system_role &&
    !isSubledgerManagedAccountKind(account.account_kind) && (account.status === 'active' || account.id === terms?.recovery_account_id)));
  const currenciesByID = $derived(new Map<number, CurrencyResponse>(
    (currenciesQuery.data?.currencies ?? []).map((currency: CurrencyResponse) => [currency.id, currency])));
  const cashCommodityID = $derived(terms?.cash_commodity_id ??
    accounts.find((account: AccountResponse) => String(account.id) === cashAccountID)?.default_commodity_id ?? undefined);
  const currencyCode = $derived(cashCommodityID ? currenciesByID.get(cashCommodityID)?.code ?? '' : '');
  const instrumentLabel = $derived.by(() => {
    const instrument = instruments.find((candidate) => String(candidate.commodity_id) === commodityID);
    return instrument ? `${instrument.symbol ?? instrument.commodity_code} · ${instrument.display_name}` : '';
  });
  const dateFormatter = $derived(new Intl.DateTimeFormat(getLocale(), { year: 'numeric', month: 'short', day: 'numeric' }));
  const loading = $derived(accountsQuery.isPending || currenciesQuery.isPending || instrumentsQuery.isPending);
  const loadError = $derived(accountsQuery.isError || currenciesQuery.isError || instrumentsQuery.isError);
  const canSubmit = $derived(!loading && !loadError && holdingAccountID !== '' && commodityID !== '' && !!exDate && !!paymentDate &&
    cashAccountID !== '' && !!cashCommodityID && expenseAccountID !== '' && amountStr.trim() !== '' &&
    (!showRecovery || recoveryStr.trim() === '' || recoveryAccountID !== '') && (!initial || !!reason.trim()));

  // A short holding's instrument follows from the holding when it names one.
  function selectHolding() {
    const holding = accounts.find((account: AccountResponse) => String(account.id) === holdingAccountID);
    if (holding?.default_commodity_id && instruments.some((instrument) => instrument.commodity_id === holding.default_commodity_id)) {
      commodityID = String(holding.default_commodity_id);
    }
  }

  function amountErrorMessage(error: AmountFieldError): string {
    switch (error) {
      case 'negative':
        return m.investments_form_negative_number();
      case 'too_large':
        return m.investments_form_amount_too_large();
      case 'invalid':
        return m.investments_form_invalid_number();
    }
  }

  async function handleSubmit(event: Event) {
    event.preventDefault();
    if (!canSubmit || pending || !cashCommodityID) return;
    const amounts = parseDividendAmounts({ amountStr, withholdingStr: recoveryStr, includeWithholding: showRecovery,
      quantityStr: '', includeQuantity: false });
    if (!amounts.ok) {
      formError = new Error(amountErrorMessage(amounts.reason));
      return;
    }
    const { amount, withholding: recovery } = amounts.values;
    const payload: PaymentInLieuRequest = {
      holding_account_id: Number(holdingAccountID), commodity_id: Number(commodityID), ex_date: exDate,
      transaction_date: paymentDate, cash_account_id: Number(cashAccountID), cash_commodity_id: cashCommodityID,
      expense_account_id: Number(expenseAccountID), amount_value: amount.value, amount_scale: amount.scale,
      recovery_value: recovery?.value, recovery_scale: recovery?.scale,
      recovery_account_id: recovery && recoveryAccountID ? Number(recoveryAccountID) : undefined,
      memo: memo.trim() || undefined, payee_id: terms?.payee_id
    };
    pending = true;
    formError = undefined;
    try {
      if (await reviewImpact(payload)) return;
      await commit(payload, []);
    } catch (error) {
      formError = error;
    } finally {
      pending = false;
    }
  }

  // Show the checkpoints the write would invalidate, if any, for an override.
  async function reviewImpact(payload: PaymentInLieuRequest): Promise<boolean> {
    const impact = initial
      ? await previewPaymentInLieuReplacementReconciliation(initial.transactionID, { reason: reason.trim(), replacement: payload })
      : await paymentInLieuReconciliationImpact(payload);
    if (impact.affected_checkpoints.length === 0) return false;
    review = { impacts: impact.affected_checkpoints, payload };
    return true;
  }

  async function commit(payload: PaymentInLieuRequest, reviewed: ReviewedCheckpoints) {
    if (initial) {
      await replacePaymentInLieu(initial.transactionID,
        { reason: reason.trim(), replacement: payload, ...checkpointOverride(reviewed) }, csrfToken);
    } else {
      await recordPaymentInLieu({ ...payload, ...checkpointOverride(reviewed) }, csrfToken);
    }
    await invalidateInvestmentReads(queryClient);
    onSaved();
  }

  async function confirmOverride() {
    if (!review) return;
    const { payload, impacts } = review;
    review = null;
    pending = true;
    formError = undefined;
    try {
      await commit(payload, impacts);
    } catch (error) {
      // A statement finished since the review: show the current set (#304).
      try {
        if (!isStaleImpactRefusal(error) || !(await reviewImpact(payload))) formError = error;
      } catch (previewError) {
        formError = previewError;
      }
    } finally {
      pending = false;
    }
  }

  const inputClass = 'min-h-11 w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground';
</script>

{#if review}
  <ReconciliationConfirm impacts={review.impacts} gainRows={[]} gainRefreshed={false} {pending}
    onCancel={() => (review = null)} onConfirm={confirmOverride} />
{/if}

<form onsubmit={handleSubmit} class="space-y-4" aria-describedby="pil-copy">
  <h2 id="payment-in-lieu-title" class="text-base font-semibold text-foreground">
    {initial ? m.investments_payment_in_lieu_correct_title() : m.investments_payment_in_lieu_title()}
  </h2>
  <p id="pil-copy" class="text-sm text-muted">{m.investments_payment_in_lieu_copy()}</p>

  {#if loading}
    <p class="text-sm text-muted" role="status">{m.investments_payment_in_lieu_loading()}</p>
  {:else if loadError}
    <p class="text-sm text-danger" role="alert">{m.investments_payment_in_lieu_load_error()}</p>
  {/if}

  {#if initial && terms}
    <dl class="grid grid-cols-1 gap-2 rounded-(--radius-panel) border border-border p-3 text-sm sm:grid-cols-2">
      <div><dt class="text-xs text-muted">{m.investments_payment_in_lieu_holding()}</dt><dd class="text-foreground">{accountName(terms.holding_account_id)}</dd></div>
      <div><dt class="text-xs text-muted">{m.investments_form_instrument()}</dt><dd class="text-foreground">{instrumentLabel}</dd></div>
      <div><dt class="text-xs text-muted">{m.investments_payment_in_lieu_ex_date()}</dt><dd class="text-foreground">{dateFormatter.format(parseISO(terms.ex_date))}</dd></div>
      <div><dt class="text-xs text-muted">{m.investments_payment_in_lieu_payment_date()}</dt><dd class="text-foreground">{dateFormatter.format(parseISO(terms.event_date))}</dd></div>
      <div class="sm:col-span-2"><dt class="text-xs text-muted">{m.investments_form_cash_account()}</dt><dd class="text-foreground">{accountName(terms.cash_account_id)} {currencyCode}</dd></div>
    </dl>
    <div>
      <label for="pil-reason" class="mb-1 block text-sm font-medium text-foreground">{m.transactions_investment_reverse_reason()}</label>
      <input id="pil-reason" type="text" bind:this={firstInput} bind:value={reason} maxlength="500" required class={inputClass} />
    </div>
  {:else}
    <div>
      <label for="pil-holding" class="mb-1 block text-sm font-medium text-foreground">{m.investments_payment_in_lieu_holding()}</label>
      <select id="pil-holding" bind:this={firstInput} bind:value={holdingAccountID} onchange={selectHolding} required class={inputClass}>
        <option value="">{m.investments_form_select_account()}</option>
        {#each holdingAccounts as account (account.id)}<option value={String(account.id)}>{account.name}</option>{/each}
      </select>
    </div>
    <div>
      <label for="pil-instrument" class="mb-1 block text-sm font-medium text-foreground">{m.investments_form_instrument()}</label>
      <select id="pil-instrument" bind:value={commodityID} required class={inputClass}>
        <option value="">{m.investments_form_instrument_placeholder()}</option>
        {#each instruments as instrument (instrument.commodity_id)}
          <option value={String(instrument.commodity_id)}>{instrument.symbol ?? instrument.commodity_code} · {instrument.display_name}</option>
        {/each}
      </select>
    </div>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label for="pil-ex-date" class="mb-1 block text-sm font-medium text-foreground">{m.investments_payment_in_lieu_ex_date()}</label>
        <input id="pil-ex-date" type="date" bind:value={exDate} required class={inputClass} aria-describedby="pil-ex-date-hint" />
        <p id="pil-ex-date-hint" class="mt-1 text-xs text-muted">{m.investments_payment_in_lieu_ex_date_hint()}</p>
      </div>
      <div>
        <label for="pil-payment-date" class="mb-1 block text-sm font-medium text-foreground">{m.investments_payment_in_lieu_payment_date()}</label>
        <input id="pil-payment-date" type="date" bind:value={paymentDate} min={exDate} required class={inputClass} />
      </div>
    </div>
    <div>
      <label for="pil-cash" class="mb-1 block text-sm font-medium text-foreground">{m.investments_form_cash_account()}</label>
      <select id="pil-cash" bind:value={cashAccountID} required class={inputClass}>
        <option value="">{m.investments_form_select_account()}</option>
        {#each cashAccounts as account (account.id)}<option value={String(account.id)}>{account.name}</option>{/each}
      </select>
    </div>
  {/if}

  <div>
    <label for="pil-amount" class="mb-1 block text-sm font-medium text-foreground">
      {m.investments_payment_in_lieu_amount()}
      {#if currencyCode}<span class="ml-1 text-xs font-normal text-muted">({currencyCode})</span>{/if}
    </label>
    <input id="pil-amount" type="text" inputmode="decimal" bind:value={amountStr} placeholder={formatEditableAmount('0', 2)} required
      class="{inputClass} font-mono placeholder:text-muted" />
  </div>
  <div>
    <label for="pil-expense" class="mb-1 block text-sm font-medium text-foreground">{m.investments_payment_in_lieu_expense_account()}</label>
    <select id="pil-expense" bind:value={expenseAccountID} required class={inputClass}>
      <option value="">{m.investments_form_select_account()}</option>
      {#each expenseAccounts as account (account.id)}<option value={String(account.id)}>{account.name}</option>{/each}
    </select>
  </div>

  <label class="flex min-h-11 items-center gap-2 text-sm font-medium text-foreground">
    <input type="checkbox" bind:checked={showRecovery} class="rounded" />
    {m.investments_payment_in_lieu_recovery_toggle()}
  </label>
  {#if showRecovery}
    <div class="space-y-3 rounded-(--radius-panel) border border-border p-3">
      <div>
        <label for="pil-recovery" class="mb-1 block text-sm font-medium text-foreground">
          {m.investments_payment_in_lieu_recovery_amount()}
          {#if currencyCode}<span class="ml-1 text-xs font-normal text-muted">({currencyCode})</span>{/if}
        </label>
        <input id="pil-recovery" type="text" inputmode="decimal" bind:value={recoveryStr} placeholder={formatEditableAmount('0', 2)}
          class="{inputClass} font-mono placeholder:text-muted" />
      </div>
      <div>
        <label for="pil-recovery-account" class="mb-1 block text-sm font-medium text-foreground">{m.investments_payment_in_lieu_recovery_account()}</label>
        <select id="pil-recovery-account" bind:value={recoveryAccountID} class={inputClass}>
          <option value="">{m.investments_form_select_account()}</option>
          {#each recoveryAccounts as account (account.id)}<option value={String(account.id)}>{account.name}</option>{/each}
        </select>
      </div>
    </div>
  {/if}

  <div>
    <label for="pil-memo" class="mb-1 block text-sm font-medium text-foreground">{m.investments_form_memo()}</label>
    <input id="pil-memo" type="text" bind:value={memo} class={inputClass} />
  </div>

  <APIFormError error={formError} id="pil-form-error" />

  <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
    <button type="button" onclick={onCancel}
      class="min-h-11 rounded-(--radius-control) border border-border bg-control px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-control-hover">
      {m.investments_form_cancel()}
    </button>
    <button type="submit" disabled={!canSubmit || pending}
      class="min-h-11 rounded-(--radius-control) bg-foreground px-4 py-2 text-sm font-semibold text-background transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50">
      {pending ? m.investments_payment_in_lieu_pending() : initial ? m.investments_payment_in_lieu_correct_submit() : m.investments_payment_in_lieu_submit()}
    </button>
  </div>
</form>
