<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import Panel from '#lib/components/panel.svelte';
  import { m } from '#lib/paraglide/messages.js';
  import { sectionTabs, type SectionID } from './routes';

  /**
   * A placeholder Overview: the section's pages as links, until the section's
   * real overview lands (Ledger #448, Planning and Reports #450). It reads the
   * route registry, so it lists exactly the tabs above it.
   */
  let {
    section,
    extra = []
  }: {
    section: SectionID;
    /** Pages that live in another section but belong in this list. */
    extra?: { href: string; label: string }[];
  } = $props();

  const links = $derived([
    ...sectionTabs(section)
      .slice(1)
      .map((entry) => ({ href: entry.route, label: entry.title() })),
    ...extra
  ]);
</script>

<Panel padding="none">
  <h2 class="px-5 pt-4 text-sm font-semibold text-muted">{m.nav_overview_pages()}</h2>
  <ul class="mt-2 divide-y divide-border">
    {#each links as link (link.href)}
      <li>
        <a href={link.href} class="flex min-h-12 items-center justify-between gap-3 px-5 text-sm font-medium text-foreground transition hover:bg-row-hover">
          {link.label}
          <ChevronRight size={16} class="shrink-0 text-muted" aria-hidden="true" />
        </a>
      </li>
    {/each}
  </ul>
</Panel>
