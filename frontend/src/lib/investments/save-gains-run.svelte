<script lang="ts">
  import { createMutation, useQueryClient } from '@tanstack/svelte-query';
  import { saveGainsRun, gainsRunQueryKey, gainsRunsQueryKey, type GainsRunRequest } from '#lib/api/investments.ts';
  import { m } from '#lib/paraglide/messages.js';

  let { request, csrfToken, onSaved }: { request: GainsRunRequest; csrfToken: string; onSaved: (id: number) => void } = $props();
  const client = useQueryClient();
  const save = createMutation(() => ({
    mutationFn: (input: GainsRunRequest) => saveGainsRun(input, csrfToken),
    onSuccess: async (run) => {
      client.setQueryData(gainsRunQueryKey(run.id), run);
      onSaved(run.id);
      await client.invalidateQueries({ queryKey: gainsRunsQueryKey });
    }
  }));
</script>

<div class="space-y-2" data-testid={`save-${request.report_type}-run`}>
  <button type="button" disabled={save.isPending || !csrfToken} onclick={() => save.mutate({ ...request })}
    class="min-h-11 rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-medium text-foreground hover:bg-control-hover disabled:opacity-50">
    {save.isPending ? m.valuation_runs_saving() : request.report_type === 'statement' ? m.valuation_runs_save_statement() : m.valuation_runs_save_gains()}
  </button>
  <p class="text-xs text-muted">{m.valuation_runs_save_note()}</p>
  {#if save.isError}<p role="alert" class="text-sm text-destructive">{m.valuation_runs_save_error()}</p>{/if}
  {#if save.isSuccess}<p role="status" class="text-sm text-foreground">{m.valuation_runs_saved()}</p>{/if}
</div>
