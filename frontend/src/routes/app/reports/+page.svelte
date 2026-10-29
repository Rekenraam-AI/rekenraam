<script lang="ts">
  import SectionOverview from '#lib/navigation/section-overview.svelte';
  import { m } from '#lib/paraglide/messages.js';

  // Gains is an Investments page; the Reports overview links it rather than
  // giving it a second home in this tab row (#436).
  const gains = $derived({
    href: '/app/investments/gains',
    label: m.nav_in_section({ page: m.investments_tab_gains(), section: m.investments_nav() })
  });
</script>

<SectionOverview section="reports" extra={[gains]} />
