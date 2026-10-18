<script lang="ts" module>
  // The opener of the most recently closed dialog. A dialog opened from inside
  // another one (a reason modal handing over to a reconciliation override)
  // often loses its own opener when the first dialog closes; it then returns
  // focus to where that first dialog would have.
  let lastReturnTarget: HTMLElement | null = null;
</script>

<script lang="ts">
  import { onMount, type Snippet } from 'svelte';

  // The app's one modal dialog (#327). Mount it to open it and unmount it to
  // close it. The native modal <dialog> moves focus inside, makes the page
  // behind it inert and turns Escape into a `cancel` event; this component
  // routes that to onCancel unless pending, and returns focus to the opener.
  // Initial focus is the first focusable descendant, or the one marked
  // `data-initial-focus` (destructive dialogs keep it off the destructive
  // button). A form inside that reports aria-busy counts as pending, so a
  // write in flight is never dismissed from under its owner.
  let {
    role = 'dialog',
    labelledby,
    label,
    describedby,
    pending = false,
    onCancel,
    closeOnBackdrop = false,
    fallbackFocus,
    class: className = '',
    children
  }: {
    role?: 'dialog' | 'alertdialog';
    labelledby?: string;
    label?: string;
    describedby?: string;
    /** While true, Escape and backdrop clicks are ignored. */
    pending?: boolean;
    /** Escape and backdrop clicks call this; without it they do nothing. */
    onCancel?: () => void;
    closeOnBackdrop?: boolean;
    /** Where focus goes on close when the opener has left the page. */
    fallbackFocus?: () => HTMLElement | null | undefined;
    class?: string;
    children: Snippet;
  } = $props();

  let dialog: HTMLDialogElement;
  let closing = false;

  onMount(() => {
    const active = document.activeElement;
    const opener = active instanceof HTMLElement && active !== document.body ? active : lastReturnTarget;
    dialog.showModal();
    dialog.querySelector<HTMLElement>('[data-initial-focus]')?.focus();
    return () => {
      closing = true;
      if (dialog.open) dialog.close();
      const target = opener?.isConnected ? opener : (fallbackFocus?.() ?? null);
      lastReturnTarget = target;
      target?.focus();
    };
  });

  function busy() {
    return pending || dialog.querySelector('[aria-busy="true"]') !== null;
  }

  function cancel(event: Event) {
    event.preventDefault();
    if (!busy()) onCancel?.();
  }

  // Chromium may close a dialog without a cancelable `cancel` when Escape is
  // pressed repeatedly without other user activation. Keep it open unless the
  // owner agrees to close.
  function closed() {
    if (closing) return;
    if (busy() || !onCancel) dialog.showModal();
    else onCancel();
  }

  const focusableSelector = 'a[href], area[href], button, input:not([type="hidden"]), select, textarea, summary, [tabindex]';

  // A modal <dialog> lets Tab leave for the browser's own controls after its
  // last element; wrap instead, so focus never lands on the inert page.
  // Events from a nested dialog belong to that dialog.
  function keydown(event: KeyboardEvent) {
    if (event.key !== 'Tab' || event.defaultPrevented) return;
    if (!(event.target instanceof Element) || event.target.closest('dialog') !== dialog) return;
    const focusable = [...dialog.querySelectorAll<HTMLElement>(focusableSelector)].filter((element) =>
      element.closest('dialog') === dialog && element.tabIndex >= 0 && !element.matches(':disabled') &&
      element.checkVisibility());
    if (focusable.length === 0) { event.preventDefault(); return; }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && (event.target === first || event.target === dialog)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && event.target === last) {
      event.preventDefault();
      first.focus();
    }
  }

  function backdropClick(event: MouseEvent) {
    if (closeOnBackdrop && event.target === dialog) cancel(event);
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<dialog
  bind:this={dialog}
  role={role === 'alertdialog' ? 'alertdialog' : undefined}
  aria-modal="true"
  aria-labelledby={labelledby}
  aria-label={label}
  aria-describedby={describedby}
  oncancel={cancel}
  onclose={closed}
  onclick={backdropClick}
  onkeydown={keydown}
  class="m-auto max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] overflow-y-auto rounded-(--radius-panel) border border-border bg-surface text-foreground shadow-(--shadow-panel) backdrop:bg-background/70 backdrop:backdrop-blur-sm {className}"
>
  {@render children()}
</dialog>
