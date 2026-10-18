<script lang="ts">
  import { QueryClientProvider } from '@tanstack/svelte-query';
  import { onMount } from 'svelte';
  import '../app.css';
  import { createQueryClient } from '#lib/query-client.ts';
  import TopBar from '#lib/components/top-bar.svelte';
  import { initializeTheme } from '#lib/theme.svelte.ts';
  import { getLocale } from '#lib/paraglide/runtime.js';

  let { children } = $props();
  const queryClient = createQueryClient();

  onMount(() => {
    initializeTheme();
    // app.html ships lang="en"; assistive technology reads the page in the
    // language it declares, so it follows the active locale (#325). Switching
    // locale reloads the document, so setting it once is enough.
    document.documentElement.lang = getLocale();
  });
</script>

<QueryClientProvider client={queryClient}>
  <div class="min-h-screen">
    <TopBar />
    {@render children()}
  </div>
</QueryClientProvider>
