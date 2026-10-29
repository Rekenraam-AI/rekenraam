<script lang="ts">
  import { page } from '$app/state';
  import type { Snippet } from 'svelte';
  import { m } from '#lib/paraglide/messages.js';
  import { routeIcons } from '#lib/navigation/icons.ts';
  import { sectionTabs } from '#lib/navigation/routes.ts';

  let { children }: { children?: Snippet } = $props();

  const pathname = $derived(page.url.pathname);

  // The pages come from the route registry, the same list as the header's
  // settings menu, so the two cannot disagree (#436).
  const sections = $derived(
    sectionTabs('settings').map((entry) => ({
      href: entry.route,
      label: entry.title(),
      icon: routeIcons[entry.icon],
      active: entry.route === '/app/settings' ? pathname === entry.route : pathname.startsWith(entry.route)
    }))
  );
</script>

<div class="grid gap-4 lg:grid-cols-[15rem_minmax(0,1fr)]">
  <aside class="min-w-0">
    <nav
      class="flex gap-2 overflow-x-auto rounded-(--radius-panel) border border-border bg-sidebar p-2 lg:block lg:space-y-1 lg:overflow-visible"
      aria-label={m.settings_nav_label()}
    >
      {#each sections as section}
        {@const Icon = section.icon}
        <a
          href={section.href}
          aria-current={section.active ? 'page' : undefined}
          class:bg-selected={section.active}
          class:text-selected-foreground={section.active}
          class:bg-transparent={!section.active}
          class:text-foreground={!section.active}
          class="flex min-w-fit items-center gap-2 rounded-(--radius-control) px-3 py-2 text-sm font-semibold transition hover:bg-control-hover"
        >
          <Icon size={16} aria-hidden="true" />
          {section.label}
        </a>
      {/each}
    </nav>
  </aside>

  <div class="min-w-0">
    {#if children}
      {@render children()}
    {/if}
  </div>
</div>
