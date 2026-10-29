<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { authSessionQueryOptions } from '#lib/api/auth.ts';
  import GainsReport from '#lib/investments/gains-report.svelte';

  const sessionQuery = createQuery(() => authSessionQueryOptions());
</script>

<GainsReport csrfToken={sessionQuery.data?.csrf_token ?? ''} />
