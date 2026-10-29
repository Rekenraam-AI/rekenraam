<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    eyebrow,
    title,
    copy,
    children
  }: {
    eyebrow?: string;
    title: string;
    copy?: string;
    children?: Snippet;
  } = $props();
</script>

<!-- No horizontal padding of its own: the shell's content column sets the one
     left edge the header, tab row and page share (#436). -->
<header class="pb-4">
  <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
    <div class="min-w-0">
      {#if eyebrow}
        <p class="text-xs font-semibold uppercase tracking-[0.14em] text-muted">{eyebrow}</p>
      {/if}
      <h1 class="truncate text-2xl font-semibold tracking-tight text-foreground sm:text-3xl" class:mt-1={eyebrow}>{title}</h1>
      {#if copy}
        <p class="mt-2 max-w-4xl text-sm leading-6 text-muted">{copy}</p>
      {/if}
    </div>

    {#if children}
      <div class="flex shrink-0 flex-wrap items-center gap-2">
        {@render children()}
      </div>
    {/if}
  </div>
</header>
