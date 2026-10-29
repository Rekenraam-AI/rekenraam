<script lang="ts">
  import type { Attachment } from 'svelte/attachments';
  import { m } from '#lib/paraglide/messages.js';
  import { getLocale } from '#lib/paraglide/runtime.js';
  import { currentTab, sectionFor, sectionTabs, tabHref, type RouteEntry, type SectionID } from './routes';

  let {
    section,
    entry,
    search,
    badges
  }: {
    section: SectionID;
    entry: RouteEntry | undefined;
    search: string;
    /** Count and its full sentence for screen readers, by tab route. */
    badges: Map<string, { count: number; label: string }>;
  } = $props();

  const tabs = $derived(sectionTabs(section));
  const current = $derived(currentTab(entry));
  const sectionTitle = $derived(sectionFor(section)?.title() ?? '');
  const numberFormat = $derived(new Intl.NumberFormat(getLocale()));

  let fadeStart = $state(false);
  let fadeEnd = $state(false);

  // Keep the current tab in view on a phone, where the row scrolls.
  function trackTabs(_current: string | undefined): Attachment<HTMLElement> {
    return (scroller) => {
      const updateFade = () => {
        const { scrollLeft, scrollWidth, clientWidth } = scroller;
        fadeStart = scrollLeft > 1;
        fadeEnd = scrollLeft + clientWidth < scrollWidth - 1;
      };
      const keepCurrentVisible = () => {
        scroller.querySelector<HTMLElement>('[aria-current="page"]')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        updateFade();
      };
      keepCurrentVisible();
      const observer = new ResizeObserver(keepCurrentVisible);
      observer.observe(scroller);
      for (const tab of scroller.children) observer.observe(tab);
      scroller.addEventListener('scroll', updateFade);
      return () => {
        observer.disconnect();
        scroller.removeEventListener('scroll', updateFade);
      };
    };
  }
</script>

<nav aria-label={m.nav_section_pages({ section: sectionTitle })}>
  <ul
    {@attach trackTabs(current)}
    class="-mx-2.5 flex h-10 scroll-px-10 items-stretch gap-1 overflow-x-auto [scrollbar-width:none]"
    class:shell-tabs-fade-start={fadeStart}
    class:shell-tabs-fade-end={fadeEnd}
  >
    {#each tabs as tab (tab.route)}
      {@const badge = badges.get(tab.route)}
      <li class="flex shrink-0">
        <a
          href={tabHref(tab, entry, search)}
          aria-current={current === tab.route ? 'page' : undefined}
          class="relative flex items-center gap-1.5 px-2.5 text-sm font-medium text-muted transition hover:text-foreground aria-[current=page]:text-foreground aria-[current=page]:after:absolute aria-[current=page]:after:inset-x-2 aria-[current=page]:after:bottom-0 aria-[current=page]:after:h-0.5 aria-[current=page]:after:rounded-full aria-[current=page]:after:bg-foreground"
        >
          {tab.title()}
          {#if badge && badge.count > 0}
            <span class="rounded-full bg-foreground px-1.5 text-xs font-semibold text-background" aria-hidden="true">
              {numberFormat.format(badge.count)}
            </span>
            <span class="sr-only">{badge.label}</span>
          {/if}
        </a>
      </li>
    {/each}
  </ul>
</nav>
