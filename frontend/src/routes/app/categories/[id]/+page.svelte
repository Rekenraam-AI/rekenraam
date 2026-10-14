<script lang="ts">
  import { page } from '$app/state';
  import { createQuery } from '@tanstack/svelte-query';
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import { m } from '#lib/paraglide/messages.js';
  import { categoriesQueryOptions } from '#lib/api/categories.ts';
  import CategoryTransactions from '#lib/transactions/category-transactions.svelte';
  import TransactionWorkspace, { type TransactionWorkspaceCreate } from '#lib/transactions/transaction-workspace.svelte';
  import { categoryDisplayName } from '#lib/categories/category-labels.ts';

  const categoryID = $derived(Number(page.params.id));

  const categoriesQuery = createQuery(() => categoriesQueryOptions({ includeArchived: true }));
  const category = $derived(categoriesQuery.data?.categories.find((c) => c.id === categoryID));
  const categoryName = $derived(category ? categoryDisplayName(category) : String(categoryID));

  // A new transaction starts in this category (#185) when it can take
  // postings; a summary or archived category still allows unassigned entry.
  const create = $derived<TransactionWorkspaceCreate>({
    kind: 'editor',
    defaults: category?.status === 'active' && category.allows_postings ? { categoryID: category.id } : undefined
  });
</script>

<TransactionWorkspace {create}>
  {#snippet header()}
    <a
      href="/app/categories"
      class="inline-flex items-center gap-1.5 rounded-(--radius-control) border border-border bg-control px-3 py-2 text-sm font-semibold text-foreground transition hover:bg-control-hover"
    >
      <ArrowLeft size={14} aria-hidden="true" />
      {m.category_tx_back_to_categories()}
    </a>
    {#if categoryName}
      <h2 class="text-sm font-semibold text-foreground">{categoryName}</h2>
    {/if}
  {/snippet}
  {#snippet list(actions, csrfToken)}
    <CategoryTransactions {categoryID} {csrfToken} {actions} onRowClick={actions.showDetail} />
  {/snippet}
</TransactionWorkspace>
