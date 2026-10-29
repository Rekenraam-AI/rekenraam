<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { authSessionQueryOptions } from '#lib/api/auth.ts';
  import InstrumentsPanel from '#lib/investments/instruments-panel.svelte';

  const sessionQuery = createQuery(() => authSessionQueryOptions());
</script>

<InstrumentsPanel csrfToken={sessionQuery.data?.csrf_token ?? ''} />
