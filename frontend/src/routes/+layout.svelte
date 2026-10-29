<script lang="ts">
  import { QueryClientProvider } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import '../app.css';
  import { createQueryClient } from '#lib/query-client.ts';
  import PublicHeader from '#lib/navigation/public-header.svelte';
  import { syncAmountPrivacy } from '#lib/money/privacy.svelte.ts';
  import { initializeTheme } from '#lib/theme.svelte.ts';
  import { getLocale } from '#lib/paraglide/runtime.js';

  let { children } = $props();
  const queryClient = createQueryClient();

  // The authenticated app draws its own header; everything else (sign-in,
  // setup, recovery, an error page) gets the minimal one.
  const inApp = $derived(page.url.pathname.startsWith('/app') && !page.error);

  onMount(() => {
    initializeTheme();
    // app.html ships lang="en"; assistive technology reads the page in the
    // language it declares, so it follows the active locale (#325). Switching
    // locale reloads the document, so setting it once is enough.
    document.documentElement.lang = getLocale();
  });
</script>

<svelte:window onstorage={syncAmountPrivacy} />

<QueryClientProvider client={queryClient}>
  <div class="min-h-screen">
    {#if !inApp}
      <PublicHeader />
    {/if}
    {@render children()}
  </div>
</QueryClientProvider>
