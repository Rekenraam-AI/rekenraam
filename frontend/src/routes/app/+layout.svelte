<script lang="ts">
  import { browser } from '$app/env';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { createQuery } from '@tanstack/svelte-query';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import { recurringSummaryQueryOptions } from '#lib/api/recurring.ts';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import APIFormError from '#lib/components/api-form-error.svelte';
  import PageHeader from '#lib/components/page-header.svelte';
  import Panel from '#lib/components/panel.svelte';
  import AppHeader from '#lib/navigation/app-header.svelte';
  import { canonicalURL, routeFor, sectionFor } from '#lib/navigation/routes.ts';
  import { authSessionQueryOptions, logout } from '#lib/api/auth.ts';
  import { setupStatusQueryOptions } from '#lib/api/setup.ts';
  import { APIClientError } from '#lib/api/client.ts';
  import { m } from '#lib/paraglide/messages.js';

  import { provideOwnerToday } from '#lib/dates/owner-date-context.ts';
  import { sessionOwnerToday } from '#lib/dates/owner-date.ts';

  let { children } = $props();

  const setupQuery = createQuery(() => setupStatusQueryOptions());
  const sessionQuery = createQuery(() => authSessionQueryOptions());

  provideOwnerToday(() => sessionOwnerToday(sessionQuery.data!));

  let logoutError = $state<unknown>(undefined);
  let logoutPending = $state(false);

  const shellError = $derived(setupQuery.error ?? sessionQuery.error);

  const shellState = $derived.by<'loading' | 'error' | 'ready'>(() => {
    if (setupQuery.isPending || sessionQuery.isPending) {
      return 'loading';
    }

    if (setupQuery.isError || sessionQuery.isError) {
      return 'error';
    }

    if (setupQuery.data === undefined || sessionQuery.data === undefined) {
      return 'loading';
    }

    return 'ready';
  });

  const shouldRedirectHome = $derived.by(() => {
    if (shellState !== 'ready') {
      return false;
    }

    return setupQuery.data?.install_state !== 'configured' || !sessionQuery.data?.authenticated;
  });

  // An old `?view=`/`?tab=` link or a moved page is replaced by its canonical
  // route before the page renders, keeping every other parameter (#436).
  const legacyTarget = $derived(canonicalURL(page.url));

  $effect(() => {
    if (browser && legacyTarget) {
      void goto(`${legacyTarget.pathname}${legacyTarget.search}${legacyTarget.hash}`, { replaceState: true });
    }
  });

  const entry = $derived(routeFor(page.url.pathname));
  // An overview's tab says "Overview"; its heading and document title name
  // the section instead.
  const pageTitle = $derived.by(() => {
    if (!entry) return m.app_name();
    if (entry.route === '/app/settings') return m.settings_title();
    const section = sectionFor(entry.section);
    return section && section.overview === entry.route ? section.title() : entry.title();
  });
  // Settings pages share one heading; the side list beside it names the page.
  const heading = $derived(entry?.section === 'settings' ? m.settings_title() : pageTitle);

  // Until the composed attention read (#458), the recurring summary is the one
  // count the shell shows.
  const recurringSummary = createQuery(() => ({ ...recurringSummaryQueryOptions(), enabled: shellState === 'ready' && !shouldRedirectHome }));
  const badges = $derived.by(() => {
    const count = recurringSummary.data?.draft_count ?? 0;
    const formatted = new Intl.NumberFormat(getLocale()).format(count);
    return new Map([['/app/recurring', { count, label: m.recurring_badge({ count: formatted }) }]]);
  });

  $effect(() => {
    if (browser && shouldRedirectHome) {
      void goto('/');
    }
  });

  async function refreshShell() {
    await Promise.all([setupQuery.refetch(), sessionQuery.refetch()]);
  }

  async function handleLogout() {
    logoutPending = true;
    logoutError = undefined;

    try {
      const refreshedSession = await sessionQuery.refetch();

      if (!refreshedSession.data?.authenticated) {
        await goto('/');
        return;
      }

      if (!refreshedSession.data.csrf_token) {
        throw new APIClientError({ status: 403, code: 'CSRF_INVALID' });
      }

      await logout(refreshedSession.data.csrf_token);
      await sessionQuery.refetch();
      await goto('/');
    } catch (error) {
      logoutError = error;
    } finally {
      logoutPending = false;
    }
  }
</script>

<!--
  A page with no <title> is a tab a reader cannot tell apart and, for a screen
  reader, a navigation that announces nothing (R3a, axe rule document-title).
  Read from the route registry, the same source as the heading and the tabs.
-->
<svelte:head>
  <title>{pageTitle} · {m.app_name()}</title>
</svelte:head>

{#if shellState === 'loading'}
  <main class="min-h-screen px-4 py-14 sm:px-6 lg:px-8">
    <div class="mx-auto max-w-3xl">
      <h1 class="text-2xl font-semibold tracking-tight text-balance">{m.shell_loading_title()}</h1>
      <p class="mt-3 text-sm leading-6 text-muted">{m.shell_loading_copy()}</p>
    </div>
  </main>
{:else if shellState === 'error'}
  <main class="min-h-screen px-4 py-14 sm:px-6 lg:px-8">
    <div class="mx-auto max-w-3xl">
      <Panel>
        <h1 class="text-2xl font-semibold tracking-tight text-balance">{m.shell_error_title()}</h1>
        <p class="mt-3 text-sm leading-6 text-muted">{m.shell_error_copy()}</p>

        <div class="mt-6 space-y-4">
          <APIFormError error={shellError} id="app-shell-error" />
          <button
            type="button"
            class="inline-flex items-center gap-2 rounded-(--radius-control) bg-foreground px-4 py-2.5 text-sm font-semibold text-background transition hover:opacity-90"
            onclick={refreshShell}
          >
            <RefreshCw size={16} aria-hidden="true" />
            {m.shell_retry()}
          </button>
        </div>
      </Panel>
    </div>
  </main>
{:else if !shouldRedirectHome && !legacyTarget}
  <AppHeader
    {entry}
    search={page.url.search}
    {badges}
    signOutPending={logoutPending}
    signOutError={logoutError}
    onSignOut={handleLogout}
  />

  <main id="main-content" tabindex="-1" class="mx-auto min-h-screen max-w-[100rem] px-4 pb-10 pt-5 outline-none sm:px-6 lg:px-8">
    {#if !entry?.ownHeader}
      <PageHeader title={heading} />
    {/if}
    {@render children()}
  </main>
{/if}
