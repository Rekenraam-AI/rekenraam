<script lang="ts">
  import { useOwnerToday } from '#lib/dates/owner-date-context.ts';
  import { createQuery, useQueryClient } from '@tanstack/svelte-query';
  import { untrack } from 'svelte';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import { accountsQueryOptions } from '#lib/api/accounts.ts';
  import { currenciesQueryOptions, type CurrencyResponse } from '#lib/api/currencies.ts';
  import {
    investmentInstrumentsQueryOptions, datedHoldingsQueryOptions, previewCashStockMerger, recordCashStockMerger,
    previewCashStockMergerReplacement, replaceCashStockMerger,
    type CashStockMergerPlan, type CashStockMergerReplacementRequest, type CashStockMergerRequest, type GainImpact,
    type InvestmentCorrectionCashStockMergerTerms, type ReconciliationImpactResponse
  } from '#lib/api/investments.ts';
  import { invalidateInvestmentReads } from './invalidate';
  import {
    gainAcknowledgement, gainImpactCurrency, gainImpactRows, hasGainChanges, impactNeedsReview,
    isGainAcknowledgementRefusal
  } from './gain-impact';
  import ReconciliationConfirm from './reconciliation-confirm.svelte';
  import CashAndStockMergerSummary from './cash-and-stock-merger-summary.svelte';
  import { parseMoneyMagnitude } from './form-amounts';
  import { basisFractionPercent, parseElectionPercent, parseExchangeRatio } from './split-ratio';
  import { TranslatedFormError } from '#lib/form-errors.ts';
  import { coefficientSign, formatEditableAmount } from '#lib/money/amount.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';

  const ownerToday = useOwnerToday();

  // Cash-and-stock merger entry (#184): the whole holding becomes a new
  // instrument at a fixed ratio and the holder also receives cash. The cash
  // recovers an elected share of each lot's cost basis, which the user enters
  // from their jurisdiction's rule; the rest carries into the new lots. The
  // server plans every lot; the form previews that plan and records exactly
  // what was previewed. A merger dated behind later activity revises it,
  // which the shared gain review discloses. With correction set, the form
  // replaces that posted merger: the merged holding is fixed, the current
  // terms are pre-filled and a reason is required.
  let { csrfToken, onSaved, onCancel, correction }: {
    csrfToken: string;
    onSaved: () => void;
    onCancel: () => void;
    correction?: { transactionID: number; terms: InvestmentCorrectionCashStockMergerTerms };
  } = $props();

  const queryClient = useQueryClient();
  const accountsQuery = createQuery(() => accountsQueryOptions(false, false));
  const instrumentsQuery = createQuery(() => investmentInstrumentsQueryOptions());
  const currenciesQuery = createQuery(() => currenciesQueryOptions());

  // Pre-fill once from the merger being corrected; later edits are the user's.
  const initialTerms = untrack(() => correction?.terms);
  const initialPercent = initialTerms
    ? basisFractionPercent(initialTerms.plan.cash_basis_fraction_value, initialTerms.plan.cash_basis_fraction_scale) : null;
  let effectiveOn = $state(initialTerms?.effective_on ?? '');
  let payableOn = $state(initialTerms?.payable_on ?? '');
  let holdingKey = $state(initialTerms ? `${initialTerms.holding_account_id}:${initialTerms.commodity_id}` : '');
  let newCommodityID = $state(initialTerms ? String(initialTerms.destination_commodity_id) : '');
  // Null follows the suggested destination; a choice the user makes sticks
  // until the holding or the new instrument changes.
  let chosenDestination = $state<string | null>(!initialTerms ? null
    : initialTerms.destination_holding_account_id === initialTerms.holding_account_id
      ? 'same' : String(initialTerms.destination_holding_account_id));
  let newUnits = $state(initialTerms ? String(initialTerms.plan.ratio_numerator) : '');
  let oldUnits = $state(initialTerms ? String(initialTerms.plan.ratio_denominator) : '');
  let cashAccountID = $state(initialTerms ? String(initialTerms.cash_account_id) : '');
  let chosenCurrencyID = $state<string | null>(initialTerms ? String(initialTerms.currency_id) : null);
  let cashAmount = $state(initialTerms ? formatEditableAmount(initialTerms.cash_value, initialTerms.cash_scale) : '');
  let electionPercent = $state(initialPercent ? formatEditableAmount(initialPercent.value, initialPercent.scale) : '');
  // The recorded evidence pre-fills the reference it was entered with;
  // sending it back unchanged is not a correction.
  const initialEvidence = (initialTerms?.source_evidence ?? {}) as Record<string, unknown>;
  let sourceReference = $state(typeof initialEvidence.reference === 'string' ? initialEvidence.reference : '');
  let reason = $state('');
  let memo = $state('');
  let pending = $state(false);
  let formError = $state<unknown>(undefined);
  let dateInput = $state<HTMLInputElement | undefined>();
  let didFocus = false;
  // The reviewed preview. Editing any field discards it, so the user always
  // records exactly the plan they saw.
  let preview = $state<{
    plan: CashStockMergerPlan;
    impacts: ReconciliationImpactResponse['affected_checkpoints'];
    gainImpact: GainImpact | null;
    payload: CashStockMergerRequest;
  } | null>(null);
  let reviewModal = $state<{ gainRefreshed: boolean } | null>(null);
  const locale = getLocale();
  // Holdings as they stood on the effective date (#166); before a date is
  // chosen, today's.
  const holdingsDate = $derived(effectiveOn || ownerToday());
  const positionsQuery = createQuery(() => datedHoldingsQueryOptions(holdingsDate));

  const accounts = $derived((accountsQuery.data?.accounts ?? []).filter((account) =>
    account.status === 'active' && account.allows_postings &&
    (account.account_kind === 'security_holding' || account.account_kind === 'fund_holding')));
  const cashAccounts = $derived((accountsQuery.data?.accounts ?? []).filter((account) =>
    account.account_class === 'asset' && account.allows_postings &&
    (account.status === 'active' || account.id === initialTerms?.cash_account_id) &&
    account.account_kind !== 'security_holding' && account.account_kind !== 'fund_holding'));
  const instruments = $derived((instrumentsQuery.data?.instruments ?? []).filter((instrument) => instrument.status === 'active'));
  // Every long lot of the old instrument in one account is merged; the cash
  // is in the lots' cost currency, which the holding suggests.
  const holdings = $derived.by(() => {
    const seen = new Map<string, { accountID: number; commodityID: number; costCurrencyIDs: number[] }>();
    for (const position of positionsQuery.data ?? []) {
      if (position.position_side !== 'long' || coefficientSign(position.quantity_value) <= 0) continue;
      if (!accounts.some((account) => account.id === position.account_id)) continue;
      const key = `${position.account_id}:${position.commodity_id}`;
      const holding = seen.get(key) ?? { accountID: position.account_id, commodityID: position.commodity_id, costCurrencyIDs: [] };
      if (!holding.costCurrencyIDs.includes(position.cost_commodity_id)) holding.costCurrencyIDs.push(position.cost_commodity_id);
      seen.set(key, holding);
    }
    return [...seen.entries()].map(([key, value]) => ({ key, ...value }));
  });
  // A corrected merger's holding stays fixed.
  const selectedHolding = $derived(correction
    ? { key: holdingKey, accountID: correction.terms.holding_account_id, commodityID: correction.terms.commodity_id,
      costCurrencyIDs: [correction.terms.currency_id] }
    : holdings.find((holding) => holding.key === holdingKey));
  const newInstruments = $derived(instruments.filter((instrument) => instrument.commodity_id !== selectedHolding?.commodityID));
  const newID = $derived(Number(newCommodityID) || 0);
  // A holding account tied to one instrument cannot hold another, so the new
  // units go to an account that is untied or tied to the new instrument.
  const canHoldNew = (account: { default_commodity_id?: number | null }) =>
    account.default_commodity_id == null || account.default_commodity_id === newID;
  const sourceAccount = $derived(accounts.find((account) => account.id === selectedHolding?.accountID));
  const sameHoldingAllowed = $derived(!!sourceAccount && newID > 0 && canHoldNew(sourceAccount));
  const otherDestinations = $derived(newID > 0
    ? accounts.filter((account) => account.id !== selectedHolding?.accountID && canHoldNew(account)) : []);
  const suggestedDestination = $derived(sameHoldingAllowed ? 'same'
    : String(otherDestinations.find((account) => account.default_commodity_id === newID)?.id ?? ''));
  const destination = $derived(chosenDestination ?? suggestedDestination);
  const currencies = $derived(currenciesQuery.data?.currencies ?? []);
  const currenciesByID = $derived(new Map<number, CurrencyResponse>(currencies.map((currency) => [currency.id, currency])));
  const currencyID = $derived(chosenCurrencyID ?? String(selectedHolding?.costCurrencyIDs[0] ?? ''));
  const currency = $derived(currenciesByID.get(Number(currencyID)));
  const instrumentLabel = (commodityID: number) => {
    const instrument = (instrumentsQuery.data?.instruments ?? []).find((item) => item.commodity_id === commodityID);
    return instrument?.display_name ?? instrument?.commodity_code ?? `#${commodityID}`;
  };
  const loading = $derived(accountsQuery.isPending || positionsQuery.isPending ||
    instrumentsQuery.isPending || currenciesQuery.isPending);
  const loadError = $derived(accountsQuery.isError || positionsQuery.isError ||
    instrumentsQuery.isError || currenciesQuery.isError);
  const canPreview = $derived(!loading && !loadError && !!effectiveOn && !!payableOn && !!selectedHolding && newID > 0 &&
    !!destination && !!newUnits.trim() && !!oldUnits.trim() && !!cashAccountID && !!currency && !!cashAmount.trim() &&
    !!electionPercent.trim() && (!correction || !!reason.trim()));
  const modalGainRows = $derived(preview?.gainImpact
    ? gainImpactRows(preview.gainImpact.changes, gainImpactCurrency(currenciesByID), locale)
    : []);

  $effect(() => {
    if (!loading && !didFocus && dateInput) {
      dateInput.focus();
      didFocus = true;
    }
  });

  function discardPreview() {
    preview = null;
    formError = undefined;
  }

  function resetDestination() {
    chosenDestination = null;
    discardPreview();
  }

  function selectHolding() {
    chosenDestination = null;
    chosenCurrencyID = null;
    discardPreview();
  }

  async function handlePreview(event: SubmitEvent) {
    event.preventDefault();
    if (!canPreview || !selectedHolding || !currency) return;
    if (payableOn < effectiveOn) {
      formError = new TranslatedFormError(m.investments_merger_payable_error());
      return;
    }
    const ratio = parseExchangeRatio(newUnits, oldUnits);
    if (!ratio.ok) {
      formError = new TranslatedFormError(m.investments_exchange_ratio_error());
      return;
    }
    const cash = parseMoneyMagnitude(cashAmount, { maxScale: currency.standard_scale });
    if (!cash.ok || coefficientSign(cash.field.value) <= 0) {
      formError = new TranslatedFormError(m.investments_merger_cash_error());
      return;
    }
    const election = parseElectionPercent(electionPercent);
    if (!election.ok) {
      formError = new TranslatedFormError(m.investments_merger_percent_error());
      return;
    }
    // Evidence keys the form does not edit carry over from the corrected
    // merger.
    const { reference: _reference, ...kept } = initialEvidence;
    const evidence: Record<string, unknown> = { ...kept };
    if (sourceReference.trim()) evidence.reference = sourceReference.trim();
    const payload: CashStockMergerRequest = {
      effective_on: effectiveOn,
      payable_on: payableOn,
      holding_account_id: selectedHolding.accountID,
      ...(destination !== 'same' ? { destination_holding_account_id: Number(destination) } : {}),
      commodity_id: selectedHolding.commodityID,
      destination_commodity_id: newID,
      ratio_numerator: ratio.numerator,
      ratio_denominator: ratio.denominator,
      cash_account_id: Number(cashAccountID),
      currency_id: currency.id,
      cash_value: cash.field.value,
      cash_scale: cash.field.scale,
      cash_basis_fraction_value: election.value,
      cash_basis_fraction_scale: election.scale,
      source_evidence: Object.keys(evidence).length > 0 ? evidence : undefined,
      memo: memo.trim() || undefined
    };
    pending = true;
    formError = undefined;
    try {
      await runPreview(payload);
    } catch (error) {
      preview = null;
      formError = error;
    } finally {
      pending = false;
    }
  }

  function replacementRequest(payload: CashStockMergerRequest): CashStockMergerReplacementRequest {
    return {
      reason: reason.trim(), effective_on: payload.effective_on, payable_on: payload.payable_on,
      destination_holding_account_id: payload.destination_holding_account_id,
      destination_commodity_id: payload.destination_commodity_id,
      ratio_numerator: payload.ratio_numerator, ratio_denominator: payload.ratio_denominator,
      cash_account_id: payload.cash_account_id, currency_id: payload.currency_id,
      cash_value: payload.cash_value, cash_scale: payload.cash_scale,
      cash_basis_fraction_value: payload.cash_basis_fraction_value,
      cash_basis_fraction_scale: payload.cash_basis_fraction_scale,
      source_evidence: payload.source_evidence, memo: payload.memo
    };
  }

  async function runPreview(payload: CashStockMergerRequest): Promise<void> {
    const result = correction
      ? await previewCashStockMergerReplacement(correction.transactionID, replacementRequest(payload))
      : await previewCashStockMerger(payload);
    preview = {
      plan: result.plan,
      impacts: result.impact.affected_checkpoints,
      gainImpact: hasGainChanges(result.impact.gain_impact) ? result.impact.gain_impact : null,
      payload
    };
  }

  async function record(override: boolean) {
    if (!preview) return;
    const acknowledgement = gainAcknowledgement(preview.gainImpact);
    const confirmations = {
      ...(override ? { reconciliation_override: true } : {}),
      ...(acknowledgement ? { gain_impact_acknowledgement: acknowledgement } : {})
    };
    if (correction) {
      await replaceCashStockMerger(correction.transactionID, { ...replacementRequest(preview.payload), ...confirmations }, csrfToken);
    } else {
      await recordCashStockMerger({ ...preview.payload, ...confirmations }, csrfToken);
    }
    await invalidateInvestmentReads(queryClient);
    onSaved();
  }

  // The server recomputes everything at commit; a moved holding is refused.
  // A changed gain set re-opens the review with the current figures instead
  // of a dead-end error.
  async function commit(override: boolean) {
    if (!preview) return;
    pending = true;
    formError = undefined;
    try {
      await record(override);
    } catch (error) {
      if (isGainAcknowledgementRefusal(error) && preview) {
        try {
          await runPreview(preview.payload);
          if (preview && impactNeedsReview({ affected_checkpoints: preview.impacts, gain_impact: preview.gainImpact })) {
            reviewModal = { gainRefreshed: !!preview.gainImpact };
          }
        } catch (previewError) {
          preview = null;
          formError = previewError;
        }
      } else {
        formError = error;
      }
    } finally {
      pending = false;
    }
  }

  function handleRecord() {
    if (!preview) return;
    if (impactNeedsReview({ affected_checkpoints: preview.impacts, gain_impact: preview.gainImpact })) {
      reviewModal = { gainRefreshed: false };
      return;
    }
    void commit(false);
  }

  function confirmReview() {
    if (!preview) return;
    const override = preview.impacts.length > 0;
    reviewModal = null;
    void commit(override);
  }
</script>

{#if reviewModal && preview}
  <ReconciliationConfirm impacts={preview.impacts} gainRows={modalGainRows}
    gainRefreshed={reviewModal.gainRefreshed} {pending}
    onCancel={() => (reviewModal = null)} onConfirm={confirmReview} />
{/if}

<form onsubmit={handlePreview} class="space-y-4" aria-busy={pending}>
  <h2 id="merger-title" class="text-base font-semibold text-foreground">
    {correction ? m.transactions_investment_replace_merger_title() : m.investments_merger_title()}
  </h2>
  <p class="text-sm text-muted">{correction ? m.investments_merger_replace_help() : m.investments_merger_help()}</p>
  {#if loading}
    <p class="text-sm text-muted" role="status">{m.investments_loading()}</p>
  {:else if loadError}
    <p class="text-sm text-danger" role="alert">{m.investments_exchange_load_error()}</p>
  {:else}
    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <label for="merger-date" class="mb-1 block text-sm font-medium text-foreground">{m.investments_exchange_effective_date()}</label>
        <input id="merger-date" type="date" bind:this={dateInput} bind:value={effectiveOn} required oninput={discardPreview}
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
      </div>
      <div>
        <label for="merger-payable" class="mb-1 block text-sm font-medium text-foreground">{m.investments_merger_payable_date()}</label>
        <input id="merger-payable" type="date" bind:value={payableOn} min={effectiveOn || undefined} required oninput={discardPreview}
          aria-describedby="merger-payable-hint"
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
        <p id="merger-payable-hint" class="mt-1 text-xs text-muted">{m.investments_merger_payable_hint()}</p>
      </div>
    </div>
    {#if holdings.length === 0 && !correction}
      <p class="text-sm text-muted" role="status">{m.investments_exchange_empty()}</p>
    {/if}
    <div>
      <label for="merger-holding" class="mb-1 block text-sm font-medium text-foreground">{m.investments_merger_holding()}</label>
      {#if correction}
        <p id="merger-holding" class="rounded-(--radius-control) border border-border bg-surface px-3 py-2 text-sm text-foreground">
          {accounts.find((account) => account.id === correction.terms.holding_account_id)?.name ?? `#${correction.terms.holding_account_id}`}
          · {instrumentLabel(correction.terms.commodity_id)}
        </p>
      {:else}
        <select id="merger-holding" aria-describedby="merger-holding-hint" bind:value={holdingKey} required onchange={selectHolding}
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground">
          <option value="">{m.investments_exchange_select_holding()}</option>
          {#each holdings as holding (holding.key)}
            <option value={holding.key}>
              {accounts.find((account) => account.id === holding.accountID)?.name} · {instrumentLabel(holding.commodityID)}
            </option>
          {/each}
        </select>
        <p id="merger-holding-hint" class="mt-1 text-xs text-muted">{m.investments_dated_holdings_hint()}</p>
      {/if}
    </div>
    <div>
      <label for="merger-new-instrument" class="mb-1 block text-sm font-medium text-foreground">{m.investments_exchange_new_instrument()}</label>
      <select id="merger-new-instrument" bind:value={newCommodityID} required onchange={resetDestination}
        aria-describedby={newInstruments.length === 0 ? 'merger-no-instruments' : undefined}
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground">
        <option value="">{m.investments_exchange_select_instrument()}</option>
        {#each newInstruments as instrument (instrument.commodity_id)}
          <option value={String(instrument.commodity_id)}>{instrument.display_name}</option>
        {/each}
      </select>
      {#if newInstruments.length === 0}
        <p id="merger-no-instruments" class="mt-1 text-xs text-muted" role="status">{m.investments_exchange_no_instruments()}</p>
      {/if}
    </div>
    {#if selectedHolding && newID > 0}
      <div>
        <label for="merger-destination" class="mb-1 block text-sm font-medium text-foreground">{m.investments_exchange_destination()}</label>
        <select id="merger-destination" aria-describedby="merger-destination-hint" value={destination} required
          onchange={(event) => { chosenDestination = event.currentTarget.value; discardPreview(); }}
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground">
          <option value="" disabled={sameHoldingAllowed || otherDestinations.length > 0}>{m.investments_exchange_select_destination()}</option>
          {#if sameHoldingAllowed}
            <option value="same">{m.investments_exchange_same_holding({ name: sourceAccount?.name ?? '' })}</option>
          {/if}
          {#each otherDestinations as account (account.id)}
            <option value={String(account.id)}>{account.name}</option>
          {/each}
        </select>
        <p id="merger-destination-hint" class="mt-1 text-xs text-muted">
          {sameHoldingAllowed || otherDestinations.length > 0 ? m.investments_exchange_destination_hint() : m.investments_exchange_no_destination()}
        </p>
      </div>
    {/if}
    <fieldset class="space-y-2">
      <legend class="text-sm font-medium text-foreground">{m.investments_exchange_ratio()}</legend>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label for="merger-new-units" class="mb-1 block text-xs text-muted">{m.investments_exchange_ratio_new()}</label>
          <input id="merger-new-units" type="text" inputmode="numeric" autocomplete="off" bind:value={newUnits}
            required oninput={discardPreview} aria-describedby="merger-ratio-help"
            class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-mono text-foreground" />
        </div>
        <div>
          <label for="merger-old-units" class="mb-1 block text-xs text-muted">{m.investments_exchange_ratio_old()}</label>
          <input id="merger-old-units" type="text" inputmode="numeric" autocomplete="off" bind:value={oldUnits}
            required oninput={discardPreview} aria-describedby="merger-ratio-help"
            class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-mono text-foreground" />
        </div>
      </div>
      <p id="merger-ratio-help" class="text-xs text-muted">{m.investments_exchange_ratio_help()}</p>
    </fieldset>
    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <label for="merger-cash-account" class="mb-1 block text-sm font-medium text-foreground">{m.investments_merger_cash_account()}</label>
        <select id="merger-cash-account" bind:value={cashAccountID} required onchange={discardPreview}
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground">
          <option value="">{m.investments_merger_select_cash_account()}</option>
          {#each cashAccounts as account (account.id)}
            <option value={String(account.id)}>{account.name}</option>
          {/each}
        </select>
      </div>
      <div>
        <label for="merger-currency" class="mb-1 block text-sm font-medium text-foreground">{m.investments_merger_currency()}</label>
        <select id="merger-currency" value={currencyID} required aria-describedby="merger-currency-hint"
          onchange={(event) => { chosenCurrencyID = event.currentTarget.value; discardPreview(); }}
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground">
          <option value="">{m.investments_merger_select_currency()}</option>
          {#each currencies as option (option.id)}
            <option value={String(option.id)}>{option.code}</option>
          {/each}
        </select>
        <p id="merger-currency-hint" class="mt-1 text-xs text-muted">{m.investments_merger_currency_hint()}</p>
      </div>
    </div>
    <div>
      <label for="merger-cash" class="mb-1 block text-sm font-medium text-foreground">{m.investments_merger_cash()}</label>
      <input id="merger-cash" type="text" inputmode="decimal" autocomplete="off" bind:value={cashAmount}
        required oninput={discardPreview} aria-describedby="merger-cash-help"
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-mono text-foreground" />
      <p id="merger-cash-help" class="mt-1 text-xs text-muted">{m.investments_merger_cash_help()}</p>
    </div>
    <div>
      <label for="merger-percent" class="mb-1 block text-sm font-medium text-foreground">{m.investments_merger_percent()}</label>
      <div class="flex items-center gap-2">
        <input id="merger-percent" type="text" inputmode="decimal" autocomplete="off" bind:value={electionPercent}
          required oninput={discardPreview} aria-describedby="merger-percent-help"
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-mono text-foreground" />
        <span class="text-sm text-muted" aria-hidden="true">%</span>
      </div>
      <p id="merger-percent-help" class="mt-1 text-xs text-muted">{m.investments_merger_percent_help()}</p>
    </div>
    <div>
      <label for="merger-reference" class="mb-1 block text-sm font-medium text-foreground">{m.investments_transfer_source_reference()}</label>
      <input id="merger-reference" type="text" bind:value={sourceReference} maxlength="500" oninput={discardPreview}
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
    </div>
    {#if correction}
      <div>
        <label for="merger-reason" class="mb-1 block text-sm font-medium text-foreground">{m.investments_exchange_replace_reason()}</label>
        <input id="merger-reason" type="text" bind:value={reason} maxlength="500" required oninput={discardPreview}
          class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
      </div>
    {/if}
    <div>
      <label for="merger-memo" class="mb-1 block text-sm font-medium text-foreground">{m.investments_form_memo()}</label>
      <input id="merger-memo" type="text" bind:value={memo} maxlength="500" oninput={discardPreview}
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
    </div>
    {#if preview && selectedHolding}
      <section class="space-y-2 rounded-(--radius-control) border border-border p-3" aria-labelledby="merger-preview-title" aria-live="polite">
        <h3 id="merger-preview-title" class="text-sm font-semibold text-foreground">{m.investments_merger_preview_title()}</h3>
        <CashAndStockMergerSummary plan={preview.plan} oldName={instrumentLabel(selectedHolding.commodityID)}
          newName={instrumentLabel(newID)} {currenciesByID} />
      </section>
    {/if}
  {/if}
  <APIFormError error={formError} id="merger-form-error" />
  <div class="flex flex-wrap justify-end gap-3">
    <button type="button" onclick={onCancel}
      class="rounded-(--radius-control) border border-border bg-control px-4 py-2.5 text-sm font-semibold text-foreground transition hover:bg-control-hover">
      {m.investments_form_cancel()}
    </button>
    {#if preview}
      <button type="button" onclick={handleRecord} disabled={pending || !csrfToken}
        class="rounded-(--radius-control) bg-foreground px-4 py-2.5 text-sm font-semibold text-background transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50">
        {pending ? m.investments_exchange_pending() : correction ? m.investments_merger_replace_submit() : m.investments_merger_submit()}
      </button>
    {:else}
      <button type="submit" disabled={!canPreview || pending}
        class="rounded-(--radius-control) bg-foreground px-4 py-2.5 text-sm font-semibold text-background transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50">
        {pending ? m.investments_exchange_previewing() : m.investments_merger_preview()}
      </button>
    {/if}
  </div>
</form>
