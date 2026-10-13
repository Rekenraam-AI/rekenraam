<script lang="ts">
  import { createQuery, useQueryClient } from '@tanstack/svelte-query';
  import { untrack } from 'svelte';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { currenciesQueryOptions, type CurrencyResponse } from '#lib/api/currencies.ts';
  import { parseMoneyMagnitude } from '#lib/investments/form-amounts.ts';
  import ReconciliationConfirm from '#lib/investments/reconciliation-confirm.svelte';
  import { TranslatedFormError } from '#lib/form-errors.ts';
  import {
    gainAcknowledgement,
    gainImpactCurrency,
    gainImpactRows,
    hasGainChanges,
    impactNeedsReview,
    isGainAcknowledgementRefusal
  } from '#lib/investments/gain-impact.ts';
  import { formatLedgerAmount } from '#lib/money/amount.ts';
  import {
    previewBasisResolutionReplacement,
    previewTransferBasisResolution,
    replaceBasisResolution,
    resolveTransferBasis,
    type InvestmentCorrectionBasisResolutionTerms,
    type GainImpact,
    type InvestmentBasisResolutionRequest,
    type ReconciliationImpactResponse
  } from '#lib/api/investments.ts';
  import { invalidateInvestmentReads } from './invalidate';
  import { basisSourceDeclaration, basisSourceFields, basisSourceTextMaxLength, basisSourceTextTooLong } from './basis-provenance';

  // Resolves an external transfer in whose basis was recorded unknown (T-145),
  // or, given the effective resolution, corrects it (#168). The basis is the
  // transfer's total sourced basis in its own currency; the resolution posts
  // the omitted cost-basis entry on the transfer date (none for a known zero)
  // and revises every sale and transfer the shares reached.
  let {
    csrfToken,
    transactionID,
    costCommodityID,
    current = null,
    onSaved,
    onCancel
  }: {
    csrfToken: string;
    transactionID: number;
    costCommodityID: number;
    current?: InvestmentCorrectionBasisResolutionTerms | null;
    onSaved: () => void;
    onCancel: () => void;
  } = $props();

  const correcting = untrack(() => current);
  const queryClient = useQueryClient();
  const currenciesQuery = createQuery(() => currenciesQueryOptions());

  let basisStr = $state(correcting
    ? formatLedgerAmount(correcting.basis_value, correcting.basis_scale) : '');
  const initialSource = basisSourceFields(correcting?.source_evidence);
  let sourceKind = $state(correcting ? initialSource.kind : 'statement');
  let reference = $state(initialSource.reference);
  let rationale = $state(initialSource.rationale);
  let knownOn = $state(initialSource.knownOn);
  let reason = $state('');
  let pending = $state(false);
  let formError = $state<unknown>(undefined);
  let basisInputElement: HTMLInputElement | undefined = $state();

  $effect(() => {
    basisInputElement?.focus();
  });

  let review = $state<{
    impacts: ReconciliationImpactResponse['affected_checkpoints'];
    gainImpact: GainImpact | null;
    gainRefreshed: boolean;
    payload: InvestmentBasisResolutionRequest;
  } | null>(null);

  const currenciesByID = $derived(new Map<number, CurrencyResponse>(
    (currenciesQuery.data?.currencies ?? []).map((currency: CurrencyResponse) => [currency.id, currency])));
  const basisCurrency = $derived(currenciesByID.get(costCommodityID));
  const gainRows = $derived(review?.gainImpact
    ? gainImpactRows(review.gainImpact.changes, gainImpactCurrency(currenciesByID), getLocale()) : []);
  const declaration = $derived(basisSourceDeclaration(sourceKind, reference, rationale, knownOn));
  const sourceTextTooLong = $derived(basisSourceTextTooLong(sourceKind === 'owner_assertion' ? rationale : reference));
  const canSubmit = $derived(!!basisStr.trim() && !!reason.trim() && declaration !== null);

  function buildPayload(): InvestmentBasisResolutionRequest | null {
    if (!declaration) return null;
    const basis = parseMoneyMagnitude(basisStr, { maxScale: 12 });
    if (!basis.ok) {
      formError = new TranslatedFormError(basis.reason === 'too_large'
        ? m.investments_form_amount_too_large() : m.investments_transfer_basis_error());
      return null;
    }
    return {
      basis_value: basis.field.value, basis_scale: basis.field.scale, reason: reason.trim(),
      source_evidence: declaration
    };
  }

  async function handleSubmit(event: Event) {
    event.preventDefault();
    if (!canSubmit || pending) return;
    formError = undefined;
    const payload = buildPayload();
    if (!payload) return;
    pending = true;
    try {
      if (!(await reviewImpact(payload, false))) await commit(payload, false, '');
    } catch (error) {
      await recover(error, payload);
    } finally {
      pending = false;
    }
  }

  // Preview through the actual resolution writer; checkpoints or the gains it
  // resolves go to the confirmation.
  async function reviewImpact(payload: InvestmentBasisResolutionRequest, refreshed: boolean): Promise<boolean> {
    const impact = correcting
      ? await previewBasisResolutionReplacement(transactionID, payload)
      : await previewTransferBasisResolution(transactionID, payload);
    if (!impactNeedsReview(impact)) return false;
    const gainImpact = hasGainChanges(impact.gain_impact) ? impact.gain_impact : null;
    review = { impacts: impact.affected_checkpoints, gainImpact, gainRefreshed: refreshed && !!gainImpact, payload };
    return true;
  }

  async function commit(payload: InvestmentBasisResolutionRequest, override: boolean, acknowledgement: string) {
    const body = {
      ...payload,
      ...(override ? { reconciliation_override: true } : {}),
      ...(acknowledgement ? { gain_impact_acknowledgement: acknowledgement } : {})
    };
    if (correcting) await replaceBasisResolution(transactionID, body, csrfToken);
    else await resolveTransferBasis(transactionID, body, csrfToken);
    await invalidateInvestmentReads(queryClient);
    onSaved();
  }

  async function recover(error: unknown, payload: InvestmentBasisResolutionRequest) {
    try {
      if (!isGainAcknowledgementRefusal(error) || !(await reviewImpact(payload, true))) formError = error;
    } catch (previewError) {
      formError = previewError;
    }
  }

  async function confirmReview() {
    if (!review) return;
    const { payload, impacts, gainImpact } = review;
    review = null;
    pending = true;
    formError = undefined;
    try {
      await commit(payload, impacts.length > 0, gainAcknowledgement(gainImpact));
    } catch (error) {
      await recover(error, payload);
    } finally {
      pending = false;
    }
  }
</script>

{#if review}
  <ReconciliationConfirm impacts={review.impacts} {gainRows} gainRefreshed={review.gainRefreshed}
    {pending} onCancel={() => (review = null)} onConfirm={confirmReview} />
{/if}

<form onsubmit={handleSubmit} class="space-y-4" aria-busy={pending}>
  <h2 class="text-base font-semibold text-foreground">
    {correcting ? m.investments_basis_resolution_correct_title() : m.investments_basis_resolution_title()}
  </h2>
  <p class="text-sm text-muted">
    {correcting ? m.investments_basis_resolution_correct_copy() : m.investments_basis_resolution_copy()}
  </p>

  <div>
    <label for="basis-resolution-amount" class="mb-1 block text-sm font-medium text-foreground">
      {m.investments_basis_resolution_amount({ currency: basisCurrency?.code ?? '' })}
    </label>
    <input id="basis-resolution-amount" type="text" inputmode="decimal" bind:this={basisInputElement}
      bind:value={basisStr} required aria-describedby="basis-resolution-amount-help"
      class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 font-mono text-sm text-foreground" />
    <p id="basis-resolution-amount-help" class="mt-1 text-xs text-muted">{m.investments_basis_resolution_amount_help()}</p>
  </div>

  <div>
    <label for="basis-resolution-source-kind" class="mb-1 block text-sm font-medium text-foreground">
      {m.investments_basis_source_kind()}
    </label>
    <select id="basis-resolution-source-kind" bind:value={sourceKind} required aria-describedby="basis-source-help"
      class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground">
      <option value="" disabled>{m.investments_basis_source_choose()}</option>
      <option value="statement">{m.investments_basis_source_statement()}</option>
      <option value="provider">{m.investments_basis_source_provider()}</option>
      <option value="owner_assertion">{m.investments_basis_source_owner()}</option>
    </select>
    <p id="basis-source-help" class="mt-1 text-xs text-muted">{m.investments_basis_source_help()}</p>
  </div>

  {#if sourceKind === 'owner_assertion'}
    <div>
      <label for="basis-resolution-rationale" class="mb-1 block text-sm font-medium text-foreground">
        {m.investments_basis_source_rationale()}
      </label>
      <textarea id="basis-resolution-rationale" bind:value={rationale} required rows="2"
        aria-invalid={sourceTextTooLong} aria-describedby={sourceTextTooLong ? "basis-resolution-error" : undefined}
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground"></textarea>
    </div>
    <div>
      <label for="basis-resolution-known-on" class="mb-1 block text-sm font-medium text-foreground">
        {m.investments_basis_source_known_on()}
      </label>
      <input id="basis-resolution-known-on" type="date" bind:value={knownOn} required
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
    </div>
  {:else}
    <div>
      <label for="basis-resolution-reference" class="mb-1 block text-sm font-medium text-foreground">
        {m.investments_basis_source_reference()}
      </label>
      <input id="basis-resolution-reference" type="text" bind:value={reference} required
        aria-invalid={sourceTextTooLong} aria-describedby={sourceTextTooLong ? "basis-resolution-error" : undefined}
        class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
    </div>
  {/if}

  <div>
    <label for="basis-resolution-reason" class="mb-1 block text-sm font-medium text-foreground">
      {m.investments_basis_resolution_reason()}
    </label>
    <input id="basis-resolution-reason" type="text" bind:value={reason} maxlength="500" required
      class="w-full rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm text-foreground" />
  </div>

  <APIFormError error={sourceTextTooLong
    ? new TranslatedFormError(m.investments_basis_source_too_long({ count: basisSourceTextMaxLength }))
    : formError} id="basis-resolution-error" />

  <div class="flex flex-wrap justify-end gap-3">
    <button type="button" onclick={onCancel} disabled={pending}
      class="min-h-10 rounded-(--radius-control) border border-border bg-control px-4 py-2 text-sm font-semibold text-foreground hover:bg-control-hover">
      {m.investments_form_cancel()}
    </button>
    <button type="submit" disabled={!canSubmit || pending}
      class="min-h-10 rounded-(--radius-control) bg-foreground px-4 py-2 text-sm font-semibold text-background transition-colors hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50">
      {pending ? m.investments_basis_resolution_pending()
        : correcting ? m.investments_basis_resolution_correct_submit() : m.investments_basis_resolution_submit()}
    </button>
  </div>
</form>
