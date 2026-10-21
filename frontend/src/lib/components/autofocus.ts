import type { Attachment } from 'svelte/attachments';

/** Focuses the element each time it mounts: a form's first field, a review step's safe action. */
export const autofocus: Attachment<HTMLElement> = (node) => {
  node.focus();
};
