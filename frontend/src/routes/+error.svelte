<script lang="ts">
  import { page } from '$app/state';
  import { m } from '#lib/paraglide/messages.js';
</script>

<!-- Even the page nobody meant to reach needs a name (R3a). -->
<svelte:head>
  <title>{page.status} · {m.app_name()}</title>
</svelte:head>

<main class="mx-auto flex min-h-[60vh] max-w-2xl flex-col justify-center px-6">
  <h1 class="text-2xl font-semibold text-foreground">{page.status}</h1>
  <p class="mt-2 text-sm leading-6 text-muted">{page.error?.message ?? ''}</p>
  <a class="mt-6 text-sm font-semibold text-accent underline" href="/app">
    {m.nav_home()}
  </a>
</main>
