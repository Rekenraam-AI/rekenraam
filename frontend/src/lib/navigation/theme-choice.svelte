<script lang="ts">
  import { m } from '#lib/paraglide/messages.js';
  import { setThemePreference, themeState, type ThemePreference } from '#lib/theme.svelte.ts';

  const choices = $derived<{ value: ThemePreference; label: string }[]>([
    { value: 'light', label: m.theme_name_light() },
    { value: 'dark', label: m.theme_name_dark() },
    { value: 'system', label: m.theme_name_system() }
  ]);
</script>

<!-- Three pressed-state buttons rather than radios: each applies at once, and
     the current one is announced as pressed. -->
<div role="group" aria-label={m.theme_label()} class="flex gap-1 rounded-(--radius-control) border border-border bg-control p-0.5">
  {#each choices as choice (choice.value)}
    <button
      type="button"
      aria-pressed={themeState.preference === choice.value}
      onclick={() => setThemePreference(choice.value)}
      class="min-h-9 flex-1 rounded-[calc(var(--radius-control)-2px)] px-2.5 text-sm font-medium transition {themeState.preference === choice.value
        ? 'bg-surface-strong text-foreground'
        : 'text-muted hover:bg-control-hover hover:text-foreground'}"
    >
      {choice.label}
    </button>
  {/each}
</div>
