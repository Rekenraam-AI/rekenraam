<script lang="ts">
  import { parseISO, isValid } from 'date-fns';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { basisSourceFields } from './basis-provenance';

  let { evidence }: { evidence: Record<string, unknown> } = $props();
  const source = $derived(basisSourceFields(evidence));
  const knownDate = $derived(parseISO(source.knownOn));
  const knownOn = $derived(isValid(knownDate)
    ? new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium' }).format(knownDate) : source.knownOn);
</script>

<div class="mt-1 break-words text-xs text-muted">
  {#if source.kind === 'statement'}
    <p>{m.investments_basis_source_history_statement({ reference: source.reference })}</p>
  {:else if source.kind === 'provider'}
    <p>{m.investments_basis_source_history_provider({ reference: source.reference })}</p>
  {:else if source.kind === 'owner_assertion'}
    <p>{m.investments_basis_source_history_owner({ rationale: source.rationale, date: knownOn })}</p>
  {:else}
    <p>{m.investments_basis_source_history_legacy()}</p>
  {/if}
  <details class="mt-1">
    <summary class="cursor-pointer">{m.investments_basis_source_history_evidence()}</summary>
    <pre class="mt-1 whitespace-pre-wrap break-words">{JSON.stringify(evidence, null, 2)}</pre>
  </details>
</div>
